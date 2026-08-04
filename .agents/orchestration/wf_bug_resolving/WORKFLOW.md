# Bug Resolution Workflow

- **Workflow name:** `wf_bug_resolving`
- **State model:** [`state-model.md`](./state-model.md)

## Purpose

Use this workflow when existing behavior differs from an expected contract, including defects, regressions, incident-driven fixes, and small corrective updates.

The workflow separates verified bug facts from technical clues and root-cause hypotheses. It also selects between a lightweight quick-fix planning path and a full planning path according to evidence, change scope, and operational risk.

## Agents

- `req_analyzer`: extracts bug facts and reviews supplied fix or implementation plans for reuse.
- `explorer`: traces affected code paths, dependencies, and possible blast radius.
- `planner`: creates a plan only when needed, or revises a reviewed supplied plan using the selected complexity profile.
- `implementer`: exclusively applies the approved fix, adds required documentation/comments, and performs self-verification.
- `code_reviewer`: reviews the fix for regressions, side effects, and unresolved findings.

## Flow

### Analysis and complexity selection

```text
+-------+
| START |
+---+---+
    |
    v
+--------------+
| BUG_ANALYSIS |
+------+-------+
       |
  +----+------------------------------------------+
  |                                               |
  | bug_brief_ready                               | quick_fix_candidate
  v                                               |
+--------------------+                            |
| IMPACT_EXPLORATION |<----------------------+    |
+---------+----------+                       |    |
          |                                  |    |
          | impact_mapped                    | more_evidence_needed
          v                                  |    |
+-----------------------+--------------------+    |
| ROOT_CAUSE_ASSESSMENT |                         |
+-----------+-----------+                         |
            |                                     |
            | evidence_sufficient                 |
            +------------------+------------------+
                               |
                               v
                  +-----------------------+
                  | COMPLEXITY_ASSESSMENT |
                  +-----------------------+
```


### Reusing a supplied fix plan

```text
+--------------+
| BUG_ANALYSIS |
+------+-------+
       |
       | supplied plan reviewed
       v
+-----------------------+
| COMPLEXITY_ASSESSMENT |
+-----------+-----------+
            |
            | provided_plan_ready
            v
+-------------+
| PLAN_REVIEW |
+-------------+
```

The supplied plan may bypass both planning states only when verified bug scope, root cause, risk, and validation are covered. Otherwise `planner` revises the existing plan or replaces it with explicit justification.

### Planning mode selection

```text
+-----------------------+
| COMPLEXITY_ASSESSMENT |
+----------+------------+
           |
      +----+---------------------------+
      |                                |
      | quick_fix_selected             | full_planning_selected
      v                                v
+--------------------+          +----------------+
| QUICK_FIX_PLANNING |          |  FIX_PLANNING  |
+---------+----------+          +--------+-------+
          |                              |
          | quick_plan_ready             | plan_ready
          +---------------+--------------+
                          |
                          v
                    +-------------+
                    | PLAN_REVIEW |
                    +-------------+
```

### Quick-fix escalation

```text
+--------------------+
| QUICK_FIX_PLANNING |
+---------+----------+
          |
          | escalate_to_full_planning
          v
+----------------+
|  FIX_PLANNING  |
+----------------+
```

### Plan review and revision

```text
+-------------+
| PLAN_REVIEW |
+------+------+ 
       |
  +----+-------------------+
  |                        |
  | approved               | rejected
  v                        v
+----------------+    +-----------+
| IMPLEMENTATION |    | CANCELLED |
+----------------+    +-----+-----+
                            |
                            v
                        +---+---+
                        |  END  |
                        +-------+
```

```text
+-------------+
| PLAN_REVIEW |
+------+------+ 
       |
       | changes_requested
       v
+----------------------+
| PLANNING MODE ROUTER |
+----+------------+----+
     |            |
     v            v
+--------------------+  +----------------+
| QUICK_FIX_PLANNING |  |  FIX_PLANNING  |
+--------------------+  +----------------+
```

### Implementation and self-verification

