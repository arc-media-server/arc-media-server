package markers

import (
	"context"
	"encoding/binary"
	"fmt"
	"math/bits"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"time"

	"ams/internal/store"
)

// Finding intros and credits on this computer (issue #45): episodes of a season share their intro
// and credits music, so the same stretch of audio turns up in each. Each episode's start and end
// are fingerprinted with FFmpeg's Chromaprint, and neighbouring episodes compared.

// pointSec is how much audio one Chromaprint point covers.
const pointSec = 0.1238

// fingerprint returns the Chromaprint fingerprint of part of a file's audio.
func fingerprint(ctx context.Context, ffmpeg, path string, fromSec, lenSec float64) ([]uint32, error) {
	work, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(work, ffmpeg, "-hide_banner", "-loglevel", "error",
		"-ss", strconv.FormatFloat(fromSec, 'f', 2, 64), "-t", strconv.FormatFloat(lenSec, 'f', 2, 64), "-i", path,
		"-map", "0:a:0?", "-ac", "1", "-f", "chromaprint", "-fp_format", "raw", "-")
	var stderr fingerprintStderr
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if strings.Contains(stderr.String(), "does not contain any stream") {
			return nil, errNoAudio
		}
		if work.Err() != nil {
			err = work.Err()
		}
		return nil, fmt.Errorf("audio fingerprint failed: %w (%s)", err, fingerprintDetail(stderr.String(), path))
	}
	if len(out) < 4 {
		return nil, errNoAudio
	}
	fp := make([]uint32, len(out)/4)
	for i := range fp {
		fp[i] = binary.LittleEndian.Uint32(out[i*4:])
	}
	return fp, nil
}

// run is a stretch of matching audio: points [aStart, aEnd) of one fingerprint and the same
// stretch from bStart in the other.
type run struct{ aStart, aEnd, bStart int }

const (
	maxBitErrors = 6   // points this close count as the same sound
	maxGap       = 12  // a run carries on over up to ~1.5 seconds of differences
	minPoints    = 121 // a shared stretch must be at least 15 seconds
)

// sharedRun finds the longest stretch of audio two fingerprints share.
func sharedRun(a, b []uint32) (run, bool) {
	// Exactly equal points suggest how far one is shifted against the other.
	at := map[uint32][]int{}
	for j, v := range b {
		at[v] = append(at[v], j)
	}
	votes := map[int]int{}
	for i, v := range a {
		for _, j := range at[v] {
			votes[j-i]++
		}
	}
	type shift struct{ by, votes int }
	var shifts []shift
	for by, n := range votes {
		shifts = append(shifts, shift{by, n})
	}
	sort.Slice(shifts, func(i, j int) bool { return shifts[i].votes > shifts[j].votes })
	tried := map[int]bool{}
	var best run
	for k := 0; k < len(shifts) && k < 5; k++ {
		for d := -2; d <= 2; d++ {
			by := shifts[k].by + d
			if tried[by] {
				continue
			}
			tried[by] = true
			if r := longestAt(a, b, by); r.aEnd-r.aStart > best.aEnd-best.aStart {
				best = r
			}
		}
	}
	return best, best.aEnd > best.aStart
}

// longestAt finds the longest matching stretch with b shifted by `by` points against a.
func longestAt(a, b []uint32, by int) run {
	var best run
	start, last := -1, -1
	for i := range a {
		j := i + by
		if j < 0 || j >= len(b) {
			continue
		}
		if bits.OnesCount32(a[i]^b[j]) > maxBitErrors {
			if start >= 0 && i-last > maxGap {
				start = -1
			}
			continue
		}
		if start < 0 {
			start = i
		}
		last = i
		if last+1-start > best.aEnd-best.aStart {
			best = run{aStart: start, aEnd: last + 1, bStart: start + by}
		}
	}
	return best
}

// detectSeason finds the intro and credits each episode of a season shares with its neighbours.
// Files must be in episode order; at least two are needed.
func detectSeason(ctx context.Context, ffmpeg string, files []store.MarkerJob, onSkip ...func(store.MarkerJob, error)) (map[int64][]store.Marker, error) {
	out := map[int64][]store.Marker{}
	if len(files) < 2 {
		return out, nil
	}
	type prints struct {
		head, tail      []uint32
		tailFrom, total float64
	}
	fps := make([]prints, 0, len(files))
	valid := make([]store.MarkerJob, 0, len(files))
	skip := func(f store.MarkerJob, err error) {
		for _, report := range onSkip {
			report(f, err)
		}
	}
	for _, f := range files {
		total := float64(f.DurationMS) / 1000
		headLen := min(total*0.3, 600)
		tailLen := min(total*0.25, 420)
		head, err := fingerprint(ctx, ffmpeg, f.Path, 0, headLen)
		if err != nil {
			if ctx.Err() != nil {
				return out, ctx.Err()
			}
			skip(f, err)
			continue
		}
		tail, err := fingerprint(ctx, ffmpeg, f.Path, total-tailLen, tailLen)
		if err != nil {
			if ctx.Err() != nil {
				return out, ctx.Err()
			}
			skip(f, err)
			continue
		}
		fps = append(fps, prints{head: head, tail: tail, tailFrom: total - tailLen, total: total})
		valid = append(valid, f)
	}
	files = valid
	if len(files) < 2 {
		return out, nil
	}
	sec := func(points int) int64 { return int64(float64(points) * pointSec * 1000) }
	for i, f := range files {
		// Compare with the next episode (the last one with the one before).
		j := i + 1
		if j == len(files) {
			j = i - 1
		}
		var found []store.Marker
		if r, ok := sharedRun(fps[i].head, fps[j].head); ok && r.aEnd-r.aStart >= minPoints {
			found = append(found, store.Marker{Kind: "intro", StartMS: sec(r.aStart), EndMS: sec(r.aEnd), Source: "detected"})
		}
		if r, ok := sharedRun(fps[i].tail, fps[j].tail); ok && r.aEnd-r.aStart >= minPoints {
			from := int64(fps[i].tailFrom*1000) + sec(r.aStart)
			to := int64(fps[i].tailFrom*1000) + sec(r.aEnd)
			// Credits that run nearly to the end are skipped to the end.
			if f.DurationMS-to < 20_000 {
				to = f.DurationMS
			}
			found = append(found, store.Marker{Kind: "credits", StartMS: from, EndMS: to, Source: "detected"})
		}
		if ok := fits(found, f.DurationMS, 0); len(ok) > 0 {
			out[f.FileID] = ok
		}
	}
	return out, nil
}
