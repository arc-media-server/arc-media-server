package scanner

import (
	"io/fs"
	"os"
	"time"
)

const probeFailureBackoff = 15 * time.Minute
const maxRecentProbeFailures = 4096

type probeFileStamp struct {
	path, binary string
	size, mtime  int64
	mode         fs.FileMode
}
type recentProbeFailure struct {
	stamp probeFileStamp
	until time.Time
}

func (s *Scanner) probeRetryTime() time.Time {
	if s.probeRetryNow != nil {
		return s.probeRetryNow()
	}
	return time.Now()
}

func (s *Scanner) probeStamp(path string, size, mtime int64) (probeFileStamp, bool) {
	info, err := os.Stat(path)
	if err != nil || info.Size() != size || info.ModTime().Unix() != mtime {
		return probeFileStamp{}, false
	}
	return probeFileStamp{path: path, binary: s.prober.Bin(), size: size, mtime: info.ModTime().UnixNano(), mode: info.Mode()}, true
}

func (s *Scanner) forgetProbeFailure(id int64) {
	s.probeRetryMu.Lock()
	delete(s.probeFailures, id)
	s.probeRetryMu.Unlock()
}

func (s *Scanner) deferRecentProbeFailure(id int64, path string, size, mtime int64, problem string) bool {
	s.probeRetryMu.Lock()
	entry := s.probeFailures[id]
	s.probeRetryMu.Unlock()
	if entry == nil {
		return false
	}
	stamp, valid := s.probeStamp(path, size, mtime)
	if problem != "" && valid && stamp == entry.stamp && s.probeRetryTime().Before(entry.until) {
		return true
	}
	s.probeRetryMu.Lock()
	if s.probeFailures[id] == entry {
		delete(s.probeFailures, id)
	}
	s.probeRetryMu.Unlock()
	return false
}

func (s *Scanner) rememberProbeFailure(id int64, stamp probeFileStamp) {
	now := s.probeRetryTime()
	s.probeRetryMu.Lock()
	defer s.probeRetryMu.Unlock()
	if s.probeFailures == nil {
		s.probeFailures = map[int64]*recentProbeFailure{}
	}
	if len(s.probeFailures) >= maxRecentProbeFailures && s.probeFailures[id] == nil {
		// Keep unexpired entries rather than thrash the cache during a large broken import.
		// Expired/deleted-file hints are swept at most once a minute, never once per failed file.
		if !now.Before(s.probeRetrySweep) {
			for key, entry := range s.probeFailures {
				if !now.Before(entry.until) {
					delete(s.probeFailures, key)
				}
			}
			s.probeRetrySweep = now.Add(time.Minute)
		}
		if len(s.probeFailures) >= maxRecentProbeFailures {
			return
		}
	}
	s.probeFailures[id] = &recentProbeFailure{stamp: stamp, until: now.Add(probeFailureBackoff)}
}
