package transcode

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const conversionSpaceReserve uint64 = 512 << 20
const historySegments = 120 / segmentSeconds

var ErrStorageFull = errors.New("not enough free space for playback conversion; free some space and try again")

func (m *Manager) available(path string) (uint64, error) {
	if m.freeSpace == nil {
		return ^uint64(0), nil
	}
	free, err := m.freeSpace(path)
	if err != nil {
		m.diskWarning.Do(func() { m.log.Warn("conversion free-space check unavailable; filesystem write errors remain enforced") })
	}
	return free, err
}
func (m *Manager) checkStorage() {
	m.mu.Lock()
	sessions := make([]*Session, 0, len(m.sessions))
	for _, s := range m.sessions {
		sessions = append(sessions, s)
	}
	m.mu.Unlock()
	type capacity struct {
		free uint64
		err  error
	}
	capacities := map[string]capacity{}
	for _, s := range sessions {
		root := filepath.Dir(s.dir)
		c, ok := capacities[root]
		if !ok {
			c.free, c.err = m.available(root)
			capacities[root] = c
		}
		if c.err != nil || c.free >= conversionSpaceReserve {
			continue
		}
		s.mu.Lock()
		select {
		case <-s.done:
			s.mu.Unlock()
			continue
		default:
		}
		if s.err != nil {
			s.mu.Unlock()
			continue
		}
		s.err = ErrStorageFull
		s.mu.Unlock()
		s.cancel()
		m.log.Warn("conversion stopped for low free space", "session", s.ID, "free_bytes", c.free, "reserve_bytes", conversionSpaceReserve)
	}
}

// Only clients which can restart for older seeks opt in. Keep two minutes behind successful
// segment requests; nonexistent/future names cannot advance retention. Pacing stays independent.
func (s *Session) pruneHistory(requested int) {
	if !s.Options.RollingHLS || requested <= historySegments {
		return
	}
	if _, err := os.Stat(filepath.Join(s.dir, fmt.Sprintf("s%05d.m4s", requested))); err != nil {
		return
	}
	s.storageMu.Lock()
	defer s.storageMu.Unlock()
	stop := min(requested-historySegments, s.retainedFrom+64)
	for s.retainedFrom < stop {
		err := os.Remove(filepath.Join(s.dir, fmt.Sprintf("s%05d.m4s", s.retainedFrom)))
		if err != nil && !os.IsNotExist(err) {
			break
		} // retry later, including Windows open-file retention
		s.retainedFrom++
	}
}
func (s *Session) windowPlaylist(b []byte) []byte {
	if !s.Options.RollingHLS {
		return b
	}
	s.storageMu.Lock()
	from := s.retainedFrom
	s.storageMu.Unlock()
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	var out strings.Builder
	var block []string
	inSegments := false
	for _, line := range lines {
		if strings.HasPrefix(line, "#EXT-X-PLAYLIST-TYPE:") {
			continue
		}
		if strings.HasPrefix(line, "#EXT-X-MEDIA-SEQUENCE:") {
			fmt.Fprintf(&out, "#EXT-X-MEDIA-SEQUENCE:%d\n", from)
			continue
		}
		if strings.HasPrefix(line, "#EXTINF:") {
			inSegments = true
			block = append(block, line)
			continue
		}
		if !inSegments {
			out.WriteString(line)
			out.WriteByte('\n')
			continue
		}
		if strings.HasPrefix(line, "s") && fileRe.MatchString(line) {
			index, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(line, "s"), ".m4s"))
			if err == nil && index >= from {
				for _, tag := range block {
					out.WriteString(tag)
					out.WriteByte('\n')
				}
				out.WriteString(line)
				out.WriteByte('\n')
			}
			block = nil
			continue
		}
		block = append(block, line)
	}
	for _, tag := range block {
		out.WriteString(tag)
		out.WriteByte('\n')
	}
	return []byte(out.String())
}
