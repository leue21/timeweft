// Command timeweft tracks where your attention goes while coding agents work
// in parallel on different branches.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/leue21/timeweft/internal/claude"
	"github.com/leue21/timeweft/internal/gitinfo"
	"github.com/leue21/timeweft/internal/report"
	"github.com/leue21/timeweft/internal/store"
	"github.com/leue21/timeweft/internal/timeline"
)

var version = "dev"

const usage = `timeweft — time tracking for parallel coding agents

Usage:
  timeweft report [DAY] [flags]   time per repo and branch
  timeweft events [DAY]           raw recorded events
  timeweft setup claude [flags]   add timeweft hooks to Claude Code's user settings
  timeweft hook claude            record a Claude Code hook event (reads JSON on stdin)
  timeweft version

DAY is today (default), yesterday, week, lastweek, or YYYY-MM-DD.

Data: $TIMEWEFT_DB, default ~/.local/share/timeweft/timeweft.db
`

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}
	cmd, args := os.Args[1], os.Args[2:]
	var err error
	switch cmd {
	case "hook":
		os.Exit(runHook(args, os.Stdin))
	case "report":
		err = runReport(args, os.Stdout)
	case "events":
		err = runEvents(args, os.Stdout)
	case "setup":
		err = runSetup(args, os.Stdout)
	case "version", "--version":
		fmt.Println("timeweft", version)
	case "help", "-h", "--help":
		fmt.Print(usage)
	default:
		fmt.Fprintf(os.Stderr, "timeweft: unknown command %q\n\n%s", cmd, usage)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "timeweft:", err)
		os.Exit(1)
	}
}

// runHook records one hook event. It must never disturb the agent: it prints
// nothing to stdout (Claude Code would add it to the conversation) and always
// exits 0. Failures go to hook-errors.log next to the database.
func runHook(args []string, stdin io.Reader) int {
	if len(args) != 1 || args[0] != "claude" {
		fmt.Fprintln(os.Stderr, "usage: timeweft hook claude")
		return 0
	}
	if err := recordClaudeHook(stdin, time.Now()); err != nil {
		logHookError(err)
	}
	return 0
}

func recordClaudeHook(stdin io.Reader, now time.Time) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	payload, err := io.ReadAll(io.LimitReader(stdin, 1<<20))
	if err != nil {
		return err
	}
	e, ok, err := claude.ParseHook(payload, now)
	if err != nil || !ok {
		return err
	}
	info := gitinfo.Resolve(e.Cwd)
	e.Repo, e.Branch = info.Repo, info.Branch
	st, err := openStore()
	if err != nil {
		return err
	}
	defer st.Close()
	return st.Add(e)
}

func logHookError(err error) {
	path, perr := store.DefaultPath()
	if perr != nil {
		return
	}
	logPath := filepath.Join(filepath.Dir(path), "hook-errors.log")
	_ = os.MkdirAll(filepath.Dir(logPath), 0o700)
	f, ferr := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if ferr != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s %v\n", time.Now().Format(time.RFC3339), err)
}

func openStore() (*store.Store, error) {
	path, err := store.DefaultPath()
	if err != nil {
		return nil, err
	}
	return store.Open(path)
}

func runReport(args []string, w io.Writer) error {
	fs := flag.NewFlagSet("report", flag.ContinueOnError)
	asJSON := fs.Bool("json", false, "output JSON")
	p := timeline.DefaultParams(time.Now())
	fs.DurationVar(&p.Idle, "idle", p.Idle, "longest gap still counted as continuous attention")
	fs.DurationVar(&p.Tail, "tail", p.Tail, "time credited after the last prompt before going idle")
	pos, err := parseInterspersed(fs, args)
	if err != nil {
		return err
	}
	from, to, label, err := parseRange(pos, p.Now)
	if err != nil {
		return err
	}
	st, err := openStore()
	if err != nil {
		return err
	}
	defer st.Close()
	margin := report.Lookback(p)
	evs, err := st.Between(from.Add(-margin), to.Add(margin))
	if err != nil {
		return err
	}
	rows := report.Build(evs, from, to, p)
	if *asJSON {
		return report.JSON(w, from, to, rows)
	}
	return report.Text(w, "timeweft — "+label, rows)
}

