package sysstats

import (
	"testing"
	"time"
)

func TestReadings(t *testing.T) {
	a, err := readCPU()
	if err != nil {
		t.Fatal(err)
	}
	end := time.Now().Add(300 * time.Millisecond)
	for time.Now().Before(end) { // some work to measure
	}
	b, _ := readCPU()
	arc, system := b.percent(a, 0.3)
	if arc <= 0 || arc > 100 || (b.haveSystem && (system < 0 || system > 100)) {
		t.Errorf("cpu arc %.1f%% system %.1f%%", arc, system)
	}
	m, err := readMemory()
	if err != nil || m.arc <= 0 {
		t.Fatalf("memory %+v %v", m, err)
	}
	if m.total > 0 && (m.used <= 0 || m.used > m.total) {
		t.Errorf("memory %+v", m)
	}
}

func TestMinutes(t *testing.T) {
	c := New()
	for i := range 70 {
		c.add(Sample{Time: int64(600 + i*2), CPUArc: float64(i % 2 * 10), NetHome: 100})
	}
	day := c.Recent(true).Samples
	if len(day) != 3 || day[0].Time != 600 || day[0].CPUArc != 5 || day[0].NetHome != 100 {
		t.Fatalf("got %+v", day)
	}
	if n := len(c.Recent(false).Samples); n != 70 {
		t.Fatalf("recent %d", n)
	}
}
