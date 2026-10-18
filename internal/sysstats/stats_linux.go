package sysstats

import (
	"bufio"
	"os"
	"strconv"
	"strings"
	"time"
)

// clockTicks is Linux's USER_HZ, the unit of /proc's CPU times (100 on every common system).
const clockTicks = 100

func ticks(n int64) time.Duration { return time.Duration(n) * time.Second / clockTicks }

func readCPU() (cpuTimes, error) {
	var c cpuTimes
	self, err := os.ReadFile("/proc/self/stat")
	if err != nil {
		return c, err
	}
	// Fields after the command name (in brackets, which may hold spaces): utime and stime are 14th and 15th.
	rest := string(self)
	if i := strings.LastIndexByte(rest, ')'); i >= 0 {
		rest = rest[i+2:]
	}
	f := strings.Fields(rest)
	if len(f) > 12 {
		u, _ := strconv.ParseInt(f[11], 10, 64)
		s, _ := strconv.ParseInt(f[12], 10, 64)
		c.arc = ticks(u + s)
	}
	stat, err := os.Open("/proc/stat")
	if err != nil {
		return c, nil
	}
	defer stat.Close()
	sc := bufio.NewScanner(stat)
	if sc.Scan() {
		f := strings.Fields(sc.Text()) // cpu user nice system idle iowait irq softirq steal
		var total, idle int64
		for i, v := range f[1:] {
			n, _ := strconv.ParseInt(v, 10, 64)
			if i < 8 {
				total += n
			}
			if i == 3 || i == 4 {
				idle += n
			}
		}
		c.total, c.busy, c.haveSystem = ticks(total), ticks(total-idle), true
	}
	return c, nil
}

func readMemory() (memory, error) {
	m := memory{arc: -1}
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return m, err
	}
	defer f.Close()
	var total, avail int64
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		k, v, _ := strings.Cut(sc.Text(), ":")
		n, _ := strconv.ParseInt(strings.TrimSuffix(strings.TrimSpace(v), " kB"), 10, 64)
		switch k {
		case "MemTotal":
			total = n * 1024
		case "MemAvailable":
			avail = n * 1024
		}
	}
	m.total, m.used = total, total-avail
	if st, err := os.ReadFile("/proc/self/status"); err == nil {
		for _, line := range strings.Split(string(st), "\n") {
			if v, ok := strings.CutPrefix(line, "VmRSS:"); ok {
				n, _ := strconv.ParseInt(strings.TrimSuffix(strings.TrimSpace(v), " kB"), 10, 64)
				m.arc = n * 1024
			}
		}
	}
	return m, nil
}
