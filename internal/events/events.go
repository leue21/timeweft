// Package events defines the raw activity events timeweft records.
package events

import "time"

// Kind is what happened in an agent session.
type Kind string

const (
	SessionStart Kind = "session_start" // user started or resumed a session
	Prompt       Kind = "prompt"        // user submitted a prompt
	Stop         Kind = "stop"          // agent finished its turn
	SessionEnd   Kind = "session_end"   // session closed
)

// Event is one recorded moment of activity, tagged with the repo and branch
// the session was working on at that time.
type Event struct {
	ID      int64
	Time    time.Time
	Source  string // e.g. "claude"
	Kind    Kind
	Session string
	Repo    string // normalized repo identity, see gitinfo
	Branch  string
	Cwd     string
}

// IsTouch reports whether the event proves the user was actively working on
// this session at that moment (as opposed to the agent working on its own).
func (e Event) IsTouch() bool {
	return e.Kind == Prompt || e.Kind == SessionStart
}
