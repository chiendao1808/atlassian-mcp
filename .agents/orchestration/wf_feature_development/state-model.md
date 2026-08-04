# State Model — Feature Development

`name: wf_feature_development` · `schema_version: 1`

Deliver a clarified new feature or enhancement through exploration, optional design, approved planning, implementation, verification, and code review.

- **Initial state:** `REQUIREMENT_ANALYSIS`
- **Terminal states:** `COMPLETED`, `CANCELLED`

> Authoritative state model in Markdown so any tool or harness can follow it by reading. Each state lists its agent, expected outputs, and allowed events; the transition table below defines every legal move and its guard. The orchestrator (main agent) maintains the runtime state manually — there is no automatic engine, so treat this as the checklist of record.

## States

| State | Agent | Approval | Terminal |
|---|---|---|---|
| `REQUIREMENT_ANALYSIS` | `req_analyzer` | — | — |
| `CLARIFICATION_REQUIRED` | `main` | — | — |
| `CODEBASE_EXPLORATION` | `explorer` | — | — |
| `DESIGN` | `designer` | — | — |
| `DESIGN_REVIEW` | `main` | yes | — |
| `PLANNING` | `planner` | — | — |
| `PLAN_REVIEW` | `main` | yes | — |
| `IMPLEMENTATION` | `implementer` | — | — |
| `SELF_VERIFICATION` | `implementer` | — | — |
| `CODE_REVIEW` | `code_reviewer` | — | — |
| `REMEDIATION` | `implementer` | — | — |
| `BLOCKED` | `main` | — | — |
| `COMPLETED` | `main` | — | yes |
| `CANCELLED` | `main` | — | yes |

### `REQUIREMENT_ANALYSIS`

Normalize the feature goal and review any supplied implementation plan for reuse before new planning.

- **Agent:** `req_analyzer`
- **Approval required:** no
- **Expected outputs:** `requirement_brief`, `blocking_questions`, `feature_classification`, `supplied_plan_review`, `plan_reuse_recommendation`
- **Allowed events:** `requirements_ready`, `clarification_needed`, `blocked`, `cancelled`
- **Metadata · raw context:** `user_prompt`, `source_snapshots`, `agent_outputs`
- **Metadata · additional context:** `user_clarifications`, `constraints`, `workflow_selection`

### `CLARIFICATION_REQUIRED`

Obtain user answers for blocking requirement gaps.

- **Agent:** `main`
- **Approval required:** no
- **Expected outputs:** `user_clarifications`
- **Allowed events:** `clarification_received`, `cancelled`
- **Metadata · raw context:** `user_prompt`
- **Metadata · additional context:** `user_clarifications`, `notes`

### `CODEBASE_EXPLORATION`

Locate implementation surfaces, execution paths, dependencies, repository rules, and project skills relevant to the feature.

- **Agent:** `explorer`
- **Approval required:** no
- **Expected outputs:** `exploration_report`, `design_requirement_decision`, `unresolved_codebase_gaps`, `supplied_plan_validation`
- **Allowed events:** `design_required`, `design_not_required`, `blocked`, `provided_plan_ready`
- **Metadata · raw context:** `agent_outputs`
- **Metadata · additional context:** `repository_rules`, `project_skills`, `constraints`, `notes`

### `DESIGN`

Produce an implementation-ready design specification or approved design artifact for UI/UX and interaction changes.

- **Agent:** `designer`
- **Approval required:** no
- **Expected outputs:** `design_spec`, `design_assumptions`, `design_questions`
- **Allowed events:** `design_ready`, `blocked`
- **Metadata · raw context:** `source_snapshots`, `agent_outputs`
- **Metadata · additional context:** `repository_rules`, `project_skills`, `constraints`, `notes`

### `DESIGN_REVIEW`

Present the design for explicit user review and approval.

- **Agent:** `main`
- **Approval required:** yes
- **Expected outputs:** `design_approval_status`, `approved_design_scope`
- **Allowed events:** `approved`, `changes_requested`, `cancelled`, `provided_plan_ready`
- **Metadata · raw context:** `agent_outputs`
- **Metadata · additional context:** `user_clarifications`, `constraints`, `notes`

### `PLANNING`

Create a codebase-grounded implementation plan, or revise a reviewed supplied plan only where evidence requires changes.

- **Agent:** `planner`
- **Approval required:** no
- **Expected outputs:** `implementation_plan`, `plan_questions`, `requested_implementation_scope`
- **Allowed events:** `plan_ready`, `clarification_needed`, `blocked`
- **Metadata · raw context:** `agent_outputs`
- **Metadata · additional context:** `repository_rules`, `project_skills`, `constraints`, `notes`

