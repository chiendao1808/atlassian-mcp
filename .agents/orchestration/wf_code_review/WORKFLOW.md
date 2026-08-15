# Code Review and Remediation Workflow

- **Workflow name:** `wf_code_review`
- **State model:** [`state-model.md`](./state-model.md)

## Purpose

Use this workflow when reviewing an existing Git change set is the primary objective. It supports review-only completion, issue triage, approved remediation, tester verification, and re-review.

This workflow is also suitable for pre-merge checks, independent post-implementation review, and remediation of findings produced by a previous review cycle.

## Agents

- `req_analyzer`: clarifies the review target when scope, requirements, or source links are incomplete.
- `code_reviewer`: inspects the change set directly in its assigned thread, may spawn a different-type helper (e.g. `explorer`) for evidence but never another reviewer, prioritizes changed code and runtime-affecting artifacts, and uses plan or memory files only as scoped supporting context.
- `explorer`: verifies uncertain impact or side-effect findings when additional codebase evidence is required.
- `planner`: creates a remediation plan only when needed, or revises a reviewed supplied plan for complex findings; code/test-changing remediation plans include a tester-consumable `verification_plan`.
- `implementer`: exclusively applies approved production remediation, adds required documentation/comments, and performs compile/build validation.
- `tester`: applies approved test-only remediation where applicable, creates approved test artifacts, executes the verification plan, and returns result/cause/evidence.

## Review Scope Priority

- Primary review targets are changed source code, tests, runtime configuration, schemas, migrations, contracts, build/deployment files, infrastructure-as-code, generated manifests, and documentation/comments tied to changed logic.
- Implementation plans, remediation plans, session handoffs, memory-bank files, agent notes, status documents, and similar context artifacts are secondary. Read them only enough to extract applicable constraints, intended behavior, approval scope, or direct contradictions with primary changes.
- Do not spend the main review budget on editorial or completeness review of secondary artifacts unless the user explicitly requests document review.
- If only secondary artifacts changed, report that no code or runtime-affecting artifacts were available and perform a bounded consistency and safety check rather than a full code review.
- Review output must distinguish primary artifacts reviewed, secondary context consulted, and files intentionally excluded from deep review.

## Flow

### Review intake

```text
+-------+
| START |
+---+---+
    |
    v
+---------------+
| REVIEW_INTAKE |
+-------+-------+
        |
        | review_scope_ready
        v
+-------------+
| DIFF_REVIEW |
+-------------+
```

### Evidence verification loop

```text
+-------------+
| DIFF_REVIEW |
+------+------+ 
       |
       | uncertain_findings
       v
+-----------------------+
| EVIDENCE_VERIFICATION |
+-----------+-----------+
            |
            | evidence_ready
            v
+-------------+
| DIFF_REVIEW |
+-------------+
```

### Review outcome routing

```text
+-------------+
| DIFF_REVIEW |
+------+------+ 
       |
  +----+-------------------+
  |                        |
  | review_accepted        | findings_found
  v                        v
+-----------+        +---------------+
| COMPLETED |        | REVIEW_TRIAGE |
+-----+-----+        +---------------+
      |
      v
+-----+
| END |
+-----+
```

### Triage routing

```text
+---------------+
| REVIEW_TRIAGE |
+-------+-------+
        |
   +----+-------------------------+
   |                              |
   | all_findings_deferred        | complex_remediation
   v                              v
+----------+              +----------------------+
| DEFERRED |              | REMEDIATION_PLANNING |
+----+-----+              +----------+-----------+
     |                                |
     |                                | plan_ready
     v                                v
+----+----+                   +----------------------+
|   END   |                   | REMEDIATION_APPROVAL |
+---------+                   +----------------------+
```

Direct remediation keeps the existing `direct_remediation` event; the guard uses triaged finding ownership to determine the approved mutation scope:

```text
+---------------+
| REVIEW_TRIAGE |
+-------+-------+
        |
        | direct_remediation
        v
+----------------------+
| REMEDIATION_APPROVAL |
+----------------------+
```

### Remediation approval

The existing `approved` event is reused; its guard determines the next write-owning agent.

