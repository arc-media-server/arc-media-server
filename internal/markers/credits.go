package markers

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"ams/internal/store"
)

// Finding a movie's end credits on this computer, when its chapters and TheIntroDB don't say.
// Credits are nearly always light text on a dark, colourless background, unlike the film before
// them: the end of the movie is sampled once a second, and the credits are the last long stretch
// where nearly every frame is dark and grey. Credits over pictures aren't found, and that's fine:
// nothing is better than a wrong Skip Credits.

const (
	creditsLookSec = 20 * 60 // how much of the end to look at, at most
	creditsMinSec  = 45      // a stretch shorter than this isn't the credits
	creditsShare   = 0.8     // how many frames of the stretch must look like credits
)

// creditFrame is whether a frame looks like credits: dark and nearly without colour.
func creditFrame(yAvg, satAvg float64) bool { return yAvg < 70 && satAvg < 10 }

// detectCredits finds a movie's credits, or nil.
func detectCredits(ctx context.Context, ffmpeg, path string, durationMS int64) (*store.Marker, error) {
	if ffmpeg == "" || durationMS < 20*60*1000 {
		return nil, nil // short videos aren't feature films
	}
	look := min(int64(creditsLookSec), durationMS/1000/4)
	from := durationMS/1000 - look
	cmd := exec.CommandContext(ctx, ffmpeg, "-hide_banner", "-nostats", "-loglevel", "error",
		// Keyframes only (every few seconds in films) keeps this quick even for 4K.
		"-skip_frame", "nokey", "-ss", strconv.FormatInt(from, 10), "-i", path, "-map", "0:v:0", "-an", "-sn",
		"-vf", "fps=1,scale=160:-2,signalstats,metadata=mode=print:file=-", "-f", "null", "-")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("measuring %s: %w", path, err)
	}
	frames := parseSignalStats(out)
	start, end, ok := creditsRun(frames)
	if !ok {
		return nil, nil
	}
	m := &store.Marker{Kind: "credits", StartMS: (from + int64(start)) * 1000, EndMS: (from + int64(end)) * 1000, Source: "detected"}
	if m.EndMS > durationMS-5000 {
		m.EndMS = durationMS
	}
	// The credits come after most of the movie.
	if m.StartMS < durationMS*6/10 {
		return nil, nil
	}
	return m, nil
}

// parseSignalStats reads, per sampled frame, whether it looks like credits.
func parseSignalStats(out []byte) []bool {
	var frames []bool
	var y, sat float64
	haveY, haveSat := false, false
	flush := func() {
		if haveY && haveSat {
			frames = append(frames, creditFrame(y, sat))
		}
		haveY, haveSat = false, false
	}
	sc := bufio.NewScanner(bytes.NewReader(out))
	for sc.Scan() {
		line := sc.Text()
		switch {
		case strings.HasPrefix(line, "frame:"):
			flush()
		case strings.HasPrefix(line, "lavfi.signalstats.YAVG="):
			y, _ = strconv.ParseFloat(strings.TrimPrefix(line, "lavfi.signalstats.YAVG="), 64)
			haveY = true
		case strings.HasPrefix(line, "lavfi.signalstats.SATAVG="):
			sat, _ = strconv.ParseFloat(strings.TrimPrefix(line, "lavfi.signalstats.SATAVG="), 64)
			haveSat = true
		}
	}
	flush()
	return frames
}

// creditsRun finds the credits among one-a-second frames: the earliest credit-like frame from which
// at least creditsShare of the rest look like credits, up to the last credit-like frame (a scene
// after the credits stays playable).
func creditsRun(frames []bool) (start, end int, ok bool) {
	last := -1
	for i := len(frames) - 1; i >= 0; i-- {
		if frames[i] {
			last = i
			break
		}
	}
	if last < 0 {
		return 0, 0, false
	}
	start = -1
	count := 0
	for i := last; i >= 0; i-- {
		if frames[i] {
			count++
			if float64(count)/float64(last-i+1) >= creditsShare {
				start = i
			}
		}
	}
	if start < 0 || last-start+1 < creditsMinSec {
		return 0, 0, false
	}
	return start, last + 1, true
}