```text
+----------------+<---------------------------+
| IMPLEMENTATION |                            |
+-------+--------+                            |
        |                                     |
        | fix_ready                           | verification_failed
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
+----------------------------+
| STATE REQUIRING USER INPUT |
+-------------+--------------+
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
+--+----------+------+--+  +-----+-----+
   |          |      |           |
   v          v      v           v
+--------------+  +-----------------------+  +--------------------+  +-----+
| BUG_ANALYSIS |  | COMPLEXITY_ASSESSMENT |  | QUICK_FIX_PLANNING |  | END |
+--------------+  +-----------------------+  +--------------------+  +-----+
```

```text
+----------------------+
| RESUME STATE ROUTING |
+----------+-----------+
           |
           v
+----------------+
|  FIX_PLANNING  |
+----------------+
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
+--+----+----+----+----+
   |    |    |    |
   v    v    v    v
+--------------+  +--------------------+  +-----------------------+
| BUG_ANALYSIS |  | IMPACT_EXPLORATION |  | COMPLEXITY_ASSESSMENT |
+--------------+  +--------------------+  +-----------------------+
```

```text
+----------------------+
| RESUME STATE ROUTING |
+--+---------+--------+-+
   |         |        |
   v         v        v
+--------------------+  +----------------+  +----------------+
| QUICK_FIX_PLANNING |  |  FIX_PLANNING  |  | IMPLEMENTATION |
+--------------------+  +----------------+  +----------------+
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

## Planning Complexity Guide

### Quick-fix planning

Select `QUICK_FIX_PLANNING` only when **all** of the following are true:

- The actual and expected behavior are explicit.
- The affected file, symbol, mapping, assignment, or configuration entry is known or directly verifiable.
- The change is localized and deterministic, normally limited to a small number of files and one bounded behavior.
- Validation is straightforward through diff review, compile/build, lint/static checks, or a focused behavior check.
- There is no material compatibility, security, data, concurrency, distributed-system, deployment, or migration risk.

Typical candidates include:

- Correcting syntax or an invalid expression.
- Fixing an incorrect constant, property assignment, field mapping, setter, condition, or returned value.
- Adding a non-breaking internal or optional field when its source, destination, and serialization behavior are already established.
- Applying a small configuration or metadata update with a known consumer and no infrastructure side effect.

The quick-fix brief should contain only:

1. Confirmed defect and expected behavior.
2. Exact change scope and likely files/symbols.
3. Minimal implementation steps.
4. Validation commands or checks.
5. A short risk and side-effect check.

Preferred execution profile:

```yaml
mode: lightweight
model: sonnet
reasoning_effort: medium
```

Use a matching configured worker or a supported per-dispatch model profile when available. If the runtime cannot select that profile, use the canonical `planner` but keep the output constrained to the quick-fix brief contract.

### Full planning

Select `FIX_PLANNING` when any of these conditions apply:

- Root cause, affected path, or expected behavior is still uncertain.
- The change crosses modules, services, repositories, or ownership boundaries.
- It affects a public API, external contract, schema, migration, stored data, or backward compatibility.
- It touches authentication, authorization, secrets, security controls, transactions, concurrency, retries, or idempotency.
- It changes Kafka or other messaging contracts, Redis/cache invalidation, distributed locks, scheduled jobs, infrastructure, deployment, or dependencies.
- The blast radius is broad, rollback is difficult, or validation requires multiple environments or coordinated consumers.

Use the canonical full planner profile for this path.

### Mandatory escalation

Immediately move from `QUICK_FIX_PLANNING` to `FIX_PLANNING` if exploration or planning reveals an escalation condition. Do not preserve the lightweight profile merely to reduce latency.

A simple planning path does **not** remove user approval, implementation self-verification, or code review.

## Key Gates

- The bug brief must distinguish facts, clues, hypotheses, and missing evidence.
- Root-cause assessment must have evidence tied to an execution path, contract, state transition, data condition, configuration condition, or deterministic local defect.
- Complexity and risk must be recorded before selecting a planning profile.
- A reviewed supplied fix plan must be reused when it remains valid; new planning is not performed merely to restate it.
- The quick-fix brief, reused plan, revised plan, or full fix plan and scope require explicit user approval.
- All code generation and code-related writes must be performed by `implementer`, never by the main agent.
- Every created or modified logic unit must have documentation and intent-comment coverage reported and verified before review acceptance.
- The fix must pass diff and compile/build or focused verification.
- Blocking review findings must be remediated and re-reviewed.

The Markdown state model (`state-model.md`) is authoritative for state definitions, metadata expectations, execution profiles, transition events, and guards.
