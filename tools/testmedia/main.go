// Command testmedia creates a small fake media library for development: a few movies and TV
// series named the way Sonarr/Radarr would (TVDB aired order), all copies of one tiny clip
// generated with ffmpeg, plus a couple of files browsers can't play as they are (to test
// conversion). The names are real so metadata matching and orderings can be tested.
//
//	go run ./tools/testmedia -out ../ams-test-media
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type series struct {
	folder  string
	name    string
	seasons map[int]int // season number -> episode count (0 = specials)
}

var movies = []string{
	"The Matrix (1999)/The Matrix (1999).mp4",
	"The Matrix (1999)/The Matrix (1999) - 1080p.mp4", // a second copy: shows as a duplicate
	"Blade.Runner.2049.2017.1080p.BluRay.x264-GRP.mp4",
	"Heat (1995)/Heat (1995) {tmdb-949}.mp4",
	"Heat (1995)/Extras/Making of.mp4", // should be skipped
	"Heat (1995)/heat-trailer.mp4",     // should be skipped
	// Two editions of one film, and a second (SD) version of Inception beside its 720p MKV.
	"Blade Runner (1982)/Blade Runner (1982) {edition-Final Cut}.mp4",
	"Blade Runner (1982)/Blade Runner (1982) {edition-Theatrical Cut}.mp4",
	"Inception (2010)/Inception (2010) - SD.mp4",
}

var shows = []series{
	// Aired order differs from DVD order and TMDB's "Intended Order" group.
	{"Firefly (2002)", "Firefly", map[int]int{0: 1, 1: 14}},
	// Several TMDB episode groups (Cours, Crunchyroll, Seasons, DVD).
	{"Mushoku Tensei - Jobless Reincarnation (2021)", "Mushoku Tensei - Jobless Reincarnation", map[int]int{0: 4, 1: 23, 2: 24, 3: 14}},
	// Doesn't exist anywhere: tests the unmatched path.
	{"Zzqx Imaginary Show", "Zzqx Imaginary Show", map[int]int{1: 3}},
}

