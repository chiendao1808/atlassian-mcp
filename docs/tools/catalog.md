# Tool Catalog

This catalog lists the tools currently registered by `atlassian-mcp`. Jira and Confluence tools require their authenticate tool first unless startup environment authentication succeeds; Bitbucket tools use the configured Bitbucket bearer token.

## Jira

| Tool | Function |
| --- | --- |
| `jira_authenticate` | Authenticate this MCP process session to Jira. |
| `jira_get_issue` | Read one Jira issue. |
| `jira_add_issue_comment` | Add a comment to one Jira issue. |
| `jira_update_issue_fields` | Update Jira issue fields using native Jira `fields`/`update` JSON. |
| `jira_transition_issue` | Transition an issue by transition ID or exact transition name. |
| `jira_create_issue` | Create one Jira issue from native Jira fields/update JSON. |
| `jira_bulk_create_issues` | Create multiple Jira issues in one call and return per-row results. |
| `jira_delete_issue` | Permanently delete one Jira issue, optionally including subtasks. |
| `jira_assign_issue` | Assign or unassign one Jira issue. |
| `jira_search_issues` | Search Jira issues by JQL. |
| `jira_list_issue_comments` | List comments for one Jira issue. |
| `jira_update_issue_comment` | Update one Jira issue comment. |
| `jira_delete_issue_comment` | Permanently delete one Jira issue comment. |
| `jira_list_issue_transitions` | List available workflow transitions for one Jira issue. |
| `jira_add_issue_attachment` | Upload one base64-encoded attachment to a Jira issue. |
| `jira_delete_issue_attachment` | Permanently delete one Jira attachment by attachment ID. |
| `jira_list_issue_worklogs` | List worklog entries for one Jira issue. |
| `jira_add_issue_worklog` | Add a worklog entry to one Jira issue. |
| `jira_get_issue_watchers` | List watchers for one Jira issue. |
| `jira_add_issue_watcher` | Add one watcher to a Jira issue. |
| `jira_remove_issue_watcher` | Remove one watcher from a Jira issue. |
| `jira_vote_issue` | Add the authenticated user's vote to a Jira issue. |
| `jira_unvote_issue` | Remove the authenticated user's vote from a Jira issue. |
| `jira_create_issue_link` | Create a native Jira issue link between two issues. |
| `jira_create_component` | Create one Jira project Component. |
| `jira_get_component` | Read one Jira Component by ID. |
| `jira_update_component` | Partially update one Jira Component. |
| `jira_delete_component` | Delete one Jira Component, optionally moving affected issues. |
| `jira_get_component_issue_count` | Read Jira's related issue count for one Component. |
| `jira_list_project_components` | List Components for one Jira project ID or key. |

## Confluence

| Tool | Function |
| --- | --- |
| `confluence_authenticate` | Authenticate this MCP process session to Confluence. |
| `confluence_search_content` | Search Confluence content with raw CQL. |
| `confluence_get_content` | Read one Confluence content item by ID. |
| `confluence_list_content` | List Confluence content with documented filters. |
| `confluence_list_content_properties` | List native content properties for one content item. |
| `confluence_get_content_property` | Read one native content property by key. |
| `confluence_list_spaces` | List visible Confluence spaces with filters. |
| `confluence_get_space` | Read one Confluence space by key. |
| `confluence_list_space_content` | List grouped content for one Confluence space. |

## Bitbucket

| Tool | Function |
| --- | --- |
| `bitbucket_get_repository` | Read Bitbucket repository metadata. |
| `bitbucket_list_branches` | List Bitbucket branches with optional filters and paging. |
| `bitbucket_get_default_branch` | Read the default branch for one repository. |
| `bitbucket_create_branch` | Create one Bitbucket branch. |
| `bitbucket_get_file` | Read one Bitbucket file as text or base64. |
| `bitbucket_list_commits` | List Bitbucket commits with optional filters and paging. |
| `bitbucket_get_commit` | Read one Bitbucket commit. |
| `bitbucket_get_commit_changes` | List changed paths for one commit. |
| `bitbucket_get_commit_diff` | Read structured diff for one commit. |
| `bitbucket_compare_commits` | Compare commits between refs. |
| `bitbucket_compare_changes` | Compare changed paths between refs. |
| `bitbucket_compare_diff` | Compare structured diff between refs. |
| `bitbucket_commit_file` | Create or update one file with one Bitbucket commit. |
| `bitbucket_list_pull_requests` | List Bitbucket pull requests. |
| `bitbucket_get_pull_request` | Read one Bitbucket pull request. |
| `bitbucket_get_pull_request_activities` | List pull request activities. |
| `bitbucket_get_pull_request_commits` | List commits on one pull request. |
| `bitbucket_get_pull_request_changes` | List changed paths on one pull request. |
| `bitbucket_get_pull_request_diff` | Read diff for one pull request. |
| `bitbucket_check_pull_request_mergeability` | Check whether one pull request can be merged. |
| `bitbucket_create_pull_request` | Create one Bitbucket pull request. |
| `bitbucket_add_pull_request_comment` | Add one pull request comment. |
| `bitbucket_set_pull_request_review_status` | Set the configured user's pull request review status. |
| `bitbucket_merge_pull_request` | Merge one pull request with version safety. |
| `bitbucket_decline_pull_request` | Decline one pull request with version safety. |
| `bitbucket_reopen_pull_request` | Reopen one pull request with version safety. |
| `bitbucket_update_pull_request` | Update pull request title, description, and reviewers while preserving omitted fields. |
