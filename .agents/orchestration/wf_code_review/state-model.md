# State Model — Code Review and Remediation

`name: wf_code_review` · `schema_version: 2`

Review an existing Git change set, triage findings, optionally remediate approved issues, verify remediation through tester, and re-review.

- **Initial state:** `REVIEW_INTAKE`
- **Terminal states:** `COMPLETED`, `DEFERRED`, `CANCELLED`

> Authoritative state model in Markdown so any tool or harness can follow it by reading. Each state lists its agent, expected outputs, and allowed events; the transition table below defines every legal move and its guard. The orchestrator (main agent) maintains the runtime state manually — there is no automatic engine, so treat this as the checklist of record.

## States

| State | Agent | Approval | Terminal |
|---|---|---|---|
| `REVIEW_INTAKE` | `req_analyzer` | — | — |
| `CLARIFICATION_REQUIRED` | `main` | — | — |
| `DIFF_REVIEW` | `code_reviewer` | — | — |
| `EVIDENCE_VERIFICATION` | `explorer` | — | — |
| `REVIEW_TRIAGE` | `main` | — | — |
| `REMEDIATION_PLANNING` | `planner` | — | — |
| `REMEDIATION_APPROVAL` | `main` | yes | — |
| `REMEDIATION` | `implementer` | — | — |
| `TESTING` | `tester` | — | — |
| `RE_REVIEW` | `code_reviewer` | — | — |
| `BLOCKED` | `main` | — | — |
| `COMPLETED` | `main` | — | yes |
| `DEFERRED` | `main` | — | yes |
| `CANCELLED` | `main` | — | yes |

### `REVIEW_INTAKE`

Clarify review scope and review any supplied remediation or implementation plan for reuse.

- **Agent:** `req_analyzer`
- **Approval required:** no
- **Expected outputs:** `review_brief`, `normalized_review_scope`, `blocking_questions`, `supplied_plan_review`, `plan_reuse_recommendation`
- **Allowed events:** `review_scope_ready`, `clarification_needed`, `blocked`, `cancelled`
- **Metadata · raw context:** `user_prompt`, `source_snapshots`, `agent_outputs`
- **Metadata · additional context:** `user_clarifications`, `constraints`, `workflow_selection`

### `CLARIFICATION_REQUIRED`

Obtain missing review scope, requirement, base revision, or expected behavior information.

- **Agent:** `main`
- **Approval required:** no
- **Expected outputs:** `user_clarifications`
- **Allowed events:** `clarification_received`, `cancelled`
- **Metadata · raw context:** `user_prompt`
- **Metadata · additional context:** `user_clarifications`, `notes`

### `DIFF_REVIEW`

Prioritize changed code and runtime-affecting artifacts, use plan or memory files only as scoped context, and return evidence-based actionable findings.

- **Agent:** `code_reviewer`
- **Approval required:** no
- **Expected outputs:** `review_report`, `review_findings`, `review_summary`, `primary_review_files`, `context_files_consulted`, `excluded_context_files`, `documentation_comment_findings`
- **Allowed events:** `review_accepted`, `findings_found`, `uncertain_findings`, `blocked`
- **Metadata · raw context:** `agent_outputs`
- **Metadata · additional context:** `repository_rules`, `project_skills`, `constraints`, `notes`

### `EVIDENCE_VERIFICATION`

Verify uncertain impact, side effects, callers, dependencies, contracts, or repository-specific behavior behind review findings.

- **Agent:** `explorer`
- **Approval required:** no
- **Expected outputs:** `evidence_report`, `confirmed_findings`, `rejected_findings`, `unresolved_findings`
- **Allowed events:** `evidence_ready`, `blocked`
- **Metadata · raw context:** `agent_outputs`
- **Metadata · additional context:** `repository_rules`, `project_skills`, `constraints`, `notes`

### `REVIEW_TRIAGE`

Decide which findings are accepted, rejected, deferred, or require further planning.

