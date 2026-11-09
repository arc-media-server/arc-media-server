package subtitles

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"ams/internal/store"
)

// ASS/SSA tracks keep their styling (fonts, colours, positions, karaoke) when an app draws them
// with libass, as the Android TV app does. Script returns a track as its own script, with the
// fonts the video carries for it packed in, so one file is all a player needs.

// IsASS reports whether a track is an ASS/SSA script.
func (t Track) IsASS() bool { return t.Codec == "ass" || t.Codec == "ssa" }

// The fonts packed into one script are capped, so a file with an unusual number of large fonts
// can't make a subtitle download enormous; fonts past the cap are left out (libass then falls
// back to similar system fonts).
const maxFontBytes = 40 << 20

// Font is a font file attached to a video (Matroska attachments).
type Font struct {
	Name string
	Data []byte
}

// Script returns an ASS/SSA track as a script: copied from the video (reading the whole file the
// first time, then cached) or read from the file beside it, as UTF-8.
func (c *Cache) Script(ctx context.Context, f *store.MediaFile, t Track) ([]byte, error) {
	if !t.IsASS() {
		return nil, errors.New("only ASS and SSA subtitles can be sent as a script")
	}
	if t.External {
		b, err := os.ReadFile(t.Path)
		if err != nil {
			return nil, err
		}
		return toUTF8(b), nil
	}
	key, err := sourceKey(f.Path, t.ID)
	if err != nil {
		return nil, err
	}
	path := filepath.Join(c.dir, strconv.FormatInt(f.ID, 10), key+".ass")
	if b, err := os.ReadFile(path); err == nil {
		return b, nil
	}
	bin := c.ffmpeg()
	if bin == "" {
		return nil, errors.New("FFmpeg wasn't found, so subtitles can't be read")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, bin, "-hide_banner", "-nostdin", "-v", "error",
		"-i", f.Path, "-map", "0:"+strconv.Itoa(t.StreamIndex), "-c:s", "copy", "-f", "ass", "pipe:1")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("couldn't read the subtitles: %s", lastLine(stderr.String()))
	}
	out = toUTF8(out)
	writeAtomic(path, out)
	return out, nil
}

