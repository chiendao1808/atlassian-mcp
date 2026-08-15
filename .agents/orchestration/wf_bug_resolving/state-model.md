# State Model — Bug Resolution

`name: wf_bug_resolving` · `schema_version: 2`

Resolve a verified defect or regression through bug clarification, impact exploration, evidence-based root-cause assessment, approved fixing, tester verification, and review.

- **Initial state:** `BUG_ANALYSIS`
- **Terminal states:** `COMPLETED`, `CANCELLED`

> Authoritative state model in Markdown so any tool or harness can follow it by reading. Each state lists its agent, expected outputs, and allowed events; the transition table below defines every legal move and its guard. The orchestrator (main agent) maintains the runtime state manually — there is no automatic engine, so treat this as the checklist of record.

## States

| State | Agent | Approval | Terminal |
|---|---|---|---|
| `BUG_ANALYSIS` | `req_analyzer` | — | — |
| `CLARIFICATION_REQUIRED` | `main` | — | — |
| `IMPACT_EXPLORATION` | `explorer` | — | — |
| `ROOT_CAUSE_ASSESSMENT` | `explorer` | — | — |
| `COMPLEXITY_ASSESSMENT` | `main` | — | — |
| `QUICK_FIX_PLANNING` | `planner` | — | — |
| `FIX_PLANNING` | `planner` | — | — |
| `PLAN_REVIEW` | `main` | yes | — |
| `IMPLEMENTATION` | `implementer` | — | — |
| `TESTING` | `tester` | — | — |
| `CODE_REVIEW` | `code_reviewer` | — | — |
| `REMEDIATION` | `implementer` | — | — |
| `BLOCKED` | `main` | — | — |
| `COMPLETED` | `main` | — | yes |
| `CANCELLED` | `main` | — | yes |

### `BUG_ANALYSIS`

Build a normalized bug brief and review any supplied fix or implementation plan for reuse.

- **Agent:** `req_analyzer`
- **Approval required:** no
- **Expected outputs:** `bug_brief`, `facts`, `clues`, `hypotheses`, `blocking_questions`, `planning_classification_hint`, `supplied_plan_review`, `plan_reuse_recommendation`
- **Allowed events:** `bug_brief_ready`, `quick_fix_candidate`, `clarification_needed`, `blocked`, `cancelled`
- **Metadata · raw context:** `user_prompt`, `source_snapshots`, `agent_outputs`
- **Metadata · additional context:** `user_clarifications`, `constraints`, `workflow_selection`

### `CLARIFICATION_REQUIRED`

Obtain missing reproduction, expected behavior, environment, scope, or access information.

- **Agent:** `main`
- **Approval required:** no
- **Expected outputs:** `user_clarifications`
- **Allowed events:** `clarification_received`, `cancelled`
- **Metadata · raw context:** `user_prompt`
- **Metadata · additional context:** `user_clarifications`, `notes`

### `IMPACT_EXPLORATION`

Trace affected code paths, data flows, contracts, dependencies, and blast radius using indexed tools when available.

- **Agent:** `explorer`
- **Approval required:** no
- **Expected outputs:** `exploration_report`, `affected_symbols`, `impact_map`, `evidence_gaps`
- **Allowed events:** `impact_mapped`, `blocked`
- **Metadata · raw context:** `agent_outputs`
- **Metadata · additional context:** `repository_rules`, `project_skills`, `constraints`, `notes`

### `ROOT_CAUSE_ASSESSMENT`

Evaluate root-cause hypotheses against codebase and bug evidence without modifying the workspace.

- **Agent:** `explorer`
- **Approval required:** no
- **Expected outputs:** `root_cause_report`, `supported_hypotheses`, `rejected_hypotheses`, `remaining_gaps`
- **Allowed events:** `evidence_sufficient`, `more_evidence_needed`, `user_evidence_needed`, `blocked`
- **Metadata · raw context:** `source_snapshots`, `agent_outputs`
- **Metadata · additional context:** `repository_rules`, `project_skills`, `user_clarifications`, `constraints`, `notes`