- **Agent:** `main`
- **Approval required:** no
- **Expected outputs:** `accepted_findings`, `deferred_findings`, `rejected_findings`, `finding_ownership`, `remediation_complexity`, `supplied_plan_validation`
- **Allowed events:** `all_findings_deferred`, `complex_remediation`, `direct_remediation`, `cancelled`, `provided_remediation_plan_ready`
- **Metadata · raw context:** `agent_outputs`
- **Metadata · additional context:** `user_clarifications`, `constraints`, `notes`

### `REMEDIATION_PLANNING`

Create a remediation plan, or revise a reviewed supplied plan, for complex, cross-cutting, or scope-sensitive accepted findings.

- **Agent:** `planner`
- **Approval required:** no
- **Expected outputs:** `remediation_plan`, `verification_plan`, `requested_remediation_scope`, `plan_questions`
- **Allowed events:** `plan_ready`, `clarification_needed`, `blocked`
- **Metadata · raw context:** `agent_outputs`
- **Metadata · additional context:** `repository_rules`, `project_skills`, `constraints`, `notes`

### `REMEDIATION_APPROVAL`

Present accepted findings and the reused, revised, or new remediation scope for explicit user approval.

- **Agent:** `main`
- **Approval required:** yes
- **Expected outputs:** `remediation_approval_status`, `approved_scope`, `approved_verification_scope`
- **Allowed events:** `approved`, `changes_requested`, `rejected_or_deferred`
- **Metadata · raw context:** `agent_outputs`
- **Metadata · additional context:** `user_clarifications`, `constraints`, `notes`

### `REMEDIATION`

Fix only accepted production/runtime findings within the approved remediation scope.

- **Agent:** `implementer`
- **Approval required:** no
- **Expected outputs:** `remediated_findings`, `changed_files`, `remediation_report`, `documentation_updates`, `comment_coverage`, `compile_validation`
- **Allowed events:** `remediation_ready`, `compile_failed`, `blocked`
- **Metadata · raw context:** `agent_outputs`
- **Metadata · additional context:** `repository_rules`, `project_skills`, `constraints`, `notes`

### `TESTING`

Apply approved test-only remediation when applicable, execute the approved verification plan, and return evidence without modifying production code.

- **Agent:** `tester`
- **Approval required:** no
- **Expected outputs:** `test_report`, `test_artifacts`, `test_execution_summary`, `failure_classification`, `verification_coverage`
- **Allowed events:** `tests_passed`, `production_failure`, `test_artifact_failure`, `testing_blocked`
- `testing_source_state` is `REMEDIATION` or `TEST_REMEDIATION`.
- **Metadata · raw context:** `agent_outputs`
- **Metadata · additional context:** `repository_rules`, `project_skills`, `constraints`, `notes`

### `RE_REVIEW`

Re-review remediated code and runtime-affecting artifacts first, consulting plan or memory files only when directly relevant to accepted findings.

- **Agent:** `code_reviewer`
- **Approval required:** no
- **Expected outputs:** `re_review_report`, `resolved_findings`, `remaining_findings`, `primary_review_files`, `context_files_consulted`, `excluded_context_files`, `documentation_comment_findings`
- **Allowed events:** `review_accepted`, `findings_remain`, `blocked`
- **Metadata · raw context:** `agent_outputs`
- **Metadata · additional context:** `repository_rules`, `project_skills`, `constraints`, `notes`

### `BLOCKED`

Resolve missing scope, evidence, tooling, approval, or write-boundary blockers.

- **Agent:** `main`
- **Approval required:** no
- **Expected outputs:** `blocker_resolution`, `resume_target`
- **Allowed events:** `resume_intake`, `resume_review`, `resume_remediation`, `resume_testing`, `cancelled`
- **Metadata · raw context:** `agent_outputs`
- **Metadata · additional context:** `user_clarifications`, `constraints`, `notes`

### `COMPLETED`

Record the final review outcome, resolved findings, verification, and remaining risks. When remediation changed production or test artifacts, completion requires the applicable compile/build evidence and a passing mandatory tester report.

- **Agent:** `main`
- **Approval required:** no
- **Terminal:** yes
- **Metadata · raw context:** `agent_outputs`
- **Metadata · additional context:** `constraints`, `notes`

### `DEFERRED`

Record intentionally deferred findings, rationale, ownership, and follow-up references.

