package trickplay

import (
	"ams/internal/store"
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteBIF(t *testing.T) {
	// Two sheets of 2×2 thumbnails, 5 thumbnails in all, each a different shade so a cut in the
	// wrong place shows.
	info := &Info{IntervalMS: 10_000, Width: 16, Height: 8, Columns: 2, Rows: 2, Count: 5, Sheets: 2}
	shade := func(n int) uint8 { return uint8(40 * (n + 1)) }
	sheet := func(s int) (image.Image, error) {
		img := image.NewGray(image.Rect(0, 0, 32, 16))
		for pos := 0; pos < 4; pos++ {
			for y := 0; y < 8; y++ {
				for x := 0; x < 16; x++ {
					img.SetGray((pos%2)*16+x, (pos/2)*8+y, color.Gray{shade(s*4 + pos)})
				}
			}
		}
		return img, nil
	}
	var buf bytes.Buffer
	if err := writeBIF(&buf, info, sheet); err != nil {
		t.Fatal(err)
	}
	b := buf.Bytes()
	if !bytes.Equal(b[:8], bifMagic) {
		t.Fatalf("magic %x", b[:8])
	}
	le := binary.LittleEndian
	if n := le.Uint32(b[12:]); n != 5 {
		t.Fatalf("count %d", n)
	}
	if ms := le.Uint32(b[16:]); ms != 10_000 {
		t.Fatalf("interval %d", ms)
	}
	if last := le.Uint32(b[64+8*5:]); last != 0xffffffff {
		t.Fatalf("index end marker %x", last)
	}
	if end := le.Uint32(b[64+8*5+4:]); int(end) != len(b) {
		t.Fatalf("end offset %d, file is %d bytes", end, len(b))
	}
	for i := 0; i < 5; i++ {
		if ts := le.Uint32(b[64+8*i:]); ts != uint32(i) {
			t.Errorf("thumbnail %d time %d", i, ts)
		}
		start, end := le.Uint32(b[64+8*i+4:]), le.Uint32(b[64+8*(i+1)+4:])
		img, err := jpeg.Decode(bytes.NewReader(b[start:end]))
		if err != nil {
			t.Fatalf("thumbnail %d: %v", i, err)
		}
		if img.Bounds().Dx() != 16 || img.Bounds().Dy() != 8 {
			t.Errorf("thumbnail %d is %v", i, img.Bounds())
		}
		r, _, _, _ := img.At(8, 4).RGBA()
		if got, want := int(r>>8), int(shade(i)); got < want-6 || got > want+6 {
			t.Errorf("thumbnail %d shade %d, want about %d", i, got, want)
		}
	}
}

func TestManagerBIF(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(t.Context(), filepath.Join(dir, "store.db"), store.Options{})
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	lib, err := st.SyncLibrary(t.Context(), "Fixture", "movies", "tmdb_aired", []string{"/fixture"})
	if err != nil {
		t.Fatal(err)
	}
	id, err := st.SaveFile(t.Context(), &store.MediaFile{LibraryID: lib, Path: "/fixture/clip.mkv", Size: 1, ModTime: 1}, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	m := New(dir, st, nil, nil)
	if _, err := m.BIF(id); err != ErrNotMade {
		t.Fatalf("before thumbnails: %v", err)
	}
	fd, _ := m.sourceDir(id, m.sourceKey(id))
	os.MkdirAll(fd, 0o755)
	os.WriteFile(filepath.Join(fd, "info.json"), []byte(`{"interval_ms":10000,"width":16,"height":8,"columns":2,"rows":2,"count":3,"sheets":1}`), 0o644)
	f, _ := os.Create(filepath.Join(fd, "0.jpg"))
	jpeg.Encode(f, image.NewGray(image.Rect(0, 0, 32, 16)), nil)
	f.Close()
	path, err := m.BIF(id)
	if err != nil {
		t.Fatal(err)
	}
	first, _ := os.Stat(path)
	if again, err := m.BIF(id); err != nil || again != path {
		t.Fatalf("second ask: %q %v", again, err)
	}
	if second, _ := os.Stat(path); !second.ModTime().Equal(first.ModTime()) {
		t.Error("the BIF file was made again instead of kept")
	}
	if tmps, _ := filepath.Glob(filepath.Join(fd, "*.tmp")); len(tmps) != 0 {
		t.Errorf("left behind: %v", tmps)
	}
	// Remaking the thumbnails removes the folder, and the BIF file with it.
	m.Forget(id)
	if _, err := m.BIF(id); err != ErrNotMade {
		t.Fatalf("after forgetting: %v", err)
	}
}