### `COMPLEXITY_ASSESSMENT`

Classify the verified bug by change complexity and operational risk, then select a lightweight or full planning path.

- **Agent:** `main`
- **Approval required:** no
- **Expected outputs:** `planning_complexity`, `risk_level`, `planning_mode`, `planning_rationale`, `escalation_triggers`, `execution_profile`, `supplied_plan_validation`
- **Allowed events:** `quick_fix_selected`, `full_planning_selected`, `clarification_needed`, `blocked`, `provided_plan_ready`
- **Metadata · raw context:** `source_snapshots`, `agent_outputs`
- **Metadata · additional context:** `repository_rules`, `project_skills`, `user_clarifications`, `constraints`, `notes`

### `QUICK_FIX_PLANNING`

Create or revise a concise, implementation-ready quick-fix brief for a deterministic, localized, low-risk bug without producing a full architecture plan.

- **Agent:** `planner`
- **Approval required:** no
- **Expected outputs:** `quick_fix_brief`, `verification_plan`, `requested_fix_scope`, `expected_code_change`, `validation_strategy`, `risk_check`
- **Allowed events:** `quick_plan_ready`, `escalate_to_full_planning`, `clarification_needed`, `blocked`
- **Metadata · raw context:** `agent_outputs`
- **Metadata · additional context:** `repository_rules`, `project_skills`, `constraints`, `notes`

### `FIX_PLANNING`

Create or revise the smallest safe fix plan grounded in the confirmed bug contract, impact map, and root-cause evidence.

- **Agent:** `planner`
- **Approval required:** no
- **Expected outputs:** `fix_plan`, `verification_plan`, `requested_fix_scope`, `validation_strategy`, `plan_questions`
- **Allowed events:** `plan_ready`, `clarification_needed`, `blocked`
- **Metadata · raw context:** `agent_outputs`
- **Metadata · additional context:** `repository_rules`, `project_skills`, `constraints`, `notes`

### `PLAN_REVIEW`

Present the reused, revised, or newly created fix plan, expected behavior, risks, validation, and scope for explicit user approval.

- **Agent:** `main`
- **Approval required:** yes
- **Expected outputs:** `plan_approval_status`, `approved_scope`, `approved_verification_scope`
- **Allowed events:** `approved`, `changes_requested`, `rejected`
- **Metadata · raw context:** `agent_outputs`
- **Metadata · additional context:** `user_clarifications`, `constraints`, `notes`

### `IMPLEMENTATION`

Implement the approved fix without expanding scope.

- **Agent:** `implementer`
- **Approval required:** no
- **Expected outputs:** `changed_files`, `implementation_report`, `documentation_updates`, `comment_coverage`, `compile_validation`
- **Allowed events:** `fix_ready`, `compile_failed`, `blocked`
- **Metadata · raw context:** `agent_outputs`
- **Metadata · additional context:** `repository_rules`, `project_skills`, `constraints`, `notes`

### `TESTING`

Create or update approved test-only artifacts, execute the approved regression/functional verification plan, and return evidence without modifying production code.

- **Agent:** `tester`
- **Approval required:** no
- **Expected outputs:** `test_report`, `test_artifacts`, `test_execution_summary`, `failure_classification`, `verification_coverage`
- **Allowed events:** `tests_passed`, `production_failure`, `test_artifact_failure`, `testing_blocked`
- `testing_source_state` is `IMPLEMENTATION`, `REMEDIATION`, or `TEST_REMEDIATION`.
- **Metadata · raw context:** `agent_outputs`
- **Metadata · additional context:** `repository_rules`, `project_skills`, `constraints`, `notes`

### `CODE_REVIEW`

Review the fix for correctness, regression risk, side effects, compatibility, and compliance with project rules.

