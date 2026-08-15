# Codex Agent Catalog

This file summarizes the purpose of each custom Codex agent available under `.codex/agents/`. Detailed behavior, permissions, tool usage, and execution rules are defined in the corresponding agent configuration files.

## `req_analyzer`

Analyzes incoming requests and gathers the information needed before design, planning, investigation, or implementation.

Typical responsibilities:

- Classify the request, such as a new feature, enhancement, bug fix, investigation, refactor, or maintenance task.
- Read requirement documents, issue-tracker items, web pages, logs, screenshots, and discussion threads.
- Normalize feature requirements into goals, inputs, outputs, business rules, constraints, acceptance criteria, dependencies, and open questions.
- Summarize bug reports with reproduction steps, actual and expected behavior, priority, impact, and technical clues.
- Review supplied implementation, fix, or remediation plans for reuse, targeted revision, replacement need, and approval readiness without authoring a new plan.

## `explorer`

Performs focused, read-only exploration of the codebase and returns implementation evidence.

Typical responsibilities:

- Locate relevant files, symbols, modules, configurations, tests, and dependencies.
- Trace callers, callees, execution paths, data flows, and integration points.
- Discover project rules, memory-bank content, agent context, and applicable project skills.
- Report exact file paths, symbols, relationships, evidence, and unresolved gaps.

## `uiux_designer`

Handles product design, web design, application design, UI/UX, accessibility, and design-system tasks.

Typical responsibilities:

- Discover and apply relevant design skills and project design guidance.
- Analyze existing interfaces, design tokens, components, Figma references, and brand constraints.
- Define user flows, information architecture, interaction behavior, visual direction, responsive behavior, component states, and accessibility requirements.
- Produce design specifications, implementation-ready design handoffs, or approved design changes.

## `planner`

Creates an implementation-ready technical plan based on verified requirements and the current codebase.

Typical responsibilities:

- Define the proposed solution and implementation approach, or revise a reviewed supplied plan while preserving valid decisions and traceability.
- Identify affected modules, interfaces, data flows, dependencies, migrations, risks, and rollout considerations.
- Adapt the solution to the technology stack and conventions detected in the repository.
- Identify missing information and blocking questions instead of making unsupported assumptions.
- Include a tester-consumable `verification_plan` with deterministic expected results for code-changing work.
- Present the plan for user review and approval.

## `implementer`

Exclusively handles production code generation and approved production code-related changes; the main agent must not implement or generate code directly.

Typical responsibilities:

- Generate requested production code artifacts and modify production code, configuration, documentation, comments, migrations, and related production artifacts within the approved scope.
- Add or update developer documentation and intent comments for every created or modified logic unit, then report coverage for review.
- Follow project rules, coding conventions, applicable skills, and existing repository patterns.
- Review the resulting Git diff and run appropriate compile or build validation before tester handoff.
- Fix compile/build validation errors caused by its changes when they remain within the approved scope.
- Return a concise implementation and compile/build validation summary.

Planned test creation and execution belong to `tester`.

## `tester`

Creates approved test-only artifacts, executes the approved verification plan, and returns evidence without modifying production code.

Typical responsibilities:

- Add or update test source, test scripts, fixtures, mocks/stubs/fakes, snapshots/golden files, and test-only harness configuration within approved verification scope.
- Run mandatory verification cases and broader planned suites.
- Report result, probable cause on failure, commands, coverage, logs/exit codes, and detailed evidence.
- Classify failures as `production_code`, `test_artifact`, `environment_or_tooling`, `verification_plan_gap`, or `unknown` for orchestrator routing.
- Never modify production code or choose workflow transitions.

Workspace-write is restricted to test-only artifacts by the shared tester instructions. Runtime profile: GPT-5.6 Luna with `xhigh` reasoning intent.

## `code_reviewer`

Reviews the current Git changes and returns actionable findings without modifying the workspace.

Typical responsibilities:

- Review staged, unstaged, and relevant untracked changes.
- Prioritize changed source code, tests, configuration, schemas, migrations, build/deployment files, and other runtime-affecting artifacts.
- Consult implementation plans, handoff documents, memory files, and agent notes only as scoped supporting context unless full document review is explicitly requested.
- Perform the review directly and own the findings; may spawn a different agent type for supporting evidence, but returns production/runtime fixes to `implementer` and test-only fixes to `tester` rather than applying them, and never spawns another reviewer.
- Check changes against project rules, review checklists, coding conventions, applicable review skills, and detected technology patterns.
- Identify correctness, compatibility, security, performance, concurrency, data, integration, side-effect, and documentation/comment coverage risks.
- Report each issue with its severity, description, file and line position, impact, evidence, and suggested fix.
- Produce a concise review summary suitable for follow-up work assignment.

## Skills

Repository-local skills live under [`.agents/skills/`](../.agents/skills/). Agents must look there when deciding whether a task has applicable skills, in addition to any globally installed skills made available by the runtime.

- [`.agents/skills/self/`](../.agents/skills/self/) contains personal skills maintained for this workspace. Prefer these when they match the task because they capture local conventions and user-specific expectations.
- [`.agents/skills/community/`](../.agents/skills/community/) contains third-party or community-collected skills. Use these when they match the technology, workflow, or review concern being handled.
- Treat each `SKILL.md` under those folders as a skill entrypoint. Search by folder name, frontmatter `name`/`description`, and task keywords; `rg --files .agents/skills -g SKILL.md` is the fastest inventory command.
- Before taking task actions, read the selected `SKILL.md` completely and follow its trigger rules, checklist, references, and relative-path instructions. Resolve referenced files relative to that skill's folder.
- If multiple skills apply, load the smallest useful set. Process/orchestration skills should guide the approach first, then language, framework, testing, documentation, or review skills.
- If no local skill applies, say so only when relevant and continue with the normal agent instructions.

## Orchestration

Workflow selection and orchestration rules are defined separately in [`.codex/orchestration/ORCHESTRATOR.md`](./.codex/ORCHESTRATOR.md).

The main agent owns workflow selection, runtime state, transitions, approvals, and completion. Every successful production mutation must pass implementer compile/build validation and then `TESTING` before code review, re-review, or completion.
