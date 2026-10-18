// Package sysstats samples how busy the server is, for the Dashboard's graphs (as Plex's): CPU and
// memory, Arc's own and the whole computer's, and the data Arc sends to devices at home and away.
package sysstats

import (
	"context"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

// Sample is one moment's figures. CPU is a percentage of the whole computer (all cores); network
// is bytes a second. A figure the system can't give is -1.
type Sample struct {
	Time      int64   `json:"t"` // unix seconds
	CPUArc    float64 `json:"cpu_arc"`
	CPUSystem float64 `json:"cpu_system"`
	MemArc    int64   `json:"mem_arc"`
	MemUsed   int64   `json:"mem_used"`
	NetHome   int64   `json:"net_home"`
	NetAway   int64   `json:"net_away"`
}

// How often samples are taken, and how many are kept: five minutes of them, then a day of
// minute averages.
const (
	every      = 2 * time.Second
	recentKeep = 150
	dayKeep    = 24 * 60
)

var sentHome, sentAway atomic.Int64

// AddSent counts data sent to a device at home or away.
func AddSent(home bool, n int) {
	if home {
		sentHome.Add(int64(n))
	} else {
		sentAway.Add(int64(n))
	}
}

// Collector takes samples in the background.
type Collector struct {
	mu     sync.Mutex
	recent []Sample
	day    []Sample
	minute []Sample // this minute's samples, averaged into day when it ends
}

func New() *Collector { return &Collector{} }

// Run samples until ctx ends.
func (c *Collector) Run(ctx context.Context) {
	prevCPU, _ := readCPU()
	prevHome, prevAway := sentHome.Load(), sentAway.Load()
	prevAt := time.Now()
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			cpu, err := readCPU()
			secs := now.Sub(prevAt).Seconds()
			s := Sample{Time: now.Unix(), CPUArc: -1, CPUSystem: -1, MemArc: -1, MemUsed: -1}
			if err == nil {
				s.CPUArc, s.CPUSystem = cpu.percent(prevCPU, secs)
				prevCPU = cpu
			}
			if m, err := readMemory(); err == nil {
				s.MemArc, s.MemUsed = m.arc, m.used
			}
			home, away := sentHome.Load(), sentAway.Load()
			s.NetHome, s.NetAway = int64(float64(home-prevHome)/secs), int64(float64(away-prevAway)/secs)
			prevHome, prevAway, prevAt = home, away, now
			c.add(s)
		}
	}
}

func (c *Collector) add(s Sample) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.recent = append(c.recent, s)
	if len(c.recent) > recentKeep {
		c.recent = c.recent[len(c.recent)-recentKeep:]
	}
	if len(c.minute) > 0 && s.Time/60 != c.minute[0].Time/60 {
		c.day = append(c.day, average(c.minute))
		if len(c.day) > dayKeep {
			c.day = c.day[len(c.day)-dayKeep:]
		}
		c.minute = c.minute[:0]
	}
	c.minute = append(c.minute, s)
}

// average is a minute's samples as one, at the start of the minute.
func average(ss []Sample) Sample {
	out := Sample{Time: ss[0].Time / 60 * 60}
	var n float64
	for _, s := range ss {
		out.CPUArc += s.CPUArc
		out.CPUSystem += s.CPUSystem
		out.MemArc += s.MemArc
		out.MemUsed += s.MemUsed
		out.NetHome += s.NetHome
		out.NetAway += s.NetAway
		n++
	}
	out.CPUArc /= n
	out.CPUSystem /= n
	out.MemArc = int64(float64(out.MemArc) / n)
	out.MemUsed = int64(float64(out.MemUsed) / n)
	out.NetHome = int64(float64(out.NetHome) / n)
	out.NetAway = int64(float64(out.NetAway) / n)
	return out
}

// Info is what the Dashboard shows.
type Info struct {
	Cores    int      `json:"cores"`
	MemTotal int64    `json:"mem_total"` // -1 if unknown
	Samples  []Sample `json:"samples"`
}

// Recent is the last five minutes, or with day the last 24 hours in minutes.
func (c *Collector) Recent(day bool) Info {
	c.mu.Lock()
	defer c.mu.Unlock()
	samples := append([]Sample{}, c.recent...)
	if day {
		samples = append([]Sample{}, c.day...)
		if len(c.minute) > 0 {
			samples = append(samples, average(c.minute)) // the minute so far
		}
	}
	out := Info{Cores: runtime.NumCPU(), MemTotal: -1, Samples: samples}
	if m, err := readMemory(); err == nil {
		out.MemTotal = m.total
	}
	return out
}

// cpuTimes are cumulative CPU times: Arc's, and the whole computer's busy and total.
type cpuTimes struct {
	arc, busy, total time.Duration
	haveSystem       bool
}

// percent works out the use between two readings, secs apart.
func (c cpuTimes) percent(prev cpuTimes, secs float64) (arc, system float64) {
	wall := secs * float64(runtime.NumCPU()) * float64(time.Second)
	arc = clamp(float64(c.arc-prev.arc) / wall * 100)
	system = -1
	if c.haveSystem && c.total > prev.total {
		system = clamp(float64(c.busy-prev.busy) / float64(c.total-prev.total) * 100)
	}
	return arc, system
}

func clamp(p float64) float64 { return max(0, min(100, p)) }

type memory struct{ arc, used, total int64 }
