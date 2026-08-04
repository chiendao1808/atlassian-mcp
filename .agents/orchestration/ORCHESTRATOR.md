# Claude Code Workflow Orchestrator Guide

## Purpose

This guide defines how the main agent or orchestrator selects and executes one of the supported software-delivery workflows after the request has been clarified.

The orchestrator owns workflow selection, runtime state, transitions, approvals, artifact references, retries, and completion. Subagents perform only the state assigned to them and must not advance the workflow independently.

Supported workflows:

| Workflow | Name | Definition |
|---|---|---|
| Feature Development | `wf_feature_development` | [`wf_feature_development/WORKFLOW.md`](./wf_feature_development/WORKFLOW.md) |
| Bug Resolution | `wf_bug_resolving` | [`wf_bug_resolving/WORKFLOW.md`](./wf_bug_resolving/WORKFLOW.md) |
| Code Review and Remediation | `wf_code_review` | [`wf_code_review/WORKFLOW.md`](./wf_code_review/WORKFLOW.md) |

## Clarify Before Selecting a Workflow

Do not select a workflow from the user's first wording alone. First establish a normalized request brief, normally by using `req_analyzer` when the request contains external documents, issue links, incomplete requirements, or mixed intent.

The clarified brief should identify:

- The primary request type and any secondary request types.
- The intended outcome.
- Whether existing behavior is incorrect or a new behavior is requested.
- Whether a Git change set already exists.
- Available requirement, bug, design, or review evidence.
- Blocking questions that must be answered by the user.
- Known scope boundaries, constraints, and acceptance conditions.

Remain in clarification when a missing fact could materially change the workflow choice, implementation scope, or approval boundary.

## Supplied Implementation Plan Reuse

When the user or parent supplies an implementation plan, fix plan, remediation plan, rollout plan, or equivalent execution document:

1. Preserve the original document as a source artifact and dispatch `req_analyzer` in supplied-plan review mode.
2. Require a traceable review classification: `reusable`, `reusable_with_targeted_revisions`, `not_reusable`, or `blocked_by_missing_evidence`.
3. Validate repository-specific file, symbol, dependency, test, migration, rollout, and risk claims through the workflow's normal exploration states.
4. When the plan is reusable and required exploration or design evidence confirms it, store it as the workflow plan artifact and route directly to the applicable user approval state. Do not dispatch `planner` merely to regenerate equivalent content.
5. When only bounded gaps exist, dispatch `planner` to revise the supplied plan in place, preserving valid decisions and recording a revision summary.
6. Create a replacement plan only when the supplied plan is materially incompatible, unsafe, obsolete, contradictory, or missing core decisions. Record why reuse was rejected.
7. User approval remains mandatory before any workspace mutation, even when the supplied plan is reused unchanged.

## Code Generation and Mutation Boundary

- `implementer` is the exclusive agent for generating code and for modifying code, configuration, tests, schemas, migrations, build files, infrastructure-as-code, and code-adjacent documentation or comments.
- The main agent may clarify requirements, select workflows, manage state, present approvals, and summarize results, but must not generate implementation code or perform code-related writes itself.
- This boundary applies to direct requests, quick fixes, feature implementation, bug fixes, remediation, validation fixes, and code examples intended as deliverable output.
- For non-mutating code generation, dispatch `implementer` with the requested output scope. For workspace writes, also provide the approved plan and exact approved scope.
- Never bypass `implementer` because a change is small, local, mechanical, or time-sensitive.

## Workflow Selection

### Select `wf_feature_development`

Use this workflow when the clarified request primarily requires creating or changing system behavior, including:

- A new feature or user capability.
- A feature enhancement.
- A technical change that introduces new runtime behavior, integration, schema, configuration, or operational capability.
- A UI/UX feature that requires design, planning, implementation, and review.
- A refactor that is coupled to an approved behavior or architecture change.

Do not select it merely because implementation may eventually be required. If the main purpose is correcting behavior that already has an expected contract, use `wf_bug_resolving`.

### Select `wf_bug_resolving`

Use this workflow when the clarified request states or establishes that existing behavior is wrong, including:

- A bug fix or regression.
- Actual behavior differs from expected behavior.
- An incident or defect requires code changes after evidence gathering.
- A Jira, Trello, GitHub, or similar issue describes reproduction steps and an expected result.