- **Agent:** `main`
- **Approval required:** no
- **Terminal:** yes
- **Metadata · raw context:** `agent_outputs`
- **Metadata · additional context:** `constraints`, `notes`

### `CANCELLED`

Record cancellation without remediation.

- **Agent:** `main`
- **Approval required:** no
- **Terminal:** yes
- **Metadata · additional context:** `notes`

## Transitions

Every legal move. A transition may fire only when its guard holds; approval-gated targets also require the explicit user approval noted on the state above.

| From | Event | Guard | To |
|---|---|---|---|
| `REVIEW_INTAKE` | `clarification_needed` | `review_scope_or_requirements_missing` | `CLARIFICATION_REQUIRED` |
| `CLARIFICATION_REQUIRED` | `clarification_received` | `user_response_recorded_and_resume_state_is_review_intake` | `REVIEW_INTAKE` |
| `REVIEW_INTAKE` | `review_scope_ready` | `review_target_and_scope_defined` | `DIFF_REVIEW` |
| `DIFF_REVIEW` | `uncertain_findings` | `material_findings_require_additional_codebase_evidence` | `EVIDENCE_VERIFICATION` |
| `EVIDENCE_VERIFICATION` | `evidence_ready` | `finding_evidence_updated` | `DIFF_REVIEW` |
| `DIFF_REVIEW` | `findings_found` | `actionable_findings_present` | `REVIEW_TRIAGE` |
| `DIFF_REVIEW` | `review_accepted` | `no_actionable_findings_and_review_summary_present` | `COMPLETED` |
| `REVIEW_TRIAGE` | `all_findings_deferred` | `no_accepted_findings` | `DEFERRED` |
| `REVIEW_TRIAGE` | `complex_remediation` | `accepted_findings_require_plan` | `REMEDIATION_PLANNING` |
| `REVIEW_TRIAGE` | `direct_remediation` | `accepted_findings_are_scoped_and_actionable` | `REMEDIATION_APPROVAL` |
| `REVIEW_TRIAGE` | `provided_remediation_plan_ready` | `accepted_findings_are_covered_by_reviewed_supplied_remediation_plan_and_verification_scope_complete` | `REMEDIATION_APPROVAL` |
| `REMEDIATION_PLANNING` | `plan_ready` | `no_blocking_plan_questions_and_verification_plan_complete` | `REMEDIATION_APPROVAL` |
| `REMEDIATION_APPROVAL` | `changes_requested` | `user_requested_plan_or_scope_changes` | `REMEDIATION_PLANNING` |
| `REMEDIATION_APPROVAL` | `approved` | `explicit_user_approval_and_scope_recorded_and_production_or_mixed_findings_approved` | `REMEDIATION` |
| `REMEDIATION_APPROVAL` | `approved` | `explicit_user_approval_and_scope_recorded_and_test_only_findings_approved` | `TESTING` |
| `REMEDIATION_APPROVAL` | `rejected_or_deferred` | `user_rejected_or_deferred_remediation` | `DEFERRED` |
| `REMEDIATION` | `compile_failed` | `compile_failure_within_approved_scope` | `REMEDIATION` |
| `REMEDIATION` | `remediation_ready` | `approved_remediation_applied_with_documentation_and_comment_coverage_and_compile_passed` | `TESTING` |
| `TESTING` | `tests_passed` | `mandatory_verification_complete_and_test_report_passed` | `RE_REVIEW` |
| `TESTING` | `production_failure` | `production_behavior_failure_after_remediation` | `REMEDIATION` |
| `TESTING` | `test_artifact_failure` | `approved_test_only_correction_scope_available` | `TESTING` |
| `TESTING` | `testing_blocked` | `testing_blocker_recorded` | `BLOCKED` |
| `RE_REVIEW` | `findings_remain` | `accepted_or_new_blocking_production_or_mixed_findings_present_and_scope_valid` | `REMEDIATION` |
| `RE_REVIEW` | `findings_remain` | `accepted_or_new_blocking_test_only_findings_present_and_scope_valid` | `TESTING` |
| `RE_REVIEW` | `review_accepted` | `accepted_findings_resolved_and_no_blocking_regressions` | `COMPLETED` |
| `REVIEW_INTAKE` | `blocked` | `blocker_recorded` | `BLOCKED` |
| `DIFF_REVIEW` | `blocked` | `blocker_recorded` | `BLOCKED` |
| `EVIDENCE_VERIFICATION` | `blocked` | `blocker_recorded` | `BLOCKED` |
| `REMEDIATION_PLANNING` | `blocked` | `blocker_recorded` | `BLOCKED` |
| `REMEDIATION` | `blocked` | `blocker_recorded` | `BLOCKED` |
| `RE_REVIEW` | `blocked` | `blocker_recorded` | `BLOCKED` |
| `BLOCKED` | `resume_intake` | `intake_blocker_resolved` | `REVIEW_INTAKE` |
| `BLOCKED` | `resume_review` | `review_blocker_resolved` | `DIFF_REVIEW` |
| `BLOCKED` | `resume_remediation` | `remediation_blocker_resolved_and_scope_approved` | `REMEDIATION` |
| `BLOCKED` | `resume_testing` | `testing_blocker_resolved_and_verification_scope_authoritative` | `TESTING` |
| `CLARIFICATION_REQUIRED` | `cancelled` | `user_cancelled` | `CANCELLED` |
| `REVIEW_INTAKE` | `cancelled` | `user_cancelled` | `CANCELLED` |
| `REVIEW_TRIAGE` | `cancelled` | `user_cancelled` | `CANCELLED` |
| `REMEDIATION_PLANNING` | `clarification_needed` | `blocking_plan_questions_present_and_resume_state_recorded` | `CLARIFICATION_REQUIRED` |
| `CLARIFICATION_REQUIRED` | `clarification_received` | `user_response_recorded_and_resume_state_is_remediation_planning` | `REMEDIATION_PLANNING` |
| `BLOCKED` | `cancelled` | `user_cancelled` | `CANCELLED` |

