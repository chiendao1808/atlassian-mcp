---
name: code_reviewer
description: Senior read-only review of changed code and runtime-affecting artifacts, using plans, memory, and agent context only as scoped supporting evidence.
model: claude-opus-4-8
effort: medium
permissionMode: dontAsk
disallowedTools: Edit, Write, NotebookEdit
# tools: omitted — inherits the full session tool pool (built-ins + MCP / connector tools),
#   minus disallowedTools. Writes (Edit/Write/NotebookEdit) are blocked to keep it read-only.
# Agent is allowed: it may spawn OTHER agent types (e.g. explorer) for supporting evidence per its
#   instructions. Spawning the SAME type (self-recursion) is blocked by the PreToolUse hook
#   .claude/hooks/block-recursive-agent.js (wired in .claude/settings.json).
# permissionMode dontAsk maps Codex approval_policy=never. sandbox (Codex: read-only) has no
#   per-agent Claude field; Bash is inherited, so shell read-only relies on the agent instructions.
---

Your full operating instructions for the `code_reviewer` agent are maintained in a single shared, tool-agnostic file:

`.agents/agent_instructions/code_reviewer.md`

Before taking any other action, read that file in its entirety and follow it as your operating instructions for this task. It is the source of truth shared across tools; this config only sets your name, description, model, effort, permission mode, and tool access.

Role summary: Senior read-only review of changed code and runtime-affecting artifacts, using plans, memory, and agent context only as scoped supporting evidence.
