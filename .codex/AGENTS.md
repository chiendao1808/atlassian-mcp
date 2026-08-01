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

## `designer`

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
- Present the plan for user review and approval.

## `implementer`

Exclusively handles code generation and approved code-related changes; the main agent must not implement or generate code directly.

Typical responsibilities:

- Generate requested code artifacts and modify production code, configuration, tests, documentation, comments, migrations, and related artifacts within the approved scope.
- Add or update developer documentation and intent comments for every created or modified logic unit, then report coverage for review.
- Follow project rules, coding conventions, applicable skills, and existing repository patterns.
- Review the resulting Git diff and run appropriate compile or build validation.
- Fix validation errors caused by its changes when they remain within the approved scope.
- Return a concise implementation and verification summary.

## `code_reviewer`

Reviews the current Git changes and returns actionable findings without modifying the workspace.

Typical responsibilities:

- Review staged, unstaged, and relevant untracked changes.
- Prioritize changed source code, tests, configuration, schemas, migrations, build/deployment files, and other runtime-affecting artifacts.
- Consult implementation plans, handoff documents, memory files, and agent notes only as scoped supporting context unless full document review is explicitly requested.
- Perform the review directly and must not spawn or delegate to any subagent.
- Check changes against project rules, review checklists, coding conventions, applicable review skills, and detected technology patterns.
- Identify correctness, compatibility, security, performance, concurrency, data, integration, side-effect, and documentation/comment coverage risks.
- Report each issue with its severity, description, file and line position, impact, evidence, and suggested fix.
- Produce a concise review summary suitable for follow-up work assignment.

## Workflow Instructions

Workflow selection and orchestration rules are defined separately in [`.codex/orchestration/ORCHESTRATOR.md`](./.codex/orchestration/ORCHESTRATOR.md).
