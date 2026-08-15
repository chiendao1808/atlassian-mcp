# Claude Agent Catalog

This file summarizes the custom subagents available under `.claude/agents/` and the orchestration rules that govern how the main agent coordinates them. Detailed behavior, permissions, tool usage, and execution rules are defined in the corresponding subagent Markdown files. This is the Claude Code counterpart of the former `.codex/AGENTS.md`.

Spawn a subagent with the Task tool using its `subagent_type`, e.g. `subagent_type="implementer"`. The main agent orchestrates. Subagents may spawn a different agent type per their instructions (e.g. an analysis agent spawning peers, or `code_reviewer` spawning `explorer` for evidence); `implementer` is unrestricted. A PreToolUse hook (`.claude/hooks/block-recursive-agent.js`, wired in `.claude/settings.json`) blocks any agent from spawning another agent of the SAME type (self-recursion while the parent idles).

## Subagents

### `req_analyzer`

Analyzes incoming requests and gathers the information needed before design, planning, investigation, or implementation.

- Classify the request (new feature, enhancement, bug fix, investigation, refactor, maintenance).
- Read requirement docs, issue-tracker items, web pages, logs, screenshots, and threads.
- Normalize feature requirements into goals, inputs, outputs, business rules, constraints, acceptance criteria, dependencies, and open questions.
- Summarize bug reports with reproduction steps, actual/expected behavior, priority, impact, and technical clues.
- Review supplied implementation/fix/remediation plans for reuse without authoring a new plan.

Read-only. Model: opus.

### `explorer`

Performs focused, read-only exploration of the codebase and returns implementation evidence.

- Locate relevant files, symbols, modules, configurations, tests, and dependencies.
- Trace callers, callees, execution paths, data flows, and integration points.
- Discover project rules, memory-bank content, agent context, and applicable project skills.
- Report exact file paths, symbols, relationships, evidence, and unresolved gaps.

Read-only. Model: sonnet.

### `uiux_designer`

Handles product design, web design, application design, UI/UX, accessibility, and design-system tasks.

- Discover and apply relevant design skills and project design guidance.
- Analyze existing interfaces, design tokens, components, Figma references, and brand constraints.
- Define user flows, information architecture, interaction behavior, visual direction, responsive behavior, component states, and accessibility requirements.
- Produce design specifications, implementation-ready handoffs, or approved design changes.

Read-only, with web search. Model: sonnet.

### `planner`

Creates an implementation-ready technical plan based on verified requirements and the current codebase.

- Define the proposed solution and approach, or revise a reviewed supplied plan while preserving valid decisions and traceability.
- Identify affected modules, interfaces, data flows, dependencies, migrations, risks, and rollout considerations.
- Adapt the solution to the detected technology stack and conventions.
- Identify missing information and blocking questions instead of making unsupported assumptions.
- Include a tester-consumable `verification_plan` with deterministic expected results for code-changing work.
- Present the plan for user review and approval.

May create or update planning documents only in the workspace. Model: opus.

### `implementer`

Exclusively handles production code generation and approved production code-related changes; the main agent must not implement or generate code directly.

- Generate requested production code artifacts and modify production code, configuration, documentation, comments, and migrations within the approved scope.
- Add or update developer documentation and intent comments for every created or modified logic unit, then report coverage.
- Follow project rules, coding conventions, applicable skills, and existing repository patterns.
- Review the resulting Git diff and run appropriate compile/build validation before tester handoff.
- Fix compile/build validation errors caused by its changes when they remain within the approved scope.
- Return a concise implementation and compile/build validation summary.

Planned test creation and execution belong to `tester`.

Workspace-write. Model: sonnet.

### `tester`

Creates approved test-only artifacts, executes the approved verification plan, and returns evidence without modifying production code.

- Add or update test source, test scripts, fixtures, mocks/stubs/fakes, snapshots/golden files, and test-only harness configuration within approved verification scope.
- Run mandatory verification cases and broader planned suites.
- Report result, probable cause on failure, commands, coverage, logs/exit codes, and detailed evidence.
- Classify failures as `production_code`, `test_artifact`, `environment_or_tooling`, `verification_plan_gap`, or `unknown` for orchestrator routing.
- Never modify production code or choose workflow transitions.

