# timeweft

**Time tracking for parallel coding agents.** Run Claude Code, Codex & co. on several branches at once — timeweft records where *your* attention actually went, merges in meetings from your calendar, and drafts Jira timesheets for you to approve.

> 🚧 **Work in progress** — design phase, nothing to install yet. Watch the repo to follow along.

## Why

With three agents running on three branches, an hour of wall-clock time is not three hours of work. Existing trackers measure editor activity or single sessions; none of them models *you* switching between parallel agents, or turns that into a timesheet.

In weaving, the **warp** threads run in parallel — your agents and branches. The **weft** is the single thread that crosses them — your attention. timeweft records the weft.

## How it works (planned)

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

- [ ] `timeweft` CLI + SQLite event log, Claude Code hooks
- [ ] Read-only git enrichment (commits via `git log`, branch switches via `git reflog`)
- [ ] `timeweft report` (day / week / topic)
- [ ] Branch → ticket mapping
- [ ] Calendar import: agent reads Outlook on the web via the browser → `import_events` (ICS optional)
- [ ] MCP server (`timeline`, `import_events`, `draft_worklogs`, `mark_exported`)
- [ ] Claude Code plugin with `/timesheet` skill
- [ ] Collectors for Codex and other agents

## License

[MIT](LICENSE)
