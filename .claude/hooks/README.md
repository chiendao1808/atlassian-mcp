# Claude Code hooks

## block-recursive-agent.js

`PreToolUse` hook on the `Agent` tool. Prevents an agent from spawning another agent of the **same function** (same `agent_type`) — the wasteful pattern where a subagent clones itself and then idles waiting for the clone. Spawning a **different** agent type (as the caller's instructions require) is allowed, and the main thread can spawn anything, including multiple agents of one type.

Logic: deny only when the calling subagent's `agent_type` equals the requested `subagent_type`. The main thread has no `agent_type`, so it is never blocked.

Wired in [`../settings.json`](../settings.json) under `hooks.PreToolUse` with `matcher: "Agent"`. Runs via Node (no external deps), so it works cross-platform.
