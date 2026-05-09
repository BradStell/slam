package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/bradstell/slam/engine"
)

func TestTTYReporter_NonTTYIsSilent(t *testing.T) {
	var buf bytes.Buffer
	r := newTTYReporter(&buf)
	if r.isTTY {
		t.Error("bytes.Buffer should not be detected as TTY")
	}
	r.OnStart(engine.Plan{Concurrency: 50})
	r.OnTick(engine.Snapshot{Sent: 100, Errors: 0, CurrentRPS: 200})
	r.OnFinish(&engine.Summary{TotalSent: 100})
	if buf.Len() > 0 {
		t.Errorf("non-TTY reporter wrote %q, want empty", buf.String())
	}
}

func TestTTYReporter_OnTickFormatWhenTTY(t *testing.T) {
	var buf bytes.Buffer
	r := &ttyReporter{w: &buf, isTTY: true}
	r.OnTick(engine.Snapshot{
		Sent:       250,
		Errors:     3,
		CurrentRPS: 125.5,
		Elapsed:    2_000_000_000, // 2s in nanoseconds
	})
	out := buf.String()
	for _, want := range []string{"250 sent", "3 errors", "126 RPS"} {
		if !strings.Contains(out, want) {
			t.Errorf("OnTick output %q missing %q", out, want)
		}
	}
}
