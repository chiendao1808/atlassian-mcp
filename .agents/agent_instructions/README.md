# Shared Agent Instructions (single source of truth)

The operating instructions for each agent type live here **once**, tool-agnostically. Each per-tool config **references** the matching file and instructs the agent to read it at the start of a task — there is no build step and no generated files to keep in sync.

## Files

| Shared instructions (edit this) | Referenced by (Claude) | Referenced by (Codex) |
|---|---|---|
| `req_analyzer.md` | `.claude/agents/req_analyzer.md` | `.codex/agents/req_analyzer.toml` |
| `explorer.md` | `.claude/agents/explorer.md` | `.codex/agents/explorer.toml` |
| `designer.md` | `.claude/agents/designer.md` | `.codex/agents/designer.toml` |
| `planner.md` | `.claude/agents/planner.md` | `.codex/agents/planner.toml` |
| `implementer.md` | `.claude/agents/implementer.md` | `.codex/agents/implementer.toml` |
| `code_reviewer.md` | `.claude/agents/code_reviewer.md` | `.codex/agents/code_reviewer.toml` |

## How it works

Each per-tool config keeps only its own metadata — Claude frontmatter (`name`, `description`, `model`, `effort`, `permissionMode`, plus a documented sandbox note); Codex (`model`, `model_reasoning_effort`, `sandbox_mode`, `approval_policy`, and extras like `web_search` / `nickname_candidates`).

**Tool access (Claude):** most agents omit `tools` and inherit the full session tool pool — including MCP / connector tools — using `disallowedTools` as a denylist (proactive control: grant by default, block the few dangerous tools). The read-only agents (`req_analyzer`, `explorer`, `designer`, `planner`, `code_reviewer`) deny `Edit, Write, NotebookEdit` (read-only) but keep `Agent`, so they may spawn other agent types per their instructions. `implementer` is unrestricted (no allowlist or denylist). To give an agent a specific MCP server, nothing is needed (it inherits it); to remove one, add `mcp__<server>` to its `disallowedTools`. Spawn governance: a PreToolUse hook (`.claude/hooks/block-recursive-agent.js`, wired in `.claude/settings.json`) denies an agent spawning another agent of the **same** type (no self-recursion with an idle parent); different-type spawns per instructions are allowed. Its instruction body is a short pointer that tells the agent:

**Exception — code_reviewer:** allows `Edit` and `Write` solely for active review reports in the workspace, as enforced by its shared instructions; it still denies `NotebookEdit`.

> Read `.agents/agent_instructions/<type>.md` in full and follow it as your operating instructions.

Every agent runs read-enabled, so it can load the shared file at task start.

## Update workflow

1. Edit the relevant `.agents/agent_instructions/<type>.md`.
2. That's it — all tools pick up the change on the next run, because they read the same file. No regeneration.

Only touch a per-tool config when its **metadata** changes (a model swap, a tool-access change, a new approval policy).

## Trade-off

This references the shared file at runtime instead of inlining it, so it depends on the agent actually reading the file (the pointer instruction is written to be unambiguous). In exchange there is no script dependency and nothing to regenerate — chosen deliberately over the earlier generator approach to avoid a build-step compatibility risk. `generate_agents.py.deprecated` is the retired generator, kept only for reference.
