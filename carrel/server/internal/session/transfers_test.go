package session

import (
	"fmt"
	"testing"

	"carrel.dev/carrel/server/internal/conf"
)

type fakeTransfer struct {
	id        string
	waiting   bool
	discarded *[]string
}

func (f fakeTransfer) ID() string    { return f.id }
func (f fakeTransfer) Waiting() bool { return f.waiting }
func (f fakeTransfer) Discard()      { *f.discarded = append(*f.discarded, f.id) }

func ids(rows []Transfer) string {
	var out []string
	for _, r := range rows {
		out = append(out, r.ID())
	}
	return fmt.Sprint(out)
}

// S-17.3: a question stays until it is answered, a finished row is handed
// over once, an answer takes its row, the oldest finished row goes first past
// the length, and the end of the session lets go of every file a row holds.
func TestTransferQueue(t *testing.T) {
	mgr := NewManager(testCfg())
	s, err := mgr.Create("a-1", "alice", testDataKey(t))
	if err != nil {
		t.Fatal(err)
	}
	var discarded []string
	row := func(id string, waiting bool) fakeTransfer {
		return fakeTransfer{id: id, waiting: waiting, discarded: &discarded}
	}

	s.KeepTransfer(row("done", false))
	s.KeepTransfer(row("ask", true))
	if got := ids(s.ReadTransfers()); got != "[done ask]" {
		t.Fatalf("first read = %s", got)
	}
	if got := ids(s.ReadTransfers()); got != "[ask]" {
		t.Fatalf("second read = %s", got)
	}
	if _, ok := s.TakeTransfer("ask"); !ok {
		t.Fatal("the question was not taken")
	}
	if _, ok := s.TakeTransfer("ask"); ok {
		t.Fatal("one question answered twice")
	}

	s.KeepTransfer(row("q0", true))
	s.KeepTransfer(row("finished", false))
	for i := 1; i < conf.TransferQueueLen; i++ {
		s.KeepTransfer(row(fmt.Sprintf("q%d", i), true))
	}
	if fmt.Sprint(discarded) != "[finished]" {
		t.Fatalf("past the length, discarded %v", discarded)
	}
	s.KeepTransfer(row("one more", true))
	if fmt.Sprint(discarded) != "[finished q0]" {
		t.Fatalf("with questions only, discarded %v", discarded)
	}

	mgr.End(s.ID())
	if len(discarded) != 2+conf.TransferQueueLen {
		t.Fatalf("the end of the session discarded %d rows", len(discarded))
	}
	s.KeepTransfer(row("late", true))
	if discarded[len(discarded)-1] != "late" {
		t.Error("an ended session kept a row")
	}
}
