// Package claude integrates with Claude Code: it turns hook payloads into
// events and installs the hooks into Claude Code's user settings.
package claude

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/leue21/timeweft/internal/events"
)

// hookInput is the subset of Claude Code's hook payload timeweft uses.
// The prompt text is deliberately not read or stored.
type hookInput struct {
	SessionID     string `json:"session_id"`
	Cwd           string `json:"cwd"`
	HookEventName string `json:"hook_event_name"`
	Source        string `json:"source"` // SessionStart: startup | resume | clear | compact
}

// HookEvents are the Claude Code hook events timeweft listens to.
var HookEvents = []string{"SessionStart", "UserPromptSubmit", "Stop", "SessionEnd"}

// ParseHook converts a hook payload into an event without repo/branch, which
// the caller fills in. ok is false for payloads that should not be recorded.
func ParseHook(payload []byte, now time.Time) (e events.Event, ok bool, err error) {
	var in hookInput
	if err := json.Unmarshal(payload, &in); err != nil {
		return e, false, fmt.Errorf("parse hook payload: %w", err)
	}
	var kind events.Kind
	switch in.HookEventName {
	case "SessionStart":
		if in.Source == "compact" { // automatic, not a user action
			return e, false, nil
		}
		kind = events.SessionStart
	case "UserPromptSubmit":
		kind = events.Prompt
	case "Stop":
		kind = events.Stop
	case "SessionEnd":
		kind = events.SessionEnd
	default:
		return e, false, nil
	}
	if in.SessionID == "" || in.Cwd == "" {
		return e, false, fmt.Errorf("hook payload for %s lacks session_id or cwd", in.HookEventName)
	}
	return events.Event{Time: now, Source: "claude", Kind: kind, Session: in.SessionID, Cwd: in.Cwd}, true, nil
}