- **Agent:** `code_reviewer`
- **Approval required:** no
- **Expected outputs:** `review_report`, `review_findings`, `review_summary`, `documentation_comment_findings`
- **Allowed events:** `review_accepted`, `findings_found`, `blocked`
- **Metadata · raw context:** `agent_outputs`
- **Metadata · additional context:** `repository_rules`, `project_skills`, `constraints`, `notes`

### `REMEDIATION`

Address accepted review findings within the approved remediation scope.

- **Agent:** `implementer`
- **Approval required:** no
- **Expected outputs:** `remediated_findings`, `remediation_report`, `documentation_updates`, `comment_coverage`, `compile_validation`
- **Allowed events:** `remediation_ready`, `compile_failed`, `blocked`
- **Metadata · raw context:** `agent_outputs`
- **Metadata · additional context:** `constraints`, `notes`

### `BLOCKED`

Resolve missing evidence, access, tooling, approval, or scope blockers while preserving current bug context.

- **Agent:** `main`
- **Approval required:** no
- **Expected outputs:** `blocker_resolution`, `resume_target`
- **Allowed events:** `resume_analysis`, `resume_exploration`, `resume_implementation`, `resume_complexity_assessment`, `resume_quick_planning`, `resume_full_planning`, `resume_testing`, `cancelled`
- **Metadata · raw context:** `agent_outputs`
- **Metadata · additional context:** `user_clarifications`, `constraints`, `notes`

### `COMPLETED`

Record the resolved behavior, changed scope, verification, review outcome, and residual risks. When production code changed, completion requires successful compile/build evidence and a passing mandatory tester report.

- **Agent:** `main`
- **Approval required:** no
- **Terminal:** yes
- **Metadata · raw context:** `agent_outputs`
- **Metadata · additional context:** `constraints`, `notes`

### `CANCELLED`

Record cancellation or rejection without further modification.

- **Agent:** `main`
- **Approval required:** no
- **Terminal:** yes
- **Metadata · additional context:** `notes`

## Transitions

Every legal move. A transition may fire only when its guard holds; approval-gated targets also require the explicit user approval noted on the state above.

