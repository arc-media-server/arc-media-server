// Command testmedia creates a small fake media library for development: a few movies and TV
// series named the way Sonarr/Radarr would (TVDB aired order), all copies of one tiny clip
// generated with ffmpeg. The names are real so metadata matching and orderings can be tested.
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
