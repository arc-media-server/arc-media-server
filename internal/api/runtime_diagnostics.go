package api

import (
	"bytes"
	"runtime/pprof"
)

const stackSnapshotLimit = 512 * 1024

// pprof's debug=1 profile contains grouped call stacks, without argument values
// or request bodies. Capture only on an explicit admin download, never per request.
func goroutineSnapshot() []byte {
	w := &boundedProfile{limit: stackSnapshotLimit}
	_ = pprof.Lookup("goroutine").WriteTo(w, 1)
	if w.truncated {
		w.buf.WriteString("\n[stack snapshot truncated]\n")
	}
	return w.buf.Bytes()
}

type boundedProfile struct {
	buf       bytes.Buffer
	limit     int
	truncated bool
}

func (w *boundedProfile) Write(p []byte) (int, error) {
	n := len(p)
	remaining := max(0, w.limit-w.buf.Len())
	if n > remaining {
		w.truncated = true
		p = p[:remaining]
	}
	_, _ = w.buf.Write(p)
	return n, nil
}
