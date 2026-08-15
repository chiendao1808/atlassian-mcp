# Feature Development Workflow

- **Workflow name:** `wf_feature_development`
- **State model:** [`state-model.md`](./state-model.md)

## Purpose

Use this workflow to deliver a new feature, feature enhancement, or approved behavior-changing technical capability from clarified requirements through implementation and review.

The workflow supports an optional design phase. The orchestrator skips design only when the requirement does not affect user experience, interaction behavior, visual structure, or design-system contracts.

## Agents

- `req_analyzer`: normalizes the feature requirements, identifies missing information, and reviews supplied implementation plans for reuse.
- `explorer`: maps the relevant codebase, project constraints, and implementation surfaces.
- `uiux_designer`: produces or updates design artifacts when design is required.
- `planner`: creates a plan only when needed, or revises a reviewed supplied plan while preserving valid content; code-changing plans include a tester-consumable `verification_plan`.
- `implementer`: exclusively performs approved production code-related changes, required documentation/comments, and compile/build validation.
- `tester`: creates approved test-only artifacts, executes the approved verification plan, and returns result/cause/evidence.
- `code_reviewer`: reviews the resulting Git change set and returns actionable findings.

## Flow

### Requirement and design

```text
+-------+
| START |
+---+---+
    |
    v
+----------------------+
| REQUIREMENT_ANALYSIS |
+----------+-----------+
           |
           | requirements_ready
           v
+----------------------+
| CODEBASE_EXPLORATION |
+----------+-----------+
           |
      +----+------------------------+
      |                             |
      | design_required             | design_not_required
      v                             v
+-------------+<---------------+  +----------+
|   DESIGN    |                |  | PLANNING |
+------+------+                |  +----------+
       |                       |
       | design_ready          | changes_requested
       v                       |
+---------------+--------------+
| DESIGN_REVIEW |
+-------+-------+
        |
        | approved
        v
+----------+
| PLANNING |
+----------+
```


### Reusing a supplied implementation plan

```text
+----------------------+
| REQUIREMENT_ANALYSIS |
+----------+-----------+
           |
           | supplied plan reviewed
           v
+----------------------+
| CODEBASE_EXPLORATION |
+----------+-----------+
           |
           | provided_plan_ready
           v
+-------------+
| PLAN_REVIEW |
+-------------+
```

When design is required, `provided_plan_ready` may route from `DESIGN_REVIEW` to `PLAN_REVIEW` only after the approved design remains covered by the supplied plan. Otherwise `planner` revises the existing plan. For code-changing work, direct plan reuse also requires a tester-consumable verification scope with deterministic expected results.

### Planning and approval

```text
+----------+<---------------------------+
| PLANNING |                            |
+----+-----+                            |
     |                                  |
     | plan_ready                       | changes_requested
     v                                  |
+-------------+-------------------------+
| PLAN_REVIEW |
+------+------+ 
       |
  +----+------------------+
  |                       |
  | approved              | rejected
  v                       v
+----------------+   +-----------+
| IMPLEMENTATION |   | CANCELLED |
+----------------+   +-----+-----+
                           |
                           v
                       +---+---+
                       |  END  |
                       +-------+
```

### Implementation and testing

```text
+----------------+<---------------------------+
| IMPLEMENTATION |                            |
+-------+--------+                            |
        |                                     |
        | implementation_ready                | production_failure
        | compile_validation=passed           | source=IMPLEMENTATION
        v                                     |
+---------+-----------------------------------+
| TESTING |
+----+----+
     |
     | tests_passed
     v
+-------------+
| CODE_REVIEW |
+-------------+
```

Compile/build failure remains in `IMPLEMENTATION`; it does not advance to `TESTING`. Planned behavioral verification belongs to `tester`, not `implementer`.

Tester-owned and blocked failures route separately:

```text
+---------+<----------------------+
| TESTING |                       |
+----+----+                       |
     |                            | test_artifact_failure
     | testing_blocked            |
     v                            |
+---------+                       |
| BLOCKED |-----------------------+
+---------+  resume_testing after blocker resolution
```

`testing_blocked` covers environment/tooling failures, verification-plan gaps, and unknown failures that require evidence before choosing a mutation owner. A `production_failure` returns to `IMPLEMENTATION` only for `source=IMPLEMENTATION`; failures discovered after production or test-only remediation route to `REMEDIATION`.

### Review and remediation

```text
+---------+<----------------------------------+
| TESTING |                                   |
+---------+                                   | remediation_ready
                                              | compile_validation=passed
                                              |
+-------------+                               |
| CODE_REVIEW |                               |
+------+------+                               |
       |                                      |
  +----+-------------------------+            |
  |                              |            |
  | review_accepted              | findings_found
  |                              | (production/mixed)
  v                              v            |
+-----------+              +-------------+    |
| COMPLETED |              | REMEDIATION |----+
+-----+-----+              +-------------+
      |
      v
+-----+
| END |
+-----+
```

Test-only review findings stay tester-owned without introducing a separate review event name:

```text
+-------------+
| CODE_REVIEW |
+------+------+ 
       |
       | findings_found
       | (test-only)
       v
+---------------------------+
| TESTING                   |
| source=TEST_REMEDIATION   |
+-------------+-------------+
              |
              | tests_passed
              v
        +-------------+
        | CODE_REVIEW |
        +-------------+
```

### Clarification routing

```text
+----------------------+
| REQUIREMENT_ANALYSIS |
+----------+-----------+
           |
           | clarification_needed
           v
+------------------------+
| CLARIFICATION_REQUIRED |
+-----------+------------+
            |
       +----+-------------------+
       |                        |
       | clarification_received | cancelled
       v                        v
+----------------------+  +-----------+
| RESUME STATE ROUTING |  | CANCELLED |
+----+------------+----+  +-----+-----+
     |            |             |
     v            v             v
+----------------------+  +----------+  +-----+
| REQUIREMENT_ANALYSIS |  | PLANNING |  | END |
+----------------------+  +----------+  +-----+
```

### Blocked-state recovery

```text
+------------------+
| ANY ACTIVE STATE |
+--------+---------+
         |
         | blocked
         v
+---------+
| BLOCKED |
+----+----+
     |
     v
+----------------------+
| RESUME STATE ROUTING |
+--+---------+--------+-+
   |         |        |
   v         v        v
+----------------------+  +----------+  +----------------+
| REQUIREMENT_ANALYSIS |  | PLANNING |  | IMPLEMENTATION |
+----------------------+  +----------+  +----------------+
```

Testing-specific recovery adds:

```text
+---------+
| BLOCKED |
+----+----+
     |
     | resume_testing
     v
+---------+
| TESTING |
+---------+
```

### Blocked-state cancellation

```text
+---------+
| BLOCKED |
+----+----+
     |
     | cancelled
     v
+-----------+
| CANCELLED |
+-----+-----+
      |
      v
+-----+
| END |
+-----+
```

## Key Gates

- Requirements must be normalized before exploration.
- Design must be approved when the feature affects UI/UX or design-system behavior.
- A reviewed supplied plan must be reused when it remains valid; `planner` is used only for targeted revision or replacement supported by evidence. Code-changing plans must include an executable verification scope for `tester`.
- The implementation plan must receive explicit user approval.
- Production code-related writes must be performed by `implementer`; approved test-only writes and planned behavioral verification are owned by `tester`, never by the main agent.
- Every created or modified production logic unit must have documentation and intent-comment coverage reported by `implementer`.
- Implementation/remediation must pass compile/build validation before tester handoff, and mandatory `TESTING` must pass before code review/re-review/completion.
- Blocking code-review findings must be remediated, retested, and re-reviewed.

The Markdown state model (`state-model.md`) is authoritative for state definitions, metadata expectations, transition events, and guards.
