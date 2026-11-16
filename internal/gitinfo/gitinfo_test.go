package gitinfo

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestNormalizeRemote(t *testing.T) {
	cases := map[string]string{
		"git@github.com:acme/api.git":                  "github.com/acme/api",
		"https://github.com/acme/api.git":              "github.com/acme/api",
		"https://user@GitHub.com/acme/api/":            "github.com/acme/api",
		"ssh://git@github.com:22/acme/api.git":         "github.com/acme/api",
		"https://org@dev.azure.com/org/proj/_git/repo": "dev.azure.com/org/proj/_git/repo",
		"git@gitlab.company.ch:group/sub/project.git":  "gitlab.company.ch/group/sub/project",
		"/srv/git/project.git":                         "local:/srv/git/project",
		"file:///srv/git/project.git":                  "local:/srv/git/project",
	}
	for in, want := range cases {
		if got := NormalizeRemote(in); got != want {
			t.Errorf("NormalizeRemote(%q) = %q, want %q", in, got, want)
		}
	}
}

func run(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func TestResolveWorktreesShareRepo(t *testing.T) {
	root := t.TempDir()
	main := filepath.Join(root, "main")
	run(t, root, "init", "-q", "-b", "main", main)
	run(t, main, "commit", "-q", "--allow-empty", "-m", "init")
	run(t, main, "remote", "add", "origin", "git@github.com:acme/api.git")
	wt := filepath.Join(root, "wt")
	run(t, main, "worktree", "add", "-q", "-b", "feat/PROJ-1", wt)
	sub := filepath.Join(wt, "pkg")
	if err := os.Mkdir(sub, 0o755); err != nil {
		t.Fatal(err)
	}

	if got := Resolve(main); got != (Info{"github.com/acme/api", "main"}) {
		t.Errorf("main checkout: %+v", got)
	}
	if got := Resolve(sub); got != (Info{"github.com/acme/api", "feat/PROJ-1"}) {
		t.Errorf("worktree subdir: %+v", got)
	}

	run(t, main, "checkout", "-q", "--detach")
	if got := Resolve(main); !strings.HasPrefix(got.Branch, "detached@") {
		t.Errorf("detached: %+v", got)
	}
}

func TestResolveWithoutRemoteOrGit(t *testing.T) {
	root := t.TempDir()
	run(t, root, "init", "-q", "-b", "trunk", ".")
	got := Resolve(root)
	real, _ := filepath.EvalSymlinks(root)
	if got.Branch != "trunk" || (got.Repo != "local:"+root && got.Repo != "local:"+real) {
		t.Errorf("no remote, unborn branch: %+v", got)
	}

	plain := t.TempDir()
	if got := Resolve(plain); got != (Info{Repo: "dir:" + plain}) {
		t.Errorf("outside git: %+v", got)
	}
}
