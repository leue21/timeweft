package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// End to end: hook payloads in, report out.
func TestHookToReport(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("TIMEWEFT_DB", filepath.Join(tmp, "tw.db"))

	repo := filepath.Join(tmp, "api")
	for _, args := range [][]string{
		{"init", "-q", "-b", "feat/PROJ-7-login", repo},
		{"-C", repo, "remote", "add", "origin", "git@github.com:acme/api.git"},
	} {
		if out, err := exec.Command("git", args...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}

	start := time.Now().Add(-30 * time.Minute)
	if start.Day() != time.Now().Day() {
		t.Skip("test window crosses midnight")
	}
	send := func(min int, event string) {
		payload, _ := json.Marshal(map[string]string{"session_id": "sess-1", "cwd": repo, "hook_event_name": event, "source": "startup"})
		if err := recordClaudeHook(bytes.NewReader(payload), start.Add(time.Duration(min)*time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	send(0, "SessionStart")
	send(1, "UserPromptSubmit")
	send(9, "Stop")
	send(10, "UserPromptSubmit")
	send(14, "Stop")

	day := start.Format("2006-01-02")
	var out bytes.Buffer
	if err := runReport([]string{day, "--json"}, &out); err != nil {
		t.Fatal(err)
	}
	var rep struct {
		Rows []struct {
			Repo, Branch     string
			Prompts          int
			AttentionSeconds int64 `json:"attention_seconds"`
			AgentSeconds     int64 `json:"agent_seconds"`
		}
	}
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("%v\n%s", err, out.String())
	}
	if len(rep.Rows) != 1 {
		t.Fatalf("want one row, got %s", out.String())
	}
	r := rep.Rows[0]
	if r.Repo != "github.com/acme/api" || r.Branch != "feat/PROJ-7-login" || r.Prompts != 2 {
		t.Errorf("row: %+v", r)
	}
	// Touches at 0, 1, 10; last one is 20 min old (> idle) so it gets the 5 min tail.
	if r.AttentionSeconds != 15*60 {
		t.Errorf("attention = %ds, want %ds", r.AttentionSeconds, 15*60)
	}
	if r.AgentSeconds != 12*60 { // 1→9 and 10→14
		t.Errorf("agent = %ds, want %ds", r.AgentSeconds, 12*60)
	}

	out.Reset()
	if err := runReport([]string{day}, &out); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "acme/api") || !strings.Contains(out.String(), "0:15") {
		t.Errorf("text report:\n%s", out.String())
	}
}

func TestHookNeverFails(t *testing.T) {
	t.Setenv("TIMEWEFT_DB", filepath.Join(t.TempDir(), "tw.db"))
	runHook([]string{"claude"}, strings.NewReader("garbage"))
	logPath := filepath.Join(filepath.Dir(os.Getenv("TIMEWEFT_DB")), "hook-errors.log")
	if b, err := os.ReadFile(logPath); err != nil || !strings.Contains(string(b), "parse hook payload") {
		t.Errorf("error not logged: %v %s", err, b)
	}
}

func TestParseRange(t *testing.T) {
	now := time.Date(2026, 11, 18, 15, 0, 0, 0, time.Local) // a Wednesday
	from, to, _, err := parseRange([]string{"week"}, now)
	if err != nil || from.Format("2006-01-02") != "2026-11-16" || to.Format("2006-01-02") != "2026-11-23" {
		t.Errorf("week: %v %v %v", from, to, err)
	}
	from, _, _, _ = parseRange([]string{"yesterday"}, now)
	if from.Format("2006-01-02") != "2026-11-17" {
		t.Errorf("yesterday: %v", from)
	}
	if _, _, _, err := parseRange([]string{"someday"}, now); err == nil {
		t.Error("expected error")
	}
}