func runEvents(args []string, w io.Writer) error {
	fs := flag.NewFlagSet("events", flag.ContinueOnError)
	pos, err := parseInterspersed(fs, args)
	if err != nil {
		return err
	}
	from, to, _, err := parseRange(pos, time.Now())
	if err != nil {
		return err
	}
	st, err := openStore()
	if err != nil {
		return err
	}
	defer st.Close()
	evs, err := st.Between(from, to)
	if err != nil {
		return err
	}
	for _, e := range evs {
		sess := e.Session
		if len(sess) > 8 {
			sess = sess[:8]
		}
		fmt.Fprintf(w, "%s  %-13s %-8s %s  %s\n", e.Time.Format("2006-01-02 15:04:05"), e.Kind, sess, report.DisplayRepo(e.Repo), e.Branch)
	}
	return nil
}

func runSetup(args []string, w io.Writer) error {
	if len(args) == 0 || args[0] != "claude" {
		return fmt.Errorf("usage: timeweft setup claude [--print] [--remove]")
	}
	fs := flag.NewFlagSet("setup claude", flag.ContinueOnError)
	printOnly := fs.Bool("print", false, "print the resulting settings instead of writing them")
	remove := fs.Bool("remove", false, "remove timeweft hooks")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if exe, err = filepath.EvalSymlinks(exe); err != nil {
		return err
	}
	change := func(b []byte) ([]byte, error) { return claude.Install(b, claude.HookCommand(exe)) }
	if *remove {
		change = claude.Uninstall
	}
	path, err := claude.SettingsPath()
	if err != nil {
		return err
	}
	if *printOnly {
		old, err := os.ReadFile(path)
		if err != nil && !os.IsNotExist(err) {
			return err
		}
		out, err := change(old)
		if err != nil {
			return err
		}
		_, err = w.Write(out)
		return err
	}
	backup, err := claude.ApplyToFile(path, change)
	if err != nil {
		return err
	}
	verb := "Installed timeweft hooks (" + strings.Join(claude.HookEvents, ", ") + ") in"
	if *remove {
		verb = "Removed timeweft hooks from"
	}
	fmt.Fprintf(w, "%s %s\n", verb, path)
	if backup != "" {
		fmt.Fprintf(w, "Backup of the previous file: %s\n", backup)
	}
	if !*remove {
		fmt.Fprintln(w, "New Claude Code sessions will be recorded; running sessions pick it up after a restart.")
	}
	return nil
}

// parseInterspersed allows flags before and after positional arguments.
func parseInterspersed(fs *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		args = fs.Args()
		if len(args) == 0 {
			return pos, nil
		}
		pos = append(pos, args[0])
		args = args[1:]
	}
}

// parseRange turns a DAY argument into a local-time range [from, to).
func parseRange(pos []string, now time.Time) (from, to time.Time, label string, err error) {
	if len(pos) > 1 {
		return from, to, "", fmt.Errorf("expected at most one DAY argument, got %q", pos)
	}
	arg := "today"
	if len(pos) == 1 {
		arg = pos[0]
	}
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	monday := today.AddDate(0, 0, -((int(today.Weekday()) + 6) % 7))
	switch arg {
	case "today":
		from = today
	case "yesterday":
		from = today.AddDate(0, 0, -1)
	case "week", "lastweek":
		from = monday
		if arg == "lastweek" {
			from = monday.AddDate(0, 0, -7)
		}
		to = from.AddDate(0, 0, 7)
		return from, to, fmt.Sprintf("week of %s", from.Format("Mon 2006-01-02")), nil
	default:
		from, err = time.ParseInLocation("2006-01-02", arg, now.Location())
		if err != nil {
			return from, to, "", fmt.Errorf("invalid DAY %q: use today, yesterday, week, lastweek or YYYY-MM-DD", arg)
		}
	}
	return from, from.AddDate(0, 0, 1), from.Format("Mon 2006-01-02"), nil
}
