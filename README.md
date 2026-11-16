# timeweft

[![CI](https://github.com/leue21/timeweft/actions/workflows/ci.yml/badge.svg)](https://github.com/leue21/timeweft/actions/workflows/ci.yml)

**Time tracking for parallel coding agents.** Run Claude Code, Codex & co. on several branches at once — timeweft records where *your* attention actually went, merges in meetings from your calendar, and drafts Jira timesheets for you to approve.

> 🚧 **Early stage** — recording and reports work for Claude Code; calendar, Jira and MCP are next.

## Why

With three agents running on three branches, an hour of wall-clock time is not three hours of work. Existing trackers measure editor activity or single sessions; none of them models *you* switching between parallel agents, or turns that into a timesheet.

In weaving, the **warp** threads run in parallel — your agents and branches. The **weft** is the single thread that crosses them — your attention. timeweft records the weft.

## Quick start

Install (Linux, macOS — amd64, arm64, armv7):

```sh
curl -fsSL https://raw.githubusercontent.com/leue21/timeweft/main/install.sh | sh
```

Windows (PowerShell):

```powershell
irm https://raw.githubusercontent.com/leue21/timeweft/main/install.ps1 | iex
```

The scripts verify the download against the release checksums. Prebuilt binaries are on the [releases page](https://github.com/leue21/timeweft/releases) once the first version is tagged; until then build from source with `go install github.com/leue21/timeweft/cmd/timeweft@latest` or `make install` in a clone.

```sh
timeweft setup claude        # adds hooks to ~/.claude/settings.json (backup kept)
# ... work with Claude Code as usual ...
timeweft report              # today; also: yesterday, week, lastweek, 2026-11-16, --json
```

```
timeweft — Mon 2026-11-16

REPO           BRANCH               ATTENTION   AGENT   PROMPTS
acme/api       feat/PROJ-123-login  1:45        3:10    23
acme/web       fix/PROJ-130         0:50        1:05    9
TOTAL                               2:35        4:15    32
```

- **Attention** — your time. It goes to the session you last prompted, until you prompt another one. After more than 15 min without a prompt (`--idle`), only 5 min (`--tail`) are credited. Never overlaps, so it can go on a timesheet.
- **Agent** — how long agents were busy (prompt → stop). Parallel agents overlap; informational.

`timeweft setup claude --print` shows the change without writing it, `--remove` takes the hooks out again. The hook stores session id, time, repo and branch — never your prompts. Data lives in `~/.local/share/timeweft/timeweft.db` (`$TIMEWEFT_DB`).

## How it works

```
 agent hooks (prompts/stops) ──────────────► timeweft (local CLI + SQLite)
 git history (log, reflog — read-only) ────►
                                                 │  event log → topics → time blocks
                                                 │
                                    MCP server ◄─┘
                                        ▲
             your coding agent ─────────┘  uses its own connectors for
             (/timesheet skill)               Calendar · Jira · browser fallback
```

- **Local-first:** recording is a fast hook writing to SQLite. No cloud, no agent needed to record.
- **Never touches your repos:** no git hooks, no config changes. Commits and branch switches are read afterwards from `git log` / `git reflog` — safe for company repos with their own hook setups.
- **Two kinds of time:** *agent time* (how long an agent worked on a branch) vs. *attention time* (your prompting, reviewing, meetings) — only the latter goes on a timesheet.
- **Branches are the unit:** time is counted per repo + branch, regardless of how it's checked out (worktree, clone, plain checkout). Branches roll up into topics: `feat/PROJ-123-login` → `PROJ-123`, configurable.
- **The agent is the integration layer:** timeweft exposes an MCP server; your agent pulls meetings from your calendar and pushes worklogs to Jira through the connectors it already has — no extra API tokens for locked-down corporate tools.
- **Deterministic numbers, human approval:** the tracker does the math; the agent only moves data and writes descriptions. Nothing is posted without your OK, and nothing is posted twice.

## Roadmap

- [x] `timeweft` CLI + SQLite event log, Claude Code hooks
- [ ] Read-only git enrichment (commits via `git log`, branch switches via `git reflog`)
- [x] `timeweft report` (day / week, text and JSON)
- [ ] Branch → ticket mapping
- [ ] Calendar import: agent reads Outlook on the web via the browser → `import_events` (ICS optional)
- [ ] MCP server (`timeline`, `import_events`, `draft_worklogs`, `mark_exported`)
- [ ] Claude Code plugin with `/timesheet` skill
- [ ] Collectors for Codex and other agents

## License

[MIT](LICENSE)
