package gateway

import (
	"bufio"
	"context"
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"
)

// spy prints what the front handed to the connector. A terminal running
// "resize" also prints the first Resize, a terminal of the resource "typeahead"
// the first input line; an exec exits with 7.
func spy(ctx context.Context, s *Session) error {
	switch {
	case s.Terminal != nil:
		t := s.Terminal
		_, _ = fmt.Fprintf(t, "spy:%s kind=terminal term=%q cmd=%q win=%dx%d\n", s.Resource.Name, t.Term, t.Command, t.Window.Cols, t.Window.Rows)
		if t.Command == "resize" {
			select {
			case w := <-t.Resize:
				_, _ = fmt.Fprintf(t, "resize=%dx%d\n", w.Cols, w.Rows)
			case <-time.After(2 * time.Second):
				_, _ = fmt.Fprintln(t, "resize=none")
			case <-ctx.Done():
			}
		}
		if s.Resource.Name == "typeahead" {
			line, _ := bufio.NewReader(t).ReadString('\r')
			_, _ = fmt.Fprintf(t, "typed=%q\n", line)
		}
	case s.Exec != nil:
		_, _ = fmt.Fprintf(s.Exec.Stdout, "spy:%s kind=exec cmd=%q\n", s.Resource.Name, s.Exec.Command)
		s.Exec.SetExitStatus(7)
	case s.Stream != nil:
		_, _ = fmt.Fprintf(s.Stream, "spy:%s kind=stream target=%s\n", s.Resource.Name, s.Target)
	}
	return nil
}

// spySink notes its Close in the control's event log: "close" when the session
// end was spooled (pending or already delivered) before, "close-before-end" otherwise.
type spySink struct {
	f  *fakeControl
	p  *Pipeline
	id uuid.UUID
}

func (spySink) Output([]byte)   {}
func (spySink) Input([]byte)    {}
func (spySink) Resize(int, int) {}
func (s spySink) Close(context.Context) error {
	ev := "close-before-end"
	if slices.Contains(s.p.spool.PendingEnds(), s.id) || len(s.f.endsOf(s.id)) > 0 { // delivery records before it leaves PendingEnds
		ev = "close"
	}
	s.f.with(func(f *fakeControl) { f.events = append(f.events, ev) })
	return nil
}
