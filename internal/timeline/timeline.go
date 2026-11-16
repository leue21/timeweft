// Package timeline turns raw events into time blocks.
//
// Two kinds of time are computed:
//
//   - Attention: the user's own time. It belongs to the session the user last
//     touched (prompted or started), until they touch another session. A gap
//     longer than Idle means they stepped away: the previous session only
//     gets Tail (time to read the answer), the rest is not counted.
//     Attention never overlaps itself, so it can go on a timesheet.
//
//   - Agent: how long an agent was busy, from a prompt until the agent stops.
//     Parallel agents overlap; this is informational, not billable.
package timeline

import (
	"sort"
	"time"

	"github.com/leue21/timeweft/internal/events"
)

type Key struct{ Repo, Branch string }

type Block struct {
	Key
	Session    string
	Start, End time.Time
}

func (b Block) Duration() time.Duration { return b.End.Sub(b.Start) }

type Params struct {
	Idle        time.Duration // longest gap that still counts as continuous attention
	Tail        time.Duration // credit after a touch when the user then went idle
	MaxAgentRun time.Duration // cap for one agent run (interrupted runs have no Stop)
	Now         time.Time
}

func DefaultParams(now time.Time) Params {
	return Params{Idle: 15 * time.Minute, Tail: 5 * time.Minute, MaxAgentRun: 2 * time.Hour, Now: now}
}

// Attention computes the user's attention blocks. evs must be sorted by time.
func Attention(evs []events.Event, p Params) []Block {
	var touches []events.Event
	for _, e := range evs {
		if e.IsTouch() {
			touches = append(touches, e)
		}
	}
	var out []Block
	for i, t := range touches {
		next, hasNext := p.Now, i+1 < len(touches)
		if hasNext {
			next = touches[i+1].Time
		}
		end := next
		if gap := next.Sub(t.Time); gap > p.Idle || gap < 0 {
			end = t.Time.Add(p.Tail)
			if hasNext && end.After(next) {
				end = next
			}
		}
		if end.After(t.Time) {
			out = append(out, Block{Key: Key{t.Repo, t.Branch}, Session: t.Session, Start: t.Time, End: end})
		}
	}
	return out
}

// AgentRuns computes when each agent was busy. evs must be sorted by time.
// A run starts at a prompt and ends at the next event of the same session
// (Stop, a new prompt after an interrupt, session end or restart), capped at
// MaxAgentRun. A run still open is counted until Now, also capped.
func AgentRuns(evs []events.Event, p Params) []Block {
	open := map[string]events.Event{}
	var out []Block
	closeRun := func(start events.Event, at time.Time) {
		if limit := start.Time.Add(p.MaxAgentRun); at.After(limit) {
			at = limit
		}
		if at.After(start.Time) {
			out = append(out, Block{Key: Key{start.Repo, start.Branch}, Session: start.Session, Start: start.Time, End: at})
		}
	}
	for _, e := range evs {
		if start, ok := open[e.Session]; ok {
			closeRun(start, e.Time)
			delete(open, e.Session)
		}
		if e.Kind == events.Prompt {
			open[e.Session] = e
		}
	}
	for _, start := range open {
		closeRun(start, p.Now)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Start.Before(out[j].Start) })
	return out
}

// Clip cuts blocks to [from, to) and drops those outside it.
func Clip(bs []Block, from, to time.Time) []Block {
	var out []Block
	for _, b := range bs {
		if b.Start.Before(from) {
			b.Start = from
		}
		if b.End.After(to) {
			b.End = to
		}
		if b.End.After(b.Start) {
			out = append(out, b)
		}
	}
	return out
}