func main() {
	out := flag.String("out", "ams-test-media", "folder to create the library in")
	ffmpeg := flag.String("ffmpeg", "ffmpeg", "ffmpeg binary")
	flag.Parse()

	if err := run(*out, *ffmpeg); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(out, ffmpeg string) error {
	out, err := filepath.Abs(out)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}
	clip := filepath.Join(out, ".sample.mp4")
	if _, err := os.Stat(clip); err != nil {
		fmt.Println("generating sample clip with ffmpeg...")
		cmd := exec.Command(ffmpeg, "-v", "error", "-y",
			"-f", "lavfi", "-i", "testsrc=duration=3:size=640x360:rate=24",
			"-f", "lavfi", "-i", "sine=duration=3",
			"-c:v", "libx264", "-c:a", "aac", "-shortest", clip)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("ffmpeg: %w", err)
		}
	}

	n := 0
	for _, m := range movies {
		if err := copyFile(clip, filepath.Join(out, "Movies", filepath.FromSlash(m))); err != nil {
			return err
		}
		n++
	}
	for _, s := range shows {
		for season, count := range s.seasons {
			dir := fmt.Sprintf("Season %02d", season)
			if season == 0 {
				dir = "Specials"
			}
			for e := 1; e <= count; e++ {
				name := fmt.Sprintf("%s - S%02dE%02d.mp4", s.name, season, e)
				if err := copyFile(clip, filepath.Join(out, "TV", s.folder, dir, name)); err != nil {
					return err
				}
				n++
			}
		}
	}
	// A file whose episode TVDB doesn't know about: tests file-only episodes.
	if err := copyFile(clip, filepath.Join(out, "TV", "Firefly (2002)", "Season 01", "Firefly - S01E20.mp4")); err != nil {
		return err
	}
	// A video in a show's folder that isn't named like an episode: not recognised.
	if err := copyFile(clip, filepath.Join(out, "TV", "Firefly (2002)", "Season 01", "Cast interview.mp4")); err != nil {
		return err
	}
	// A file ffprobe can't read: shows as unreadable.
	broken := filepath.Join(out, "Movies", "Broken Download (2010)", "Broken Download (2010).mkv")
	if err := os.MkdirAll(filepath.Dir(broken), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(broken, []byte("this is not a video file\n"), 0o644); err != nil {
		return err
	}
	n += 2

	// Local artwork: a plain orange poster.jpg in Heat's folder, used instead of TMDB's poster.
	poster := filepath.Join(out, "Movies", "Heat (1995)", "poster.jpg")
	if _, err := os.Stat(poster); err != nil {
		cmd := exec.Command(ffmpeg, "-v", "error", "-y", "-f", "lavfi", "-i", "color=c=0xff8c1a:s=400x600", "-frames:v", "1", poster)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("ffmpeg poster: %w", err)
		}
	}

	// Files a browser can't play as they are, for testing conversion: an MKV with two audio
	// tracks (AAC English, AC-3 French) and subtitles, and 10-bit HDR10 HEVC with E-AC-3.
	srt := "1\n00:00:01,000 --> 00:00:04,000\nHello from the subtitles\n\n2\n00:00:30,000 --> 00:00:33,000\nHalfway there\n"
	for _, f := range []struct {
		path string
		args []string
	}{
		{"Inception (2010)/Inception (2010).mkv", []string{
			"-f", "lavfi", "-i", "testsrc2=duration=60:size=1280x720:rate=24",
			"-f", "lavfi", "-i", "sine=f=440:duration=60", "-f", "lavfi", "-i", "sine=f=880:duration=60", "-i", "{srt}",
			// Chapters, for testing the player's chapter marks and list.
			"-i", "{chapters}", "-map_chapters", "4",
			"-map", "0", "-map", "1", "-map", "2", "-map", "3", "-c:v", "libx264", "-preset", "veryfast", "-g", "60",
			"-c:a:0", "aac", "-c:a:1", "ac3", "-c:s", "srt", "-metadata:s:a:0", "language=eng", "-metadata:s:a:1", "language=fra",
			"-metadata:s:s:0", "language=eng"}},
		{"Interstellar (2014)/Interstellar (2014).mkv", []string{
			"-f", "lavfi", "-i", "testsrc2=duration=20:size=1280x720:rate=24", "-f", "lavfi", "-i", "sine=duration=20",
			"-c:v", "libx265", "-preset", "ultrafast", "-pix_fmt", "yuv420p10le", "-x265-params", "log-level=error",
			// Tagged as HDR10 (BT.2020, PQ), for testing tone mapping.
			"-bsf:v", "hevc_metadata=colour_primaries=9:transfer_characteristics=16:matrix_coefficients=9", "-c:a", "eac3"}},
	} {
		dst := filepath.Join(out, "Movies", filepath.FromSlash(f.path))
		if _, err := os.Stat(dst); err == nil {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		sub := strings.TrimSuffix(dst, ".mkv") + ".en.srt"
		if err := os.WriteFile(sub, []byte(srt), 0o644); err != nil {
			return err
		}
		chapters := dst + ".chapters.txt"
		if err := os.WriteFile(chapters, []byte(chapterMetadata), 0o644); err != nil {
			return err
		}
		args := []string{"-v", "error", "-y"}
		for _, a := range f.args {
			args = append(args, strings.NewReplacer("{srt}", sub, "{chapters}", chapters).Replace(a))
		}
		cmd := exec.Command(ffmpeg, append(args, dst)...)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		err := cmd.Run()
		os.Remove(chapters)
		if err != nil {
			return fmt.Errorf("ffmpeg %s: %w", f.path, err)
		}
		n++
	}

	fmt.Printf("created %d files in %s\n\nIn Arc, add these as libraries (Manage > Libraries > Add Library):\n\n", n, out)
	fmt.Printf("  Movies:   %s\n", filepath.Join(out, "Movies"))
	fmt.Printf("  TV Shows: %s (TheTVDB aired order)\n", filepath.Join(out, "TV"))
	return nil
}

func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, in); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// chapterMetadata is an FFmpeg metadata file with four chapters across a minute.
const chapterMetadata = `;FFMETADATA1
[CHAPTER]
TIMEBASE=1/1000
START=0
END=12000
title=Opening
[CHAPTER]
TIMEBASE=1/1000
START=12000
END=30000
title=The Dream Within a Dream
[CHAPTER]
TIMEBASE=1/1000
START=30000
END=48000
title=Limbo
[CHAPTER]
TIMEBASE=1/1000
START=48000
END=60000
title=The Kick
`
