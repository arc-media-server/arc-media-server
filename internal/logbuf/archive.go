package logbuf

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
)

const archiveLimit = 2 << 20
const archiveCount = 3
const recordLimit = 8 << 10

type archive struct {
	path  string
	file  *os.File
	size  int64
	limit int64
}

func (a *archive) open() error {
	f, err := os.OpenFile(a.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return err
	}
	a.file, a.size = f, info.Size()
	return nil
}

func (a *archive) append(line Line) error {
	if len(line.Message) > recordLimit {
		line.Message = line.Message[:recordLimit] + " [truncated]"
	}
	if len(line.Attrs) > recordLimit {
		line.Attrs = line.Attrs[:recordLimit] + " [truncated]"
	}
	data, err := json.Marshal(line)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	if a.file == nil {
		if err := a.open(); err != nil {
			return err
		}
	}
	if a.size+int64(len(data)) > a.limit {
		a.file.Close()
		a.file = nil
		for i := archiveCount - 1; i > 0; i-- {
			from := a.path
			if i > 1 {
				from += "." + strconv.Itoa(i-1)
			}
			to := a.path + "." + strconv.Itoa(i)
			if err := os.Remove(to); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
			if err := os.Rename(from, to); err != nil && !errors.Is(err, os.ErrNotExist) {
				return err
			}
		}
		if err := a.open(); err != nil {
			return err
		}
	}
	n, err := a.file.Write(data)
	a.size += int64(n)
	return err
}

func (a *archive) records() ([]Line, error) {
	var records []Line
	for i := archiveCount - 1; i >= 0; i-- {
		path := a.path
		if i > 0 {
			path += "." + strconv.Itoa(i)
		}
		f, err := os.Open(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		s := bufio.NewScanner(io.LimitReader(f, a.limit+1))
		s.Buffer(make([]byte, 4096), 256<<10)
		for s.Scan() {
			var l Line
			if json.Unmarshal(s.Bytes(), &l) == nil {
				records = append(records, l)
			}
		}
		err = s.Err()
		f.Close()
		if err != nil {
			return nil, err
		}
	}
	return records, nil
}

// EnableArchive retains at most three 2MiB files and reloads recent records after restart.
func (b *Buffer) EnableArchive(dir string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	fail := func(err error) error { b.archiveError = err.Error(); return err }
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fail(err)
	}
	a := &archive{path: filepath.Join(dir, "arc.jsonl"), limit: archiveLimit}
	old, err := a.records()
	if err != nil {
		return fail(err)
	}
	if err := a.open(); err != nil {
		return fail(err)
	}
	if b.archive != nil {
		b.archive.file.Close()
	}
	for _, l := range old {
		b.seq = max(b.seq, l.Seq)
		b.keep(l)
	}
	b.archive, b.archiveError = a, ""
	return nil
}

func (b *Buffer) Close() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.timer != nil {
		b.timer.Stop()
	}
	if b.archive == nil || b.archive.file == nil {
		return nil
	}
	err := b.archive.file.Close()
	b.archive = nil
	return err
}

// Export returns retained, freshly redacted JSONL; no settings database or media is included.
func (b *Buffer) Export() ([]byte, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	var records []Line
	if b.archive != nil {
		var err error
		records, err = b.archive.records()
		if err != nil {
			b.archiveError = fmt.Sprintf("read retained logs: %v", err)
			records = nil
		}
	}
	var last int64
	for _, l := range records {
		last = max(last, l.Seq)
	}
	// Include records that reached memory when retained storage was unavailable.
	for i := range b.lines {
		l := b.lines[(b.next+i)%len(b.lines)]
		if l.Seq > last {
			records = append(records, l)
		}
	}
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	for _, l := range records {
		l.Message, l.Attrs = b.redact(l.Message), b.redact(l.Attrs)
		if err := enc.Encode(l); err != nil {
			return nil, err
		}
	}
	return out.Bytes(), nil
}