A suspected bug without sufficient evidence still uses this workflow, but remains in analysis or exploration until the defect and expected behavior are clear enough to plan a fix.

### Select `wf_code_review`

Use this workflow when a Git change set already exists and the primary objective is to inspect, assess, and optionally remediate it, including:

- Review staged, unstaged, committed, or untracked changes.
- Perform a pre-merge or post-implementation review.
- Re-review changes after remediation.
- Convert code-review findings into targeted fixes.

Do not select this workflow as the primary workflow for ordinary feature or bug implementation. Those workflows already contain a code-review state. Select `wf_code_review` when review itself is the entry point or main deliverable.


## Bug Complexity and Planning Profile Selection

After `wf_bug_resolving` has enough evidence to describe the defect and probable change condition, first determine whether a supplied plan has been reviewed and can be reused. Route a verified reusable plan directly to `PLAN_REVIEW`. Otherwise classify the planning need before dispatching a planner.

Record the decision in runtime state under:

```yaml
planning:
  complexity: simple | standard | complex
  risk_level: low | medium | high
  mode: quick_fix | full
  rationale: []
  escalation_triggers: []
  execution_profile:
    preferred_model: null
    preferred_reasoning_effort: null
    selected_agent_type: null
    fallback_agent_type: planner
```

### Choose the lightweight quick-fix path

Use `QUICK_FIX_PLANNING` for a deterministic, localized, low-risk correction such as a syntax fix, wrong assignment, incorrect mapping, simple field update, or bounded configuration change. All quick-fix eligibility guards in the workflow must pass.

Prefer:

```yaml
preferred_model: sonnet
preferred_reasoning_effort: medium
```

The output must be a concise quick-fix brief rather than an architecture-oriented implementation plan. A lightweight path still requires explicit scope approval, implementer verification, and code review.

### Choose the full planning path

Use `FIX_PLANNING` when the issue has uncertain root cause, cross-module scope, public contract impact, compatibility risk, schema/data implications, security concerns, concurrency or transaction behavior, messaging/cache effects, infrastructure changes, or a broad blast radius.

Use the canonical full planner profile for this route.

### Runtime profile handling

Treat `execution_profile` as an orchestration preference:

1. Use a matching configured agent profile or a supported per-dispatch model/effort selection when available.
2. If the runtime cannot apply the preferred lightweight profile, use the canonical `planner` and constrain it to the quick-fix output contract.
3. Do not edit agent Markdown files during an active workflow instance merely to change a model.
4. Escalate to full planning immediately when new evidence violates a quick-fix eligibility guard.
5. Save the selected profile and rationale in runtime state and `metadata.additional_context.notes`.

## Ambiguous and Mixed Requests

Apply these rules in order:

1. Prefer the workflow that matches the user's requested outcome, not the proposed technical solution.
2. If the request contains both a feature and a bug, separate them into independent workflow instances unless one is strictly required to complete the other.
3. If existing code changes are supplied only as evidence for a bug, select `wf_bug_resolving`; if the user asks whether those changes are safe or correct, select `wf_code_review`.
4. If a review discovers a product requirement gap rather than a code defect, pause remediation and create a separate feature or bug workflow.
5. Record the selected workflow and the evidence used to choose it in runtime `metadata.additional_context.workflow_selection`.

## Loading a Workflow

After selection:

1. Read the workflow Markdown file for intent and flow.
2. Load the referenced `state-model.md` as the authoritative state and transition definition.
3. Create a runtime state instance from `runtime_state_template`.
4. Assign a unique `workflow_id` and preserve the workflow `name` exactly.
5. Store the runtime state in a project-local location such as `.agents/orchestration/runtime/<workflow_id>.yaml` when persistence is required.
6. Do not modify the workflow definition while a workflow instance is running. Record instance-specific data only in runtime state and artifacts.

## Runtime State Ownership

The orchestrator must update runtime state after every agent result, user decision, approval, failure, and transition.

At minimum, maintain:

- `current_state`, `previous_state`, and `active_agent`.
- Request summary and classification.
- Artifact references produced by agents.
- Approval status and approved scope.
- Retry and review-cycle counters.
- Transition history.
- `metadata` for raw and additional context.

Subagents may propose a transition event, but the orchestrator must validate that the event exists for the current state and that its guard is satisfied.

## Metadata Contract

Every runtime state contains:

