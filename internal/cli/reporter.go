package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/bradstell/slam/engine"
)

// ttyReporter renders a single live-updating status line to a TTY writer.
// When the destination is not a TTY (e.g. stdout is piped or redirected,
// or output was set to a bytes.Buffer in tests), it stays silent so the
// captured stream is clean.
type ttyReporter struct {
	w     io.Writer
	isTTY bool
}

func newTTYReporter(w io.Writer) *ttyReporter {
	return &ttyReporter{w: w, isTTY: isTerminal(w)}
}

// isTerminal returns true if w is an *os.File pointing at a character device.
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return (fi.Mode() & os.ModeCharDevice) != 0
}

func (r *ttyReporter) OnStart(engine.Plan) {}

func (r *ttyReporter) OnTick(s engine.Snapshot) {
	if !r.isTTY {
		return
	}
	// \r returns to start of line; trailing spaces clear leftover chars from
	// a previous longer line.
	fmt.Fprintf(r.w, "\r  %s elapsed · %d sent · %d errors · %.0f RPS · p99 %s    ",
		roundDur(s.Elapsed), s.Sent, s.Errors, s.CurrentRPS, roundDur(s.RollingP99))
}

func (r *ttyReporter) OnFinish(*engine.Summary) {
	if r.isTTY {
		fmt.Fprintln(r.w)
	}
}
