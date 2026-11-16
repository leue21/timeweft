// Package gitinfo resolves which repository and branch a directory belongs to.
//
// It only ever reads: no hooks, no config writes, and GIT_OPTIONAL_LOCKS=0 so
// git does not even refresh the index. Safe to run inside company repos.
package gitinfo

import (
	"context"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Info identifies the unit of tracking: a repository and a branch.
// Worktrees and separate clones of the same repository share one Repo.
type Info struct {
	Repo   string // e.g. "github.com/acme/api", "local:/path/to/repo", "dir:/some/dir"
	Branch string // branch name, "detached@<sha>", or "" outside git
}

// Resolve inspects dir. It never fails: outside a repository it returns
// Repo "dir:<dir>" and an empty branch.
func Resolve(dir string) Info {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	commonDir, err := git(ctx, dir, "rev-parse", "--path-format=absolute", "--git-common-dir")
	if err != nil {
		return Info{Repo: "dir:" + dir}
	}

	var info Info
	if origin, err := git(ctx, dir, "config", "--get", "remote.origin.url"); err == nil && origin != "" {
		info.Repo = NormalizeRemote(origin)
	} else {
		info.Repo = "local:" + strings.TrimSuffix(filepath.Clean(commonDir), string(filepath.Separator)+".git")
	}

	if b, err := git(ctx, dir, "symbolic-ref", "--quiet", "--short", "HEAD"); err == nil && b != "" {
		info.Branch = b
	} else if sha, err := git(ctx, dir, "rev-parse", "--short", "HEAD"); err == nil && sha != "" {
		info.Branch = "detached@" + sha
	} else {
		info.Branch = "detached"
	}
	return info
}

func git(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_OPTIONAL_LOCKS=0", "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

// NormalizeRemote turns any form of remote URL into "host/path", so that
// https and ssh clones of the same repository compare equal.
//
//	git@github.com:acme/api.git        -> github.com/acme/api
//	https://user@github.com/acme/api   -> github.com/acme/api
//	ssh://git@github.com:22/acme/api   -> github.com/acme/api
func NormalizeRemote(raw string) string {
	raw = strings.TrimSpace(raw)
	var host, path string
	switch {
	case strings.Contains(raw, "://"):
		u, err := url.Parse(raw)
		if err == nil && u.Scheme == "file" {
			return "local:" + strings.TrimSuffix(u.Path, ".git")
		}
		if err != nil || u.Host == "" {
			return "local:" + raw
		}
		host, path = u.Hostname(), u.Path
	case isSCPLike(raw):
		at := strings.Index(raw, "@")
		host, path, _ = strings.Cut(raw[at+1:], ":")
	default:
		return "local:" + strings.TrimSuffix(raw, ".git")
	}
	path = strings.Trim(strings.TrimSuffix(strings.Trim(path, "/"), ".git"), "/")
	return strings.ToLower(host) + "/" + path
}

// isSCPLike matches "[user@]host:path" but not Windows drive paths or local paths.
func isSCPLike(s string) bool {
	colon := strings.Index(s, ":")
	if colon <= 1 { // "C:\..." or no colon
		return false
	}
	slash := strings.Index(s, "/")
	return slash == -1 || colon < slash
}