```yaml
metadata:
  raw_context:
    user_prompt: null
    source_snapshots: []
    agent_outputs: []
  additional_context:
    user_clarifications: []
    repository_rules: []
    project_skills: []
    constraints: []
    notes: []
    workflow_selection: {}
  tags: []
  correlation: {}
  updated_at: null
```

Use `raw_context` for unnormalized evidence or references to it, such as the original prompt, issue payloads, document extracts, logs, tool output, and full agent handoffs.

Use `additional_context` for normalized or supplemental context, such as user clarifications, applicable repository rules, inherited skill instructions, approved constraints, assumptions, and orchestrator notes.

Prefer references or artifact paths over duplicating large or sensitive payloads. Never persist secrets, credentials, private tokens, or unnecessary personal data. Label facts, hypotheses, and unresolved gaps separately.

Each state definition lists the metadata keys it expects to read or update. Preserve prior metadata unless it is explicitly superseded by verified newer evidence.

## Agent Dispatch Contract

For each state, provide the assigned agent with a self-contained task containing:

- Workflow ID and workflow name.
- Current state.
- State objective.
- Relevant artifact and metadata references.
- Applicable project rules and skill context already loaded by the parent.
- Approved scope, when writes are allowed.
- Supplied-plan source, review classification, reusable scope, revision requirements, and original-plan traceability when applicable.
- Documentation and comment coverage requirements for every code-related implementation or remediation task.
- Expected output contract.
- Allowed transition events.

Use canonical agent types from `CLAUDE.md` and `.claude/agents/*.md`.

Dispatch `code_reviewer` once per review state and require it to perform the review directly in its existing thread. It may spawn a different agent type (such as `explorer`) for supporting evidence, or return uncertain evidence to the orchestrator for an explicit `explorer` dispatch when the workflow provides that state; it must not spawn another `code_reviewer` (blocked by the recursion hook) or delegate the review judgment. In every review dispatch, instruct it to prioritize changed source code and runtime-affecting artifacts. Treat plans, handoffs, memory-bank files, and agent notes as secondary context to consult selectively unless the user explicitly requests full document review.

Do not send the full conversation when a compact state snapshot is sufficient. Preserve agent outputs in `metadata.raw_context.agent_outputs` or as referenced artifacts before compacting them into normalized workflow fields.

## Transition Evaluation

After a state completes:

1. Verify that the agent returned the required outputs.
2. Save raw output and normalized artifacts.
3. Evaluate the proposed event against the transition list.
4. Check the transition guard using verified state data.
5. Update `previous_state`, `current_state`, `active_agent`, `history`, and `metadata.updated_at`.
6. Stop at approval states until explicit user approval is received.
7. Do not treat silence, tool approval, partial feedback, or an agent recommendation as user approval.

## Approval Boundaries

User approval is required before implementation when the selected workflow reaches an approval state.

The orchestrator must record:

- The exact requested scope.
- The files, modules, or artifacts expected to change when known.
- Material migrations, dependencies, external effects, or destructive operations.
- The user's approval status.
- The approved scope that write-enabled agents may act within.

If implementation expands beyond the approved scope, return to the relevant review or approval state.

Every implementation or remediation approval must include documentation and intent-comment coverage for the changed logic. The implementer must return `documentation_updates` and `comment_coverage`; missing coverage blocks successful self-verification unless a repository rule explicitly prohibits that documentation form and an equivalent approved form is used.

## Failure, Timeout, and Blocked Handling

- If an agent is missing required context, move to the workflow's clarification or blocked state instead of guessing.
- If an indexed MCP or specialized connector fails, use the documented fallback and record the limitation in metadata.
- If a spawned explorer times out, the parent continues targeted exploration and does not spawn a replacement explorer for the same task.
- If an implementer finds an out-of-scope issue, preserve the current changes, record the blocker, and return control to the orchestrator.
- If a review finding is uncertain, gather evidence before routing it to remediation.
- Never transition to `COMPLETED` while a required approval, blocking finding, failed verification, unresolved mandatory artifact, or missing documentation/comment coverage remains.

## Completion

A workflow is complete only when its terminal-state guard is satisfied and the runtime state contains:

- Final outcome summary.
- Relevant artifact references.
- Verification and review status when applicable.
- Documentation updates and comment coverage for every created or modified logic unit.
- Deferred issues or residual risks.
- Final metadata and transition history.
