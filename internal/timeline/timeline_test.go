package timeline

import (
	"testing"
	"time"

	"github.com/leue21/timeweft/internal/events"
)

var t0 = time.Date(2026, 11, 16, 9, 0, 0, 0, time.UTC)

func at(min int) time.Time { return t0.Add(time.Duration(min) * time.Minute) }

func ev(min int, kind events.Kind, session, branch string) events.Event {
	return events.Event{Time: at(min), Kind: kind, Session: session, Repo: "github.com/acme/api", Branch: branch}
}

func totals(bs []Block) map[string]time.Duration {
	m := map[string]time.Duration{}
	for _, b := range bs {
		m[b.Branch] += b.Duration()
	}
	return m
}

func TestAttentionFollowsLastTouchedSession(t *testing.T) {
	evs := []events.Event{
		ev(0, events.Prompt, "A", "feat/a"),
		ev(2, events.Stop, "A", "feat/a"), // agent events don't move attention
		ev(10, events.Prompt, "B", "feat/b"),
		ev(12, events.Prompt, "A", "feat/a"),
		ev(20, events.Prompt, "B", "feat/b"),
	}
	p := DefaultParams(at(22)) // still active: last block runs until now
	got := totals(Attention(evs, p))
	want := map[string]time.Duration{"feat/a": 18 * time.Minute, "feat/b": 4 * time.Minute}
	if got["feat/a"] != want["feat/a"] || got["feat/b"] != want["feat/b"] {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestAttentionIdleGapOnlyGetsTail(t *testing.T) {
	evs := []events.Event{
		ev(0, events.Prompt, "A", "feat/a"),
		ev(60, events.Prompt, "A", "feat/a"), // came back after lunch
	}
	p := DefaultParams(at(200)) // long gone after the second prompt too
	got := totals(Attention(evs, p))["feat/a"]
	if want := 10 * time.Minute; got != want { // tail + tail
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestAttentionNeverOverlaps(t *testing.T) {
	evs := []events.Event{
		ev(0, events.Prompt, "A", "a"), ev(1, events.Prompt, "B", "b"),
		ev(2, events.Prompt, "C", "c"), ev(40, events.Prompt, "A", "a"),
	}
	bs := Attention(evs, DefaultParams(at(41)))
	for i := 1; i < len(bs); i++ {
		if bs[i].Start.Before(bs[i-1].End) {
			t.Fatalf("blocks overlap: %v and %v", bs[i-1], bs[i])
		}
	}
}

func TestAgentRuns(t *testing.T) {
	evs := []events.Event{
		ev(0, events.Prompt, "A", "feat/a"),
		ev(1, events.Prompt, "B", "feat/b"),
		ev(30, events.Stop, "A", "feat/a"),
		ev(40, events.Prompt, "A", "feat/a"), // interrupted: no Stop ...
		ev(45, events.Prompt, "A", "feat/a"), // ... the new prompt closes the run
		ev(50, events.Stop, "A", "feat/a"),
		ev(400, events.Prompt, "C", "feat/c"), // never stopped, now is far away
	}
	p := DefaultParams(at(1000))
	got := totals(AgentRuns(evs, p))
	// B never stopped either: capped at MaxAgentRun.
	want := map[string]time.Duration{"feat/a": 40 * time.Minute, "feat/b": 2 * time.Hour, "feat/c": 2 * time.Hour}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: got %v, want %v", k, got[k], v)
		}
	}
}

func TestClip(t *testing.T) {
	bs := []Block{{Start: at(-30), End: at(30)}, {Start: at(100), End: at(200)}}
	got := Clip(bs, at(0), at(60))
	if len(got) != 1 || got[0].Start != at(0) || got[0].End != at(30) {
		t.Fatalf("got %v", got)
	}
}