## Runtime state template

The orchestrator instantiates and maintains this structure per workflow run (persist where needed, e.g. `.agents/orchestration/runtime/<workflow_id>.json`). Shown as JSON — a neutral data shape, not a machine-parsed spec:

```json
{
  "workflow_id": null,
  "workflow_name": "wf_code_review",
  "status": "running",
  "current_state": "REVIEW_INTAKE",
  "previous_state": null,
  "active_agent": "req_analyzer",
  "request": {
    "summary": null,
    "classification": "code_review",
    "source_refs": [],
    "review_scope": null
  },
  "review": {
    "reviewed_files": [],
    "primary_review_files": [],
    "context_files_consulted": [],
    "excluded_context_files": [],
    "findings": [],
    "accepted_findings": [],
    "finding_ownership": {},
    "deferred_findings": [],
    "rejected_findings": []
  },
  "artifacts": {
    "review_brief": null,
    "supplied_remediation_plan": null,
    "plan_review_report": null,
    "review_report": null,
    "evidence_report": null,
    "remediation_plan": null,
    "verification_plan": null,
    "remediation_report": null,
    "documentation_updates": [],
    "comment_coverage": [],
    "compile_validation": null,
    "test_report": null,
    "test_artifacts": [],
    "re_review_report": null
  },
  "approval": {
    "required": false,
    "status": "not_required",
    "requested_scope": [],
    "approved_scope": [],
    "approved_verification_scope": []
  },
  "testing": {
    "source_state": null,
    "cycle": 0,
    "max_cycles": 3,
    "last_result": null,
    "last_failure_classification": null
  },
  "execution": {
    "retry_count": 0,
    "review_cycle": 0,
    "max_review_cycles": 3,
    "blocker": null
  },
  "metadata": {
    "raw_context": {
      "user_prompt": null,
      "source_snapshots": [],
      "agent_outputs": []
    },
    "additional_context": {
      "user_clarifications": [],
      "repository_rules": [],
      "project_skills": [],
      "constraints": [],
      "notes": [],
      "workflow_selection": {}
    },
    "tags": [],
    "correlation": {},
    "updated_at": null
  },
  "history": []
}
```
