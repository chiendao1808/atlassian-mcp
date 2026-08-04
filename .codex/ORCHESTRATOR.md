# Orchestrator (Codex)

This is a pointer file for easy per-tool copying. The authoritative orchestration guide — workflow selection, supplied-plan reuse, code-mutation boundary, state ownership, approval gates, and completion rules — lives in the shared docs:

**→ [`.agents/orchestration/ORCHESTRATOR.md`](../.agents/orchestration/ORCHESTRATOR.md)**

Supported workflows (all defined under `.agents/orchestration/`):

- Feature Development — [`wf_feature_development/WORKFLOW.md`](../.agents/orchestration/wf_feature_development/WORKFLOW.md)
- Bug Resolution — [`wf_bug_resolving/WORKFLOW.md`](../.agents/orchestration/wf_bug_resolving/WORKFLOW.md)
- Code Review & Remediation — [`wf_code_review/WORKFLOW.md`](../.agents/orchestration/wf_code_review/WORKFLOW.md)

Agents are defined in [`.codex/agents/`](./agents/) and spawned via `spawn_agent(agent_type = "...")`. See [`.codex/AGENTS.md`](./AGENTS.md) for the agent catalog.