| From | Event | Guard | To |
|---|---|---|---|
| `BUG_ANALYSIS` | `clarification_needed` | `blocking_bug_information_missing` | `CLARIFICATION_REQUIRED` |
| `CLARIFICATION_REQUIRED` | `clarification_received` | `user_response_recorded_and_resume_state_is_bug_analysis` | `BUG_ANALYSIS` |
| `BUG_ANALYSIS` | `bug_brief_ready` | `actual_and_expected_behavior_sufficiently_defined` | `IMPACT_EXPLORATION` |
| `BUG_ANALYSIS` | `quick_fix_candidate` | `deterministic_local_issue_with_verified_location_expected_behavior_and_change_condition` | `COMPLEXITY_ASSESSMENT` |
| `IMPACT_EXPLORATION` | `impact_mapped` | `affected_paths_and_dependencies_identified` | `ROOT_CAUSE_ASSESSMENT` |
| `ROOT_CAUSE_ASSESSMENT` | `more_evidence_needed` | `additional_codebase_evidence_can_resolve_gap` | `IMPACT_EXPLORATION` |
| `ROOT_CAUSE_ASSESSMENT` | `user_evidence_needed` | `missing_external_or_reproduction_evidence` | `CLARIFICATION_REQUIRED` |
| `ROOT_CAUSE_ASSESSMENT` | `evidence_sufficient` | `root_cause_or_fix_condition_supported_by_evidence` | `COMPLEXITY_ASSESSMENT` |
| `COMPLEXITY_ASSESSMENT` | `quick_fix_selected` | `all_quick_fix_eligibility_checks_pass_and_no_escalation_trigger_present` | `QUICK_FIX_PLANNING` |
| `COMPLEXITY_ASSESSMENT` | `full_planning_selected` | `complexity_or_risk_requires_full_planning` | `FIX_PLANNING` |
| `COMPLEXITY_ASSESSMENT` | `provided_plan_ready` | `supplied_plan_review_recommends_reuse_and_verified_scope_root_cause_risk_and_validation_are_covered_and_verification_scope_complete` | `PLAN_REVIEW` |
| `COMPLEXITY_ASSESSMENT` | `clarification_needed` | `planning_classification_depends_on_missing_information_and_resume_state_recorded` | `CLARIFICATION_REQUIRED` |
| `COMPLEXITY_ASSESSMENT` | `blocked` | `blocker_recorded` | `BLOCKED` |
| `QUICK_FIX_PLANNING` | `quick_plan_ready` | `concise_scope_change_and_validation_are_complete_and_no_escalation_trigger_present_and_verification_plan_complete` | `PLAN_REVIEW` |
| `QUICK_FIX_PLANNING` | `escalate_to_full_planning` | `new_cross_cutting_risk_or_scope_uncertainty_discovered` | `FIX_PLANNING` |
| `QUICK_FIX_PLANNING` | `clarification_needed` | `blocking_quick_plan_question_present_and_resume_state_recorded` | `CLARIFICATION_REQUIRED` |
| `QUICK_FIX_PLANNING` | `blocked` | `blocker_recorded` | `BLOCKED` |
| `FIX_PLANNING` | `plan_ready` | `no_blocking_plan_questions_and_verification_plan_complete` | `PLAN_REVIEW` |
| `PLAN_REVIEW` | `changes_requested` | `user_requested_plan_changes_and_planning_mode_is_quick_fix` | `QUICK_FIX_PLANNING` |
| `PLAN_REVIEW` | `changes_requested` | `user_requested_plan_changes_and_planning_mode_is_full` | `FIX_PLANNING` |
| `PLAN_REVIEW` | `approved` | `explicit_user_plan_approval_and_production_and_verification_scope_recorded` | `IMPLEMENTATION` |
| `PLAN_REVIEW` | `rejected` | `user_rejected_plan` | `CANCELLED` |
| `IMPLEMENTATION` | `compile_failed` | `compile_failure_within_approved_scope` | `IMPLEMENTATION` |
| `IMPLEMENTATION` | `fix_ready` | `approved_fix_applied_with_documentation_and_comment_coverage_and_compile_passed` | `TESTING` |
| `TESTING` | `tests_passed` | `mandatory_verification_complete_and_test_report_passed` | `CODE_REVIEW` |
| `TESTING` | `production_failure` | `testing_source_state_is_implementation` | `IMPLEMENTATION` |
| `TESTING` | `production_failure` | `testing_source_state_is_remediation` | `REMEDIATION` |
| `TESTING` | `production_failure` | `testing_source_state_is_test_remediation` | `REMEDIATION` |
| `TESTING` | `test_artifact_failure` | `approved_test_only_correction_scope_available` | `TESTING` |
| `TESTING` | `testing_blocked` | `testing_blocker_recorded` | `BLOCKED` |
| `CODE_REVIEW` | `findings_found` | `accepted_actionable_production_or_mixed_findings_present` | `REMEDIATION` |
| `CODE_REVIEW` | `findings_found` | `accepted_actionable_test_only_findings_present` | `TESTING` |
| `CODE_REVIEW` | `review_accepted` | `expected_behavior_restored_and_no_blocking_findings` | `COMPLETED` |
| `REMEDIATION` | `compile_failed` | `compile_failure_within_approved_scope` | `REMEDIATION` |
| `REMEDIATION` | `remediation_ready` | `accepted_findings_addressed_with_documentation_and_comment_coverage_and_compile_passed` | `TESTING` |
| `BUG_ANALYSIS` | `blocked` | `blocker_recorded` | `BLOCKED` |
| `IMPACT_EXPLORATION` | `blocked` | `blocker_recorded` | `BLOCKED` |
| `ROOT_CAUSE_ASSESSMENT` | `blocked` | `blocker_recorded` | `BLOCKED` |
| `FIX_PLANNING` | `blocked` | `blocker_recorded` | `BLOCKED` |
| `IMPLEMENTATION` | `blocked` | `blocker_recorded` | `BLOCKED` |
| `CODE_REVIEW` | `blocked` | `blocker_recorded` | `BLOCKED` |
| `REMEDIATION` | `blocked` | `blocker_recorded` | `BLOCKED` |
| `BLOCKED` | `resume_analysis` | `analysis_blocker_resolved` | `BUG_ANALYSIS` |
| `BLOCKED` | `resume_exploration` | `exploration_blocker_resolved` | `IMPACT_EXPLORATION` |
| `BLOCKED` | `resume_implementation` | `implementation_blocker_resolved_and_scope_approved` | `IMPLEMENTATION` |
| `BLOCKED` | `resume_testing` | `testing_blocker_resolved_and_verification_scope_authoritative` | `TESTING` |
| `CLARIFICATION_REQUIRED` | `cancelled` | `user_cancelled` | `CANCELLED` |
| `BUG_ANALYSIS` | `cancelled` | `user_cancelled` | `CANCELLED` |
| `FIX_PLANNING` | `clarification_needed` | `blocking_plan_questions_present_and_resume_state_recorded` | `CLARIFICATION_REQUIRED` |
| `CLARIFICATION_REQUIRED` | `clarification_received` | `user_response_recorded_and_resume_state_is_fix_planning` | `FIX_PLANNING` |
| `BLOCKED` | `cancelled` | `user_cancelled` | `CANCELLED` |
| `CLARIFICATION_REQUIRED` | `clarification_received` | `user_response_recorded_and_resume_state_is_complexity_assessment` | `COMPLEXITY_ASSESSMENT` |
| `CLARIFICATION_REQUIRED` | `clarification_received` | `user_response_recorded_and_resume_state_is_quick_fix_planning` | `QUICK_FIX_PLANNING` |
| `BLOCKED` | `resume_complexity_assessment` | `classification_blocker_resolved` | `COMPLEXITY_ASSESSMENT` |
| `BLOCKED` | `resume_quick_planning` | `quick_planning_blocker_resolved` | `QUICK_FIX_PLANNING` |
| `BLOCKED` | `resume_full_planning` | `full_planning_blocker_resolved` | `FIX_PLANNING` |

