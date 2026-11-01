package trickplay

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"os"
	"path/filepath"
	"strconv"
)

// ErrNotMade means a file's thumbnails haven't been made yet.
var ErrNotMade = errors.New("thumbnails not made yet")

// bifMagic starts every BIF file (Roku's "base index frames" format for timeline thumbnails).
var bifMagic = []byte{0x89, 'B', 'I', 'F', '\r', '\n', 0x1a, '\n'}

// BIF returns the path of a file's thumbnails as one BIF file, the format Roku's player shows
// while seeking. It is cut from the sheets the first time it's asked for and kept beside them, so
// it goes when they're remade.
func (m *Manager) BIF(fileID int64) (string, error) {
	info := m.Info(fileID)
	if info == nil {
		return "", ErrNotMade
	}
	dir := m.fileDir(fileID)
	path := filepath.Join(dir, "thumbs.bif")
	m.bifMu.Lock()
	defer m.bifMu.Unlock()
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	tmp, err := os.CreateTemp(dir, "bif-*.tmp")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	err = writeBIF(tmp, info, func(n int) (image.Image, error) {
		f, err := os.Open(filepath.Join(dir, strconv.Itoa(n)+".jpg"))
		if err != nil {
			return nil, err
		}
		defer f.Close()
		return jpeg.Decode(f)
	})
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return "", err
	}
	return path, os.Rename(tmp.Name(), path)
}

// writeBIF writes info's thumbnails, cut from the sheets sheet(n) returns, as a BIF file: a
// 64-byte header, an index of (time, offset) pairs ending with 0xffffffff and the end offset,
// then each thumbnail as a JPEG. Times count in units of the thumbnail interval.
func writeBIF(w io.Writer, info *Info, sheet func(n int) (image.Image, error)) error {
	if info.Count <= 0 || info.Columns <= 0 || info.Rows <= 0 || info.Width <= 0 || info.Height <= 0 {
		return fmt.Errorf("no thumbnails")
	}
	perSheet := info.Columns * info.Rows
	thumbs := make([][]byte, 0, info.Count)
	var buf bytes.Buffer
	for s := 0; len(thumbs) < info.Count; s++ {
		img, err := sheet(s)
		if err != nil {
			return fmt.Errorf("sheet %d: %w", s, err)
		}
		sub, ok := img.(interface {
			SubImage(image.Rectangle) image.Image
		})
		if !ok {
			return fmt.Errorf("sheet %d can't be cut", s)
		}
		for pos := 0; pos < perSheet && len(thumbs) < info.Count; pos++ {
			at := img.Bounds().Min.Add(image.Pt((pos%info.Columns)*info.Width, (pos/info.Columns)*info.Height))
			buf.Reset()
			if err := jpeg.Encode(&buf, sub.SubImage(image.Rectangle{Min: at, Max: at.Add(image.Pt(info.Width, info.Height))}), &jpeg.Options{Quality: 72}); err != nil {
				return err
			}
			thumbs = append(thumbs, bytes.Clone(buf.Bytes()))
		}
	}
	header := make([]byte, 64)
	copy(header, bifMagic)
	binary.LittleEndian.PutUint32(header[8:], 0) // version
	binary.LittleEndian.PutUint32(header[12:], uint32(len(thumbs)))
	binary.LittleEndian.PutUint32(header[16:], uint32(info.IntervalMS))
	index := make([]byte, 8*(len(thumbs)+1))
	offset := uint32(len(header) + len(index))
	for i, t := range thumbs {
		binary.LittleEndian.PutUint32(index[8*i:], uint32(i))
		binary.LittleEndian.PutUint32(index[8*i+4:], offset)
		offset += uint32(len(t))
	}
	binary.LittleEndian.PutUint32(index[8*len(thumbs):], 0xffffffff)
	binary.LittleEndian.PutUint32(index[8*len(thumbs)+4:], offset)
	if _, err := w.Write(header); err != nil {
		return err
	}
	if _, err := w.Write(index); err != nil {
		return err
	}
	for _, t := range thumbs {
		if _, err := w.Write(t); err != nil {
			return err
		}
	}
	return nil
}