Workspace-write restricted to test-only artifacts. Model: `claude-sonnet-4-6`, effort: `max`.

### `code_reviewer`

Reviews the current Git changes and may create or update active review reports in the workspace.

- Review staged, unstaged, and relevant untracked changes.
- Prioritize changed source code, tests, configuration, schemas, migrations, and build/deployment files.
- Consult plans, handoffs, memory files, and agent notes only as scoped supporting context.
- Perform the review directly and own the findings; may spawn a different agent type for supporting evidence, but returns fixes to the appropriate owner (`implementer` for production/runtime findings, `tester` for test-only findings) rather than applying them.
- Check changes against project rules, review checklists, coding conventions, and detected technology patterns.
- Identify correctness, compatibility, security, performance, concurrency, data, integration, side-effect, and documentation risks.
- Report each issue with severity, description, file/line position, impact, evidence, and suggested fix.

May create or update active review reports only in the workspace. Model: opus.

## Skills

Repository-local skills live under [`.agents/skills/`](../.agents/skills/). The main agent and subagents must look there when deciding whether a task has applicable skills, in addition to any globally installed skills made available by Claude Code or the surrounding runtime.

- [`.agents/skills/self/`](../.agents/skills/self/) contains personal skills maintained for this workspace. Prefer these when they match the task because they capture local conventions and user-specific expectations.
- [`.agents/skills/community/`](../.agents/skills/community/) contains third-party or community-collected skills. Use these when they match the technology, workflow, or review concern being handled.
- Treat each `SKILL.md` under those folders as a skill entrypoint. Search by folder name, frontmatter `name`/`description`, and task keywords; `rg --files .agents/skills -g SKILL.md` is the fastest inventory command.
- Before taking task actions, read the selected `SKILL.md` completely and follow its trigger rules, checklist, references, and relative-path instructions. Resolve referenced files relative to that skill's folder.
- If multiple skills apply, load the smallest useful set. Process/orchestration skills should guide the approach first, then language, framework, testing, documentation, or review skills.
- If no local skill applies, say so only when relevant and continue with the normal agent instructions.

## Orchestration

Workflow selection and orchestration rules are defined separately in [`.claude/orchestration/ORCHESTRATOR.md`](./.claude/ORCHESTRATOR.md).

The main agent owns workflow selection, runtime state, transitions, approvals, and completion. Because Claude Code has no built-in workflow state engine, the `state-model.md` files are authoritative *specifications* that the main agent follows manually — they are not automatically enforced.

### Core boundaries

- Production code generation and production code-related writes must be performed by `implementer`; approved test-only writes and planned behavioral verification must be performed by `tester`. The main agent may clarify, select workflows, manage state, present approvals, and summarize results, but must not generate implementation or test code itself.
- Every successful production mutation must pass implementer compile/build validation and then `TESTING` before code review, re-review, or completion.
- User approval is required before any workspace mutation, even when a supplied plan is reused unchanged.
- `code_reviewer` performs its review directly and owns the findings; it returns production/runtime fixes to `implementer` and test-only fixes to `tester` rather than applying them, and never spawns another reviewer.
- Read-only subagents are constrained by their `tools` allowlist. Approval gating for writes is enforced via Claude Code `permissions` in `.claude/settings.json`, not per-agent.

## Migration notes (Codex → Claude)

- `model_reasoning_effort` and `preferred_reasoning_effort` have no Claude Code equivalent and are retained only as intent labels; they have no runtime effect.
- `approval_policy` / `sandbox_mode` were per-agent in Codex; here they map to tool allowlists plus project-level `permissions`.
- Model mapping applied: `gpt-5.6-sol` → opus (planner), `gpt-5.5` / `gpt-5.6-terra` → sonnet (existing non-tester agents). `tester` explicitly uses Claude Sonnet 4.6 with `max` effort.

## Agent instructions (single source)

Each subagent's operating instructions are maintained once, tool-agnostically, in `.agents/agent_instructions/<type>.md`. The per-tool configs under `.claude/agents/*.md` (and `.codex/agents/*.toml`) keep only their own metadata; their body is a short pointer that tells the agent to read the shared file and follow it. There is no build step — edit the shared file and every tool picks up the change on its next run.

See [`.agents/agent_instructions/README.md`](../.agents/agent_instructions/README.md) for the mapping and update workflow.
