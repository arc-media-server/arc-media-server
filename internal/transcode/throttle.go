package transcode

import (
	"bytes"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Throttling, as Plex's throttle buffer: a conversion runs at full speed until it's this far ahead
// of what the player last asked for, then waits (FFmpeg is paused, using no CPU or disk) until the
// player catches up to within resumeAhead or asks for something not converted yet. Someone who
// stops after ten minutes no longer leaves a whole film converted behind them, and a paused
// viewer's conversion stops too. Seeking past what's converted still starts a new session.
//
// resumeAhead is one segment short of pauseAhead, so while someone watches, the playlist grows a
// segment at a time rather than standing still. Players treat a live playlist that doesn't change
// for a few segment lengths as stuck: Android's Media3 fails it after 3.5 target durations, which
// with a minute-long pause made throttled streams spin and fall back to converting (owner's
// Chromecast, 31 October).
type throttling struct {
	pauseAhead, resumeAhead, tick time.Duration
}

// segmentSeconds is the length of each converted segment (-hls_time).
const segmentSeconds = 6

var defaultThrottling = throttling{pauseAhead: 120 * time.Second, resumeAhead: 120*time.Second - segmentSeconds*time.Second, tick: time.Second}

// Requested records that the player fetched an output file: a media segment moves the point the
// conversion keeps ahead of, and asking for one not made yet resumes it at once.
func (s *Session) Requested(name string) {
	if !strings.HasPrefix(name, "s") || !strings.HasSuffix(name, ".m4s") {
		return
	}
	n, err := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(name, "s"), ".m4s"))
	if err != nil {
		return
	}
	s.mu.Lock()
	s.requested = n
	s.mu.Unlock()
	if _, err := os.Stat(filepath.Join(s.dir, name)); err != nil {
		s.setPaused(false)
	}
}

// Throttled reports whether the conversion is waiting because it's far enough ahead.
func (s *Session) Throttled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.paused
}

// throttle keeps the conversion within pauseAhead of the player until FFmpeg finishes.
func (s *Session) throttle() {
	t := time.NewTicker(s.throttling.tick)
	defer t.Stop()
	for {
		select {
		case <-s.done:
			return
		case <-t.C:
		}
		made := s.segments()
		s.mu.Lock()
		at := max(s.requested, 0)
		s.mu.Unlock()
		ahead := time.Duration(made-1-at) * segmentSeconds * time.Second
		switch {
		case ahead >= s.throttling.pauseAhead:
			s.setPaused(true)
		case ahead <= s.throttling.resumeAhead:
			s.setPaused(false)
		}
	}
}

// ConvertedMS is how far into the file the conversion has got: where it started plus the
// segments finished (each about segmentSeconds long).
func (s *Session) ConvertedMS() int64 {
	return s.OffsetMS + int64(s.segments())*segmentSeconds*1000
}

// segments is how many media segments are finished (listed in the playlist).
func (s *Session) segments() int {
	b, err := os.ReadFile(filepath.Join(s.dir, "index.m3u8"))
	if err != nil {
		return 0
	}
	return bytes.Count(b, []byte("#EXTINF"))
}

func (s *Session) setPaused(paused bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.paused == paused || s.proc == nil {
		return
	}
	select {
	case <-s.done:
		return
	default:
	}
	var err error
	if paused {
		err = suspend(s.proc)
	} else {
		err = resume(s.proc)
	}
	if err != nil {
		// Where pausing isn't possible the conversion simply runs at full speed, as before.
		s.proc = nil
		return
	}
	s.paused = paused
}