```text
                         +----------------------+
                         | REMEDIATION_APPROVAL |
                         +----------+-----------+
                                    |
                 +------------------+------------------+
                 |                                     |
                 | approved                            | approved
                 | production/mixed                    | test-only
                 v                                     v
           +-------------+                       +---------+
           | REMEDIATION |                       | TESTING |
           +-------------+                       +---------+

                         +----------------------+
                         | REMEDIATION_APPROVAL |
                         +----------+-----------+
                                    |
                                    | rejected_or_deferred
                                    v
                               +----------+
                               | DEFERRED |
                               +----+-----+
                                    |
                                    v
                                +---+---+
                                |  END  |
                                +-------+
```

```text
+----------------------+
| REMEDIATION_APPROVAL |
+----------+-----------+
           |
           | changes_requested
           v
+----------------------+
| REMEDIATION_PLANNING |
+----------------------+
```

### Remediation and testing

```text
+-------------+<---------------------------+
| REMEDIATION |                            |
+------+------+                            |
       |                                   |
       | remediation_ready                 | production_failure
       | compile_validation=passed         |
       v                                   |
+---------+--------------------------------+
| TESTING |
+----+----+
     |
     | tests_passed
     v
+-----------+
| RE_REVIEW |
+-----------+
```

Compile/build failure remains in `REMEDIATION`; it does not advance to `TESTING`. Test-only remediation enters `TESTING` directly after approval with `source=TEST_REMEDIATION` and remains tester-owned.

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

`testing_blocked` covers environment/tooling failures, verification-plan gaps, and unknown failures that require evidence before selecting a mutation owner.

### Re-review outcome

The existing `findings_remain` event is reused; the guard routes by ownership.

```text
+-----------+
| RE_REVIEW |
+-----+-----+
      |
 +----+-------------------------------+------------------------------+
 |                                    |                              |
 | review_accepted                    | findings_remain              | findings_remain
 |                                    | production/mixed             | test-only
 v                                    v                              v
+-----------+                   +-------------+                  +---------+
| COMPLETED |                   | REMEDIATION |                  | TESTING |
+-----+-----+                   +-------------+                  +---------+
      |
      v
+-----+
| END |
+-----+
```

Production/mixed findings return through `REMEDIATION -> TESTING -> RE_REVIEW`; test-only findings return through `TESTING -> RE_REVIEW`.

### Clarification routing

```text
+---------------+
| REVIEW_INTAKE |
+-------+-------+
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
+---------------+        +-----------+
| REVIEW_INTAKE |        | CANCELLED |
+---------------+        +-----+-----+
                               |
                               v
                           +---+---+
                           |  END  |
                           +-------+
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
+---------------+  +-------------+  +-------------+
| REVIEW_INTAKE |  | DIFF_REVIEW |  | REMEDIATION |
+---------------+  +-------------+  +-------------+
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


### Reusing a supplied remediation plan

```text
+---------------+
| REVIEW_INTAKE |
+-------+-------+
        |
        | supplied plan reviewed
        v
+---------------+
| REVIEW_TRIAGE |
+-------+-------+
        |
        | provided_remediation_plan_ready
        v
+----------------------+
| REMEDIATION_APPROVAL |
+----------------------+
```

Direct reuse is allowed only when the reviewed supplied plan covers all accepted findings and current diff evidence. When remediation mutates production or test artifacts, the plan must also provide sufficient verification scope for `tester`. Otherwise `planner` revises the existing plan.

## Key Gates

- Review scope must be explicit: workspace diff, staged diff, commit range, branch comparison, or supplied files. Within that scope, changed code and runtime-affecting artifacts take priority over plan and memory artifacts.
- Findings must contain evidence, severity, position, impact or side effect, and a fix suggestion when possible.
- Uncertain findings require evidence verification before remediation.
- A reviewed supplied remediation plan must be reused when it covers the accepted findings; new planning is only for targeted revision or justified replacement. Mutating remediation must include an executable verification scope for `tester`.
- Remediation requires explicit user approval and a recorded scope.
- Production code-related remediation must be performed by `implementer`; approved test-only remediation and planned behavioral verification are owned by `tester`, never by the main agent.
- Every created or modified production logic unit must have documentation and intent-comment coverage.
- Production remediation must pass compile/build validation before tester handoff; all approved remediation must pass mandatory `TESTING` before re-review/completion.

The Markdown state model (`state-model.md`) is authoritative for state definitions, metadata expectations, transition events, and guards.