### `PLAN_REVIEW`

Present the reused, revised, or newly created implementation plan and scope for explicit user approval.

- **Agent:** `main`
- **Approval required:** yes
- **Expected outputs:** `plan_approval_status`, `approved_scope`
- **Allowed events:** `approved`, `changes_requested`, `rejected`
- **Metadata · raw context:** `agent_outputs`
- **Metadata · additional context:** `user_clarifications`, `constraints`, `notes`

### `IMPLEMENTATION`

Implement only the approved plan and scope.

- **Agent:** `implementer`
- **Approval required:** no
- **Expected outputs:** `changed_files`, `implementation_report`, `documentation_updates`, `comment_coverage`
- **Allowed events:** `implementation_ready`, `blocked`
- **Metadata · raw context:** `agent_outputs`
- **Metadata · additional context:** `repository_rules`, `project_skills`, `constraints`, `notes`

### `SELF_VERIFICATION`

Review the Git diff and run the narrowest appropriate compile or build validation.

- **Agent:** `implementer`
- **Approval required:** no
- **Expected outputs:** `diff_summary`, `verification_commands`, `verification_result`, `documentation_comment_check`
- **Allowed events:** `verification_passed`, `verification_failed`, `blocked`
- **Metadata · raw context:** `agent_outputs`
- **Metadata · additional context:** `constraints`, `notes`

### `CODE_REVIEW`

Review the verified change set against repository rules, skills, contracts, side effects, and the detected stack.

- **Agent:** `code_reviewer`
- **Approval required:** no
- **Expected outputs:** `review_report`, `review_findings`, `review_summary`, `documentation_comment_findings`
- **Allowed events:** `review_accepted`, `findings_found`, `blocked`
- **Metadata · raw context:** `agent_outputs`
- **Metadata · additional context:** `repository_rules`, `project_skills`, `constraints`, `notes`

### `REMEDIATION`

Fix accepted review findings within the approved remediation scope.

- **Agent:** `implementer`
- **Approval required:** no
- **Expected outputs:** `remediated_findings`, `remediation_report`, `documentation_updates`, `comment_coverage`
- **Allowed events:** `remediation_ready`, `blocked`
- **Metadata · raw context:** `agent_outputs`
- **Metadata · additional context:** `constraints`, `notes`

### `BLOCKED`

Preserve context and resolve missing information, permission, tooling, or scope blockers.

- **Agent:** `main`
- **Approval required:** no
- **Expected outputs:** `blocker_resolution`, `resume_target`
- **Allowed events:** `resume_analysis`, `resume_planning`, `resume_implementation`, `cancelled`
- **Metadata · raw context:** `agent_outputs`
- **Metadata · additional context:** `user_clarifications`, `constraints`, `notes`

### `COMPLETED`

Record the final implementation, verification, review, residual risks, and artifact references.

- **Agent:** `main`
- **Approval required:** no
- **Terminal:** yes
- **Metadata · raw context:** `agent_outputs`
- **Metadata · additional context:** `constraints`, `notes`

### `CANCELLED`

Record cancellation or rejection without further execution.

- **Agent:** `main`
- **Approval required:** no
- **Terminal:** yes
- **Metadata · additional context:** `notes`

## Transitions

Every legal move. A transition may fire only when its guard holds; approval-gated targets also require the explicit user approval noted on the state above.

