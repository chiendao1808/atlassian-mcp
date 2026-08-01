# Feature Development Workflow

- **Workflow name:** `wf_feature_development`
- **State model:** [`state-model.yaml`](./state-model.yaml)

## Purpose

Use this workflow to deliver a new feature, feature enhancement, or approved behavior-changing technical capability from clarified requirements through implementation and review.

The workflow supports an optional design phase. The orchestrator skips design only when the requirement does not affect user experience, interaction behavior, visual structure, or design-system contracts.

## Agents

- `req_analyzer`: normalizes the feature requirements, identifies missing information, and reviews supplied implementation plans for reuse.
- `explorer`: maps the relevant codebase, project constraints, and implementation surfaces.
- `designer`: produces or updates design artifacts when design is required.
- `planner`: creates a plan only when needed, or revises a reviewed supplied plan while preserving valid content.
- `implementer`: exclusively performs code generation, approved code-related changes, required documentation/comments, and self-verification.
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

When design is required, `provided_plan_ready` may route from `DESIGN_REVIEW` to `PLAN_REVIEW` only after the approved design remains covered by the supplied plan. Otherwise `planner` revises the existing plan.

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

### Implementation and self-verification

```text
+----------------+<---------------------------+
| IMPLEMENTATION |                            |
+-------+--------+                            |
        |                                     |
        | implementation_ready                | verification_failed
        v                                     |
+-------------------+-------------------------+
| SELF_VERIFICATION |
+---------+---------+
          |
          | verification_passed
          v
+-------------+
| CODE_REVIEW |
+-------------+
```

### Review and remediation

```text
+-------------------+<--------------------------+
| SELF_VERIFICATION |                           |
+-------------------+                           | remediation_ready
                                                |
+-------------+                                 |
| CODE_REVIEW |                                 |
+------+------+                                 |
       |                                        |
  +----+--------------------+                   |
  |                         |                   |
  | review_accepted         | findings_found    |
  v                         v                   |
+-----------+         +-------------+           |
| COMPLETED |         | REMEDIATION |-----------+
+-----+-----+         +-------------+
      |
      v
+-----+
| END |
+-----+
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
- A reviewed supplied plan must be reused when it remains valid; `planner` is used only for targeted revision or replacement supported by evidence.
- The implementation plan must receive explicit user approval.
- All code generation and code-related writes must be performed by `implementer`, never by the main agent.
- Every created or modified logic unit must have documentation and intent-comment coverage reported by `implementer`.
- Implementation must pass the implementer's diff, compile/build, and documentation/comment verification.
- Blocking code-review findings must be remediated and re-reviewed.

The YAML state model is authoritative for state definitions, metadata expectations, transition events, and guards.
