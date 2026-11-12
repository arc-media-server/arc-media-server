package optimize

import (
	"bytes"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPriorityFailureContinuesConversionAndWarnsOnce(t *testing.T) {
	x := conversionTest(t)
	helperConverter(t, x)
	var logs bytes.Buffer
	x.m.log = slog.New(slog.NewTextHandler(&logs, nil))
	calls := 0
	x.m.lowerPriority = func(*os.Process) error {
		calls++
		return errors.New("test policy denied")
	}
	// The helper finishes normally: a denied scheduling hint must not fail or strand work.
	if err := os.WriteFile(filepath.Join(x.root, "release"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	for _, quality := range []int{720, 1080} {
		o := x.queue(t, quality)
		if err := x.m.convert(t.Context(), o, nil); err != nil {
			t.Fatal(err)
		}
		if got := x.copy(t, quality); got.Status != "done" {
			t.Fatalf("priority denial changed completion: %+v", got)
		}
	}
	if calls != 2 {
		t.Fatalf("priority was not requested for both attempts: %d", calls)
	}
	if got := strings.Count(logs.String(), "optimized conversion CPU priority unavailable"); got != 1 {
		t.Fatalf("expected one diagnostic, got %d: %s", got, logs.String())
	}
}
