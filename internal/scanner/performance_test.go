package scanner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"ams/internal/store"
)

// Measures first grouping, unchanged rescans and a single addition independently of
// provider latency/ffprobe. All files are synthetic and seeded as already probed.
// This is a local filesystem/SQLite benchmark, not a NAS or end-to-end import result.
func BenchmarkScanWork(b *testing.B) {
	for _, kind := range []string{"movies", "shows"} {
		for _, mode := range []string{"first", "unchanged", "add-one"} {
			b.Run(kind+"/"+mode, func(b *testing.B) {
				files := make(map[string]string, 1800)
				for i := range 1800 {
					name := fmt.Sprintf("Fixture %04d (2000)/Fixture %04d (2000).mkv", i, i)
					if kind == "shows" {
						name = fmt.Sprintf("Show %02d (2000)/Season 01/Show %02d - S01E%02d.mkv", i/60, i/60, i%60+1)
					}
					files[name] = "synthetic pre-probed file"
				}
				b.StopTimer()
				st, s, lib, root := libraryFixture(b, kind, files)
				ctx := context.Background()
				if mode != "first" {
					if err := s.scan(ctx, lib); err != nil {
						b.Fatal(err)
					}
				}
				b.ResetTimer()
				for i := range b.N {
					// A fresh store is needed for each first-grouping iteration.
					if mode == "first" && i > 0 {
						st, s, lib, root = libraryFixture(b, kind, files)
					}
					if mode == "add-one" {
						name := fmt.Sprintf("New Movie %04d (2000)/New Movie %04d (2000).mkv", i, i)
						if kind == "shows" {
							name = fmt.Sprintf("Show 00 (2000)/Season 02/Show 00 - S02E%02d.mkv", i+1)
						}
						path := filepath.Join(root, filepath.FromSlash(name))
						if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
							b.Fatal(err)
						}
						if err := os.WriteFile(path, []byte("new synthetic file"), 0644); err != nil {
							b.Fatal(err)
						}
						info, err := os.Stat(path)
						if err != nil {
							b.Fatal(err)
						}
						if _, err := st.SaveFile(ctx, &store.MediaFile{LibraryID: lib, Path: path, Size: info.Size(), ModTime: info.ModTime().Unix()}, 1, 1); err != nil {
							b.Fatal(err)
						}
					}
					s.status[lib] = &Status{}
					b.StartTimer()
					if err := s.scan(ctx, lib); err != nil {
						b.Fatal(err)
					}
					b.StopTimer()
				}
			})
		}
	}
}