## Runtime state template

The orchestrator instantiates and maintains this structure per workflow run (persist where needed, e.g. `.agents/orchestration/runtime/<workflow_id>.json`). Shown as JSON — a neutral data shape, not a machine-parsed spec:

```json
{
  "workflow_id": null,
  "workflow_name": "wf_bug_resolving",
  "status": "running",
  "current_state": "BUG_ANALYSIS",
  "previous_state": null,
  "active_agent": "req_analyzer",
  "request": {
    "summary": null,
    "classification": "bug_fix",
    "source_refs": [],
    "bug_id": null,
    "priority": null
  },
  "bug": {
    "reproduction_steps": [],
    "actual_result": null,
    "expected_result": null,
    "environment": null,
    "facts": [],
    "clues": [],
    "hypotheses": [],
    "impact": []
  },
  "artifacts": {
    "bug_brief": null,
    "supplied_fix_plan": null,
    "plan_review_report": null,
    "exploration_report": null,
    "root_cause_report": null,
    "quick_fix_brief": null,
    "fix_plan": null,
    "verification_plan": null,
    "implementation_report": null,
    "documentation_updates": [],
    "comment_coverage": [],
    "compile_validation": null,
    "test_report": null,
    "test_artifacts": [],
    "review_report": null
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
  "history": [],
  "planning": {
    "complexity": "unknown",
    "risk_level": "unknown",
    "mode": "undecided",
    "rationale": [],
    "escalation_triggers": [],
    "execution_profile": {
      "preferred_model": null,
      "preferred_reasoning_effort": null,
      "selected_agent_type": null,
      "fallback_agent_type": "planner"
    }
  }
}
```
