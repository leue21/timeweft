package claude

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// hookMarker identifies hook entries owned by timeweft.
const hookMarker = "timeweft hook claude"

// SettingsPath is Claude Code's user settings file.
func SettingsPath() (string, error) {
	dir := os.Getenv("CLAUDE_CONFIG_DIR")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		dir = filepath.Join(home, ".claude")
	}
	return filepath.Join(dir, "settings.json"), nil
}

// HookCommand is the shell command Claude Code runs for each hook.
func HookCommand(binary string) string {
	if strings.ContainsAny(binary, " '\"\\$") {
		binary = "'" + strings.ReplaceAll(binary, "'", `'\''`) + "'"
	}
	return binary + " hook claude"
}

// Install adds timeweft's hooks to settings, replacing earlier timeweft
// entries and leaving everything else untouched. It is idempotent.
func Install(settings []byte, command string) ([]byte, error) {
	return edit(settings, func(groups []any) []any {
		return append(groups, map[string]any{
			"hooks": []any{map[string]any{"type": "command", "command": command, "timeout": 10}},
		})
	})
}

// Uninstall removes timeweft's hooks from settings.
func Uninstall(settings []byte) ([]byte, error) {
	return edit(settings, func(groups []any) []any { return groups })
}

// edit strips timeweft hook entries for every event in HookEvents, then lets
// add append new ones. Unknown settings and other hooks are preserved.
func edit(settings []byte, add func([]any) []any) ([]byte, error) {
	root := map[string]json.RawMessage{}
	if len(strings.TrimSpace(string(settings))) > 0 {
		if err := json.Unmarshal(settings, &root); err != nil {
			return nil, fmt.Errorf("settings.json is not a JSON object: %w", err)
		}
	}
	hooks := map[string]json.RawMessage{}
	if raw, ok := root["hooks"]; ok {
		if err := json.Unmarshal(raw, &hooks); err != nil {
			return nil, fmt.Errorf("settings.json \"hooks\" is not an object: %w", err)
		}
	}
	for _, name := range HookEvents {
		var groups []any
		if raw, ok := hooks[name]; ok {
			if err := json.Unmarshal(raw, &groups); err != nil {
				return nil, fmt.Errorf("settings.json hooks.%s is not an array: %w", name, err)
			}
		}
		groups = add(withoutOurs(groups))
		if len(groups) == 0 {
			delete(hooks, name)
			continue
		}
		raw, err := json.Marshal(groups)
		if err != nil {
			return nil, err
		}
		hooks[name] = raw
	}
	if len(hooks) == 0 {
		delete(root, "hooks")
	} else {
		raw, err := json.Marshal(hooks)
		if err != nil {
			return nil, err
		}
		root["hooks"] = raw
	}
	out, err := json.MarshalIndent(root, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(out, '\n'), nil
}

// withoutOurs drops timeweft commands from matcher groups, and groups that
// end up empty because of it.
func withoutOurs(groups []any) []any {
	var out []any
	for _, g := range groups {
		group, ok := g.(map[string]any)
		if !ok {
			out = append(out, g)
			continue
		}
		list, ok := group["hooks"].([]any)
		if !ok {
			out = append(out, g)
			continue
		}
		var kept []any
		for _, h := range list {
			if hm, ok := h.(map[string]any); ok {
				if cmd, _ := hm["command"].(string); strings.Contains(cmd, hookMarker) {
					continue
				}
			}
			kept = append(kept, h)
		}
		if len(kept) == 0 && len(list) > 0 {
			continue
		}
		group["hooks"] = kept
		out = append(out, group)
	}
	return out
}

// ApplyToFile runs change on the settings file, keeping a timestamped backup
// of the previous version and replacing the file atomically. It returns the
// backup path ("" if there was no previous file).
func ApplyToFile(path string, change func([]byte) ([]byte, error)) (backup string, err error) {
	// Settings are often symlinked from a dotfiles repo: edit the target,
	// don't replace the link with a regular file.
	if real, err := filepath.EvalSymlinks(path); err == nil {
		path = real
	}
	old, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return "", err
	}
	updated, err := change(old)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	mode := fs.FileMode(0o600)
	if st, err := os.Stat(path); err == nil {
		mode = st.Mode().Perm()
		backup = fmt.Sprintf("%s.bak-timeweft-%s", path, time.Now().Format("20060102-150405"))
		if err := os.WriteFile(backup, old, mode); err != nil {
			return "", err
		}
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".settings-*.json")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(updated); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	return backup, os.Rename(tmp.Name(), path)
}