| From | Event | Guard | To |
|---|---|---|---|
| `REQUIREMENT_ANALYSIS` | `clarification_needed` | `blocking_questions_present` | `CLARIFICATION_REQUIRED` |
| `CLARIFICATION_REQUIRED` | `clarification_received` | `user_response_recorded_and_resume_state_is_requirement_analysis` | `REQUIREMENT_ANALYSIS` |
| `REQUIREMENT_ANALYSIS` | `requirements_ready` | `goal_scope_and_acceptance_criteria_sufficient` | `CODEBASE_EXPLORATION` |
| `CODEBASE_EXPLORATION` | `design_required` | `ui_ux_or_interaction_design_required` | `DESIGN` |
| `CODEBASE_EXPLORATION` | `design_not_required` | `implementation_surfaces_identified` | `PLANNING` |
| `CODEBASE_EXPLORATION` | `provided_plan_ready` | `supplied_plan_review_recommends_reuse_and_exploration_confirms_scope_and_design_not_required` | `PLAN_REVIEW` |
| `DESIGN` | `design_ready` | `design_spec_complete` | `DESIGN_REVIEW` |
| `DESIGN_REVIEW` | `changes_requested` | `user_requested_design_changes` | `DESIGN` |
| `DESIGN_REVIEW` | `approved` | `explicit_user_design_approval` | `PLANNING` |
| `DESIGN_REVIEW` | `provided_plan_ready` | `explicit_user_design_approval_and_supplied_plan_covers_approved_design` | `PLAN_REVIEW` |
| `PLANNING` | `plan_ready` | `no_blocking_plan_questions` | `PLAN_REVIEW` |
| `PLAN_REVIEW` | `changes_requested` | `user_requested_plan_changes` | `PLANNING` |
| `PLAN_REVIEW` | `approved` | `explicit_user_plan_approval_and_scope_recorded` | `IMPLEMENTATION` |
| `PLAN_REVIEW` | `rejected` | `user_rejected_plan` | `CANCELLED` |
| `IMPLEMENTATION` | `implementation_ready` | `approved_changes_applied_with_documentation_and_comment_coverage` | `SELF_VERIFICATION` |
| `SELF_VERIFICATION` | `verification_failed` | `failure_within_approved_scope` | `IMPLEMENTATION` |
| `SELF_VERIFICATION` | `verification_passed` | `diff_reviewed_compile_or_build_passed_and_documentation_comment_coverage_confirmed` | `CODE_REVIEW` |
| `CODE_REVIEW` | `findings_found` | `accepted_actionable_findings_present` | `REMEDIATION` |
| `CODE_REVIEW` | `review_accepted` | `no_blocking_findings_and_completion_artifacts_present` | `COMPLETED` |
| `REMEDIATION` | `remediation_ready` | `accepted_findings_addressed_with_documentation_and_comment_coverage` | `SELF_VERIFICATION` |
| `REQUIREMENT_ANALYSIS` | `blocked` | `blocker_recorded` | `BLOCKED` |
| `CODEBASE_EXPLORATION` | `blocked` | `blocker_recorded` | `BLOCKED` |
| `DESIGN` | `blocked` | `blocker_recorded` | `BLOCKED` |
| `PLANNING` | `blocked` | `blocker_recorded` | `BLOCKED` |
| `IMPLEMENTATION` | `blocked` | `blocker_recorded` | `BLOCKED` |
| `SELF_VERIFICATION` | `blocked` | `blocker_recorded` | `BLOCKED` |
| `CODE_REVIEW` | `blocked` | `blocker_recorded` | `BLOCKED` |
| `REMEDIATION` | `blocked` | `blocker_recorded` | `BLOCKED` |
| `BLOCKED` | `resume_analysis` | `analysis_blocker_resolved` | `REQUIREMENT_ANALYSIS` |
| `BLOCKED` | `resume_planning` | `planning_blocker_resolved` | `PLANNING` |
| `BLOCKED` | `resume_implementation` | `implementation_blocker_resolved_and_scope_approved` | `IMPLEMENTATION` |
| `CLARIFICATION_REQUIRED` | `cancelled` | `user_cancelled` | `CANCELLED` |
| `REQUIREMENT_ANALYSIS` | `cancelled` | `user_cancelled` | `CANCELLED` |
| `DESIGN_REVIEW` | `cancelled` | `user_cancelled` | `CANCELLED` |
| `PLANNING` | `clarification_needed` | `blocking_plan_questions_present_and_resume_state_recorded` | `CLARIFICATION_REQUIRED` |
| `CLARIFICATION_REQUIRED` | `clarification_received` | `user_response_recorded_and_resume_state_is_planning` | `PLANNING` |
| `BLOCKED` | `cancelled` | `user_cancelled` | `CANCELLED` |

## Runtime state template

The orchestrator instantiates and maintains this structure per workflow run (persist where needed, e.g. `.agents/orchestration/runtime/<workflow_id>.json`). Shown as JSON — a neutral data shape, not a machine-parsed spec:

```json
{
  "workflow_id": null,
  "workflow_name": "wf_feature_development",
  "status": "running",
  "current_state": "REQUIREMENT_ANALYSIS",
  "previous_state": null,
  "active_agent": "req_analyzer",
  "request": {
    "summary": null,
    "classification": "new_feature",
    "source_refs": [],
    "acceptance_criteria": []
  },
  "artifacts": {
    "requirement_brief": null,
    "supplied_implementation_plan": null,
    "plan_review_report": null,
    "exploration_report": null,
    "design_spec": null,
    "implementation_plan": null,
    "implementation_report": null,
    "documentation_updates": [],
    "comment_coverage": [],
    "review_report": null
  },
  "approval": {
    "required": false,
    "status": "not_required",
    "requested_scope": [],
    "approved_scope": []
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