// Fonts returns the fonts attached to a video file (none for files without attachments),
// extracting them the first time and caching them.
func (c *Cache) Fonts(ctx context.Context, f *store.MediaFile) ([]Font, error) {
	key, err := sourceKey(f.Path, "fonts-safe-v2")
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(c.dir, strconv.FormatInt(f.ID, 10), "fonts-"+key)
	if _, err := os.Stat(filepath.Join(dir, ".done")); err != nil {
		bin := c.ffmpeg()
		if bin == "" {
			return nil, errors.New("FFmpeg wasn't found, so the video's fonts can't be read")
		}
		if err := os.MkdirAll(filepath.Dir(dir), 0755); err != nil {
			return nil, err
		}
		tmp, err := os.MkdirTemp(filepath.Dir(dir), "fonts-*.tmp")
		if err != nil {
			return nil, err
		}
		defer os.RemoveAll(tmp)
		ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
		defer cancel()
		names, err := extractFonts(ctx, bin, f.Path, tmp)
		if err != nil {
			return nil, err
		}
		for _, name := range names {
			if _, err = os.Stat(filepath.Join(tmp, name)); err != nil {
				return nil, errors.New("couldn't extract the video's fonts")
			}
		}
		if err = os.WriteFile(filepath.Join(tmp, ".done"), nil, 0644); err != nil {
			return nil, err
		}
		if err = os.Rename(tmp, dir); err != nil {
			// Another request may have published the same source atomically.
			if _, doneErr := os.Stat(filepath.Join(dir, ".done")); doneErr != nil {
				return nil, err
			}
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var fonts []Font
	total := 0
	for _, e := range entries {
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if e.IsDir() || !slices.Contains([]string{".ttf", ".otf", ".ttc", ".otc"}, ext) {
			continue
		}
		info, err := e.Info()
		if err != nil || !info.Mode().IsRegular() || info.Size() > int64(maxFontBytes-total) {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil || total+len(b) > maxFontBytes {
			continue
		}
		total += len(b)
		fonts = append(fonts, Font{Name: e.Name(), Data: b})
	}
	return fonts, nil
}

// extractFonts never delegates paths to attachment metadata. Probe only attachment headers,
// validate portable basenames, then explicitly name every indexed output inside our temp dir.
func extractFonts(ctx context.Context, bin, source, dir string) ([]string, error) {
	resolved, err := exec.LookPath(bin)
	if err != nil {
		return nil, err
	}
	probeName := "ffprobe"
	if strings.EqualFold(filepath.Ext(resolved), ".exe") {
		probeName += ".exe"
	}
	probeBin := filepath.Join(filepath.Dir(resolved), probeName)
	if _, err = os.Stat(probeBin); err != nil {
		probeBin, err = exec.LookPath(probeName)
		if err != nil {
			return nil, errors.New("FFprobe wasn't found, so the video's fonts can't be listed")
		}
	}
	cmd := exec.CommandContext(ctx, probeBin, "-v", "error", "-select_streams", "t", "-show_entries", "stream=index,extradata_size:stream_tags=filename", "-of", "json", source)
	data, err := cmd.Output()
	if err != nil {
		return nil, errors.New("couldn't list the video's fonts")
	}
	var result struct {
		Streams []struct {
			Index int   `json:"index"`
			Size  int64 `json:"extradata_size"`
			Tags  struct {
				Filename string `json:"filename"`
			} `json:"tags"`
		} `json:"streams"`
	}
	if err = json.Unmarshal(data, &result); err != nil {
		return nil, errors.New("couldn't read the video's font list")
	}
	args := []string{"-hide_banner", "-nostdin", "-v", "error", "-y"}
	var names []string
	total := int64(0)
	for _, stream := range result.Streams {
		name := stream.Tags.Filename
		if !safeFontName(name) || stream.Index < 0 || stream.Size < 0 || stream.Size > int64(maxFontBytes)-total {
			continue
		}
		total += stream.Size
		args = append(args, "-dump_attachment:"+strconv.Itoa(stream.Index), filepath.Join(dir, name))
		names = append(names, name)
	}
	if len(names) == 0 {
		return names, nil
	}
	args = append(args, "-i", source)
	// Input-only FFmpeg exits nonzero after dumping attachments because there is no media output.
	// Verify expected files above; cancellation is never cached as success.
	_ = exec.CommandContext(ctx, resolved, args...).Run()
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	return names, nil
}
func safeFontName(name string) bool {
	return name != "" && !filepath.IsAbs(name) && !strings.Contains(name, "..") && !strings.ContainsAny(name, "/\\:") && strings.IndexFunc(name, unicode.IsControl) < 0 && slices.Contains([]string{".ttf", ".otf", ".ttc", ".otc"}, strings.ToLower(filepath.Ext(name)))
}

// PackScript moves every event in an ASS script by shiftMS (negative is earlier; events that end
// before 0 are left out, as for WebVTT) and appends fonts in a [Fonts] section, which libass reads.
func PackScript(script []byte, shiftMS int64, fonts []Font) []byte {
	lines := strings.Split(strings.ReplaceAll(string(script), "\r\n", "\n"), "\n")
	var b strings.Builder
	section := ""
	start, end := 1, 2 // Start and End in the usual Dialogue format
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			section = strings.ToLower(trimmed)
			// An existing [Fonts] section is kept as it is; the video's fonts are added after.
		}
		if section == "[events]" && shiftMS != 0 {
			key, rest, ok := strings.Cut(trimmed, ":")
			switch {
			case ok && strings.EqualFold(key, "Format"):
				for i, name := range strings.Split(rest, ",") {
					switch strings.ToLower(strings.TrimSpace(name)) {
					case "start":
						start = i
					case "end":
						end = i
					}
				}
			case ok && (strings.EqualFold(key, "Dialogue") || strings.EqualFold(key, "Comment")):
				shifted, keep := shiftEvent(rest, start, end, shiftMS)
				if !keep {
					continue
				}
				line = key + ":" + shifted
			}
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
	if len(fonts) > 0 {
		b.WriteString("\n[Fonts]\n")
		for _, f := range fonts {
			b.WriteString("fontname: ")
			b.WriteString(f.Name)
			b.WriteByte('\n')
			b.WriteString(uuencodeASS(f.Data))
		}
	}
	return []byte(b.String())
}

// shiftEvent moves an event's start and end; false when it ends before 0.
func shiftEvent(fields string, startIdx, endIdx int, shiftMS int64) (string, bool) {
	// Only the first fields are split: the text (last) may hold commas.
	n := max(startIdx, endIdx) + 2
	parts := strings.SplitN(fields, ",", n)
	if len(parts) < n {
		return fields, true
	}
	s, okS := assTime(parts[startIdx])
	e, okE := assTime(parts[endIdx])
	if !okS || !okE {
		return fields, true
	}
	s, e = s+shiftMS, e+shiftMS
	if e <= 0 {
		return "", false
	}
	parts[startIdx] = formatASSTime(max(s, 0))
	parts[endIdx] = formatASSTime(e)
	return strings.Join(parts, ","), true
}

// assTime reads H:MM:SS.cc (centiseconds) as milliseconds.
func assTime(s string) (int64, bool) {
	s = strings.TrimSpace(s)
	hms := strings.Split(s, ":")
	if len(hms) != 3 {
		return 0, false
	}
	h, err1 := strconv.ParseInt(hms[0], 10, 64)
	m, err2 := strconv.ParseInt(hms[1], 10, 64)
	secs, err3 := strconv.ParseFloat(hms[2], 64)
	if err1 != nil || err2 != nil || err3 != nil {
		return 0, false
	}
	return (h*3600+m*60)*1000 + int64(secs*1000+0.5), true
}

func formatASSTime(ms int64) string {
	cs := (ms + 5) / 10
	return fmt.Sprintf("%d:%02d:%02d.%02d", cs/360000, cs/6000%60, cs/100%60, cs%100)
}

// uuencodeASS encodes a font as the ASS spec's [Fonts] section wants: each 3 bytes become 4
// characters (6 bits each, plus 33), a partial last group becomes 2 or 3, in lines of 80.
func uuencodeASS(data []byte) string {
	var out []byte
	for i := 0; i < len(data); i += 3 {
		var chunk [3]byte
		n := copy(chunk[:], data[i:])
		v := uint32(chunk[0])<<16 | uint32(chunk[1])<<8 | uint32(chunk[2])
		chars := [4]byte{byte(v>>18&63) + 33, byte(v>>12&63) + 33, byte(v>>6&63) + 33, byte(v&63) + 33}
		out = append(out, chars[:n+1]...)
	}
	var b strings.Builder
	for len(out) > 80 {
		b.Write(out[:80])
		b.WriteByte('\n')
		out = out[80:]
	}
	if len(out) > 0 {
		b.Write(out)
		b.WriteByte('\n')
	}
	return b.String()
}

// sourceKey names a cache entry for a file as it is now (its path, size and modification time).
func sourceKey(path, what string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%d|%d|%s", path, info.Size(), info.ModTime().Unix(), what)))
	return hex.EncodeToString(sum[:12]), nil
}

func writeAtomic(path string, b []byte) {
	if os.MkdirAll(filepath.Dir(path), 0o755) != nil {
		return
	}
	tmp := path + ".tmp"
	if os.WriteFile(tmp, b, 0o644) == nil {
		os.Rename(tmp, path)
	}
}

func lastLine(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.LastIndex(s, "\n"); i >= 0 {
		return s[i+1:]
	}
	return s
}
