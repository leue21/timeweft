// Package report aggregates time per repo and branch for a time range.
package report

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/leue21/timeweft/internal/events"
	"github.com/leue21/timeweft/internal/timeline"
)

type Row struct {
	Repo      string        `json:"repo"`
	Branch    string        `json:"branch"`
	Attention time.Duration `json:"-"`
	Agent     time.Duration `json:"-"`
	Prompts   int           `json:"prompts"`
}

// Lookback is how far before/after a range events must be loaded so that
// blocks crossing the range boundaries are computed correctly.
func Lookback(p timeline.Params) time.Duration { return max(p.Idle, p.MaxAgentRun) }

// Build aggregates evs (loaded with Lookback margin on both sides) into rows
// for [from, to), sorted by attention, then agent time.
func Build(evs []events.Event, from, to time.Time, p timeline.Params) []Row {
	rows := map[timeline.Key]*Row{}
	row := func(k timeline.Key) *Row {
		r, ok := rows[k]
		if !ok {
			r = &Row{Repo: k.Repo, Branch: k.Branch}
			rows[k] = r
		}
		return r
	}
	for _, b := range timeline.Clip(timeline.Attention(evs, p), from, to) {
		row(b.Key).Attention += b.Duration()
	}
	for _, b := range timeline.Clip(timeline.AgentRuns(evs, p), from, to) {
		row(b.Key).Agent += b.Duration()
	}
	for _, e := range evs {
		if e.Kind == events.Prompt && !e.Time.Before(from) && e.Time.Before(to) {
			row(timeline.Key{Repo: e.Repo, Branch: e.Branch}).Prompts++
		}
	}
	out := make([]Row, 0, len(rows))
	for _, r := range rows {
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Attention != b.Attention {
			return a.Attention > b.Attention
		}
		if a.Agent != b.Agent {
			return a.Agent > b.Agent
		}
		return a.Repo+a.Branch < b.Repo+b.Branch
	})
	return out
}

func Text(w io.Writer, title string, rows []Row) error {
	fmt.Fprintf(w, "%s\n\n", title)
	if len(rows) == 0 {
		_, err := fmt.Fprintln(w, "No activity recorded.")
		return err
	}
	tw := tabwriter.NewWriter(w, 0, 0, 3, ' ', 0)
	fmt.Fprintln(tw, "REPO\tBRANCH\tATTENTION\tAGENT\tPROMPTS")
	var att, ag time.Duration
	var prompts int
	for _, r := range rows {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%d\n", DisplayRepo(r.Repo), r.Branch, HM(r.Attention), HM(r.Agent), r.Prompts)
		att, ag, prompts = att+r.Attention, ag+r.Agent, prompts+r.Prompts
	}
	fmt.Fprintf(tw, "TOTAL\t\t%s\t%s\t%d\n", HM(att), HM(ag), prompts)
	return tw.Flush()
}

func JSON(w io.Writer, from, to time.Time, rows []Row) error {
	type jrow struct {
		Row
		AttentionSeconds int64 `json:"attention_seconds"`
		AgentSeconds     int64 `json:"agent_seconds"`
	}
	out := struct {
		From time.Time `json:"from"`
		To   time.Time `json:"to"`
		Rows []jrow    `json:"rows"`
	}{From: from, To: to, Rows: []jrow{}}
	for _, r := range rows {
		out.Rows = append(out.Rows, jrow{r, int64(r.Attention.Seconds()), int64(r.Agent.Seconds())})
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// HM formats a duration as h:mm, rounded to the minute.
func HM(d time.Duration) string {
	m := int(d.Round(time.Minute).Minutes())
	return fmt.Sprintf("%d:%02d", m/60, m%60)
}

// DisplayRepo shortens repo identities for humans:
// "github.com/acme/api" -> "acme/api", "dir:/home/me/x" -> "~/x".
func DisplayRepo(repo string) string {
	for _, prefix := range []string{"dir:", "local:"} {
		if p, ok := strings.CutPrefix(repo, prefix); ok {
			if home, err := os.UserHomeDir(); err == nil && home != "" {
				if rest, ok := strings.CutPrefix(p, home); ok {
					p = "~" + rest
				}
			}
			return p
		}
	}
	if _, rest, ok := strings.Cut(repo, "/"); ok {
		return rest
	}
	return repo
}
