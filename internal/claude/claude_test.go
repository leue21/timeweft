package claude

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/leue21/timeweft/internal/events"
)

func TestParseHook(t *testing.T) {
	now := time.Now()
	e, ok, err := ParseHook([]byte(`{"session_id":"s1","cwd":"/x","hook_event_name":"UserPromptSubmit","prompt":"secret"}`), now)
	if err != nil || !ok || e.Kind != events.Prompt || e.Session != "s1" || e.Cwd != "/x" {
		t.Fatalf("prompt: %+v %v %v", e, ok, err)
	}
	if _, ok, _ := ParseHook([]byte(`{"session_id":"s1","cwd":"/x","hook_event_name":"SessionStart","source":"compact"}`), now); ok {
		t.Error("compaction must not count as a user action")
	}
	if _, ok, _ := ParseHook([]byte(`{"session_id":"s1","cwd":"/x","hook_event_name":"PreToolUse"}`), now); ok {
		t.Error("unrelated hook events must be ignored")
	}
	if _, _, err := ParseHook([]byte(`not json`), now); err == nil {
		t.Error("expected error for invalid JSON")
	}
}

const existing = `{
  "model": "opus",
  "hooks": {
    "Stop": [
      {"hooks": [{"type": "command", "command": "notify-send done"}]}
    ],
    "PreToolUse": [
      {"matcher": "Bash", "hooks": [{"type": "command", "command": "guard"}]}
    ]
  }
}`

func TestInstallKeepsOtherSettingsAndIsIdempotent(t *testing.T) {
	once, err := Install([]byte(existing), "/bin/timeweft hook claude")
	if err != nil {
		t.Fatal(err)
	}
	twice, err := Install(once, "/new/path/timeweft hook claude")
	if err != nil {
		t.Fatal(err)
	}
	var s struct {
		Model string                      `json:"model"`
		Hooks map[string][]map[string]any `json:"hooks"`
	}
	if err := json.Unmarshal(twice, &s); err != nil {
		t.Fatal(err)
	}
	if s.Model != "opus" || len(s.Hooks["PreToolUse"]) != 1 {
		t.Errorf("unrelated settings changed: %s", twice)
	}
	if len(s.Hooks["Stop"]) != 2 {
		t.Errorf("Stop should keep the user's hook plus one timeweft hook: %s", twice)
	}
	if n := strings.Count(string(twice), "timeweft hook claude"); n != len(HookEvents) {
		t.Fatalf("want exactly one timeweft hook per event, found %d total", n)
	}
	if !strings.Contains(string(twice), "/new/path/timeweft") || strings.Contains(string(twice), "/bin/timeweft") {
		t.Fatalf("old command not replaced: %s", twice)
	}

	removed, err := Uninstall(twice)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(removed), "timeweft") || !strings.Contains(string(removed), "notify-send") {
		t.Errorf("uninstall: %s", removed)
	}
}

func TestInstallIntoEmptyOrInvalid(t *testing.T) {
	if _, err := Install(nil, "x hook claude"); err != nil {
		t.Errorf("empty settings: %v", err)
	}
	if _, err := Install([]byte(`[1,2]`), "x hook claude"); err == nil {
		t.Error("expected error for non-object settings")
	}
}

func TestApplyToFileFollowsSymlinkAndBacksUp(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "dotfiles-settings.json")
	if err := os.WriteFile(target, []byte(existing), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "settings.json")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	backup, err := ApplyToFile(link, func(b []byte) ([]byte, error) { return Install(b, "tw hook claude") })
	if err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Lstat(link); fi.Mode()&os.ModeSymlink == 0 {
		t.Error("symlink was replaced by a regular file")
	}
	if b, _ := os.ReadFile(backup); string(b) != existing {
		t.Error("backup does not contain the previous settings")
	}
	if b, _ := os.ReadFile(target); !strings.Contains(string(b), "tw hook claude") {
		t.Error("target not updated")
	}
}

func TestHookCommandQuotes(t *testing.T) {
	if got := HookCommand("/opt/my tools/timeweft"); got != `'/opt/my tools/timeweft' hook claude` {
		t.Errorf("got %s", got)
	}
}
