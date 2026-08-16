#!/usr/bin/env bash
set -euo pipefail

MARKER="atlassian-mcp managed block"
BACKUP_PATHS=()
BACKUP_FILES=()
BACKUP_EXISTS=()
download_dir=""
# Set by download_release_binary() to the resolved GitHub release tag so the final success
# message can report it; stays empty when --binary is used (no release tag is resolved).
installed_release_tag=""

# Prints a fatal user-facing validation or execution error and terminates the installer.
die() {
  echo "install-from-remote.sh: $*" >&2
  exit 1
}

# Documents the public Bash interface; this is the contract used by raw-url bootstrap callers.
usage() {
  cat <<'USAGE'
Usage: scripts/install-from-remote.sh [options]

Installs atlassian-mcp from the published GitHub release binary or a prebuilt
local binary, then writes non-secret wrapper and agent configuration for Codex
and/or Claude.

Required source:
  --binary FILE_PATH                   install a local binary instead of downloading a release

Common options:
  --release-tag TAG                    exact release tag to install; default: latest stable release
  --install-dir DIRECTORY              default: $HOME/.local/bin
  --agents claude|codex|both|none      interactive when omitted unless --non-interactive
  --scope local|project|user           default: user
  --project-dir DIRECTORY              default: current directory
  --enable-jira
  --jira-base-url URL                  required with --enable-jira
  --jira-ca-file FILE_PATH
  --jira-username NAME                 optional; requires --enable-jira
  --jira-password-env VARIABLE_NAME    default: JIRA_PASSWORD
  --enable-confluence
  --confluence-base-url URL            required with --enable-confluence
  --confluence-ca-file FILE_PATH
  --confluence-username NAME           optional; requires --enable-confluence
  --confluence-password-env VARIABLE_NAME
                                      default: CONFLUENCE_PASSWORD
  --enable-bitbucket
  --bitbucket-base-url URL             required with --enable-bitbucket
  --bitbucket-project-key KEY          required with --enable-bitbucket
  --bitbucket-user-slug SLUG
  --bitbucket-token-env VARIABLE_NAME  default: BITBUCKET_BEARER_TOKEN
  --bitbucket-ca-file FILE_PATH
  --atlassian-tls-verify true|false    default: false
  --dry-run
  --replace                            replace managed Claude project config
  --non-interactive
USAGE
}

# Emits a single-quoted shell literal for wrapper exports without evaluating caller input.
shell_quote() {
  local value="${1//\'/\'\\\'\'}"
  printf "'%s'" "$value"
}

# Escapes the command path for a TOML basic string in Codex config.
toml_string() {
  local value="${1//\\/\\\\}"
  value="${value//\"/\\\"}"
  printf "%s" "$value"
}

# Escapes the command path for a JSON string in Claude MCP config.
json_string() {
  local value="${1//\\/\\\\}"
  value="${value//\"/\\\"}"
  printf "%s" "$value"
}

# Validates non-secret Atlassian service URLs before they are written to the wrapper.
require_url() {
  local name="$1"
  local value="$2"
  local authority
  [[ "$value" == http://* || "$value" == https://* ]] || die "$name must be an http or https URL"
  [[ "$value" != *"?"* && "$value" != *"#"* ]] || die "$name must not include query or fragment"
  authority="${value#*://}"
  authority="${authority%%/*}"
  [[ "$authority" != *@* ]] || die "$name must not include embedded credentials"
}

# Ensures a token/password environment indirection name can be embedded safely and looked up.
validate_token_env_name() {
  local flag_name="$1"
  local value="$2"
  [[ "$value" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]] || die "$flag_name must be a shell variable name"
}

# Reads the agent target only from the controlling terminal so piped installer source is never consumed as user input.
prompt_agents() {
  exec 3<>/dev/tty || die "--agents requires a terminal when omitted; pass --agents for piped or non-interactive installs"
  printf 'Select coding agents (claude/codex/both/none): ' >&3
  read -r agents <&3 || {
    exec 3>&-
    die "--agents requires a terminal when omitted; pass --agents for piped or non-interactive installs"
  }
  exec 3>&-
}

# Per-agent selection flags set by normalize_agents; consumed by validation and configure_agents.
select_claude="no"
select_codex="no"
select_cursor="no"
select_kiro="no"

# Normalizes the --agents expression into the four selection flags. Accepts canonical lowercase
# names, comma-separated combinations (repeated names are deduplicated), and the standalone
# aliases both (claude+codex), all (claude+codex+cursor+kiro), and none.
normalize_agents() {
  local raw="$1" name rest
  local -a names=()
  select_claude="no"
  select_codex="no"
  select_cursor="no"
  select_kiro="no"
  rest="$raw"
  while [[ "$rest" == *,* ]]; do
    names+=("${rest%%,*}")
    rest="${rest#*,}"
  done
  names+=("$rest")
  case "${names[0]}" in
    both)
      [[ "${#names[@]}" -eq 1 ]] || die "both cannot be combined with other agent names"
      select_claude="yes"
      select_codex="yes"
      return 0
      ;;
    all)
      [[ "${#names[@]}" -eq 1 ]] || die "all cannot be combined with other agent names"
      select_claude="yes"
      select_codex="yes"
      select_cursor="yes"
      select_kiro="yes"
      return 0
      ;;
    none)
      [[ "${#names[@]}" -eq 1 ]] || die "none cannot be combined with other agent names"
      return 0
      ;;
  esac
  for name in "${names[@]}"; do
    case "$name" in
      claude) select_claude="yes" ;;
      codex) select_codex="yes" ;;
      cursor) select_cursor="yes" ;;
      kiro) select_kiro="yes" ;;
      both|all|none) die "$name cannot be combined with other agent names" ;;
      "") die "--agents must not contain empty names" ;;
      *) die "--agents name '$name' must be one of: claude, codex, cursor, kiro (or the aliases both, all, none)" ;;
    esac
  done
}

# Records the current state of an agent config so later config failures can be rolled back.
backup_config() {
  local path="$1"
  local backup="${path}.bak.$$"
  BACKUP_PATHS+=("$path")
  BACKUP_FILES+=("$backup")
  if [[ -e "$path" ]]; then
    BACKUP_EXISTS+=("yes")
    cp "$path" "$backup"
  else
    BACKUP_EXISTS+=("no")
    mkdir -p "$(dirname "$path")"
    : >"$backup"
  fi
}

# Restores every backed-up agent config in reverse write order after a partial config failure.
rollback_configs() {
  local i
  for ((i=${#BACKUP_PATHS[@]}-1; i>=0; i--)); do
    if [[ "${BACKUP_EXISTS[$i]}" == "yes" ]]; then
      mv "${BACKUP_FILES[$i]}" "${BACKUP_PATHS[$i]}" || true
    else
      rm -f "${BACKUP_PATHS[$i]}" "${BACKUP_FILES[$i]}"
    fi
  done
}

# Removes successful-install backup files after all selected agent configs have been written.
cleanup_backups() {
  local backup
  for backup in "${BACKUP_FILES[@]}"; do
    rm -f "$backup"
  done
}

# Copies through a sibling temp file and renames into place so readers never see partial content.
atomic_copy() {
  local from="$1"
  local to="$2"
  local dir
  dir="$(dirname "$to")"
  mkdir -p "$dir"
  local tmp="${to}.tmp.$$"
  cp "$from" "$tmp"
  chmod +x "$tmp"
  mv "$tmp" "$to"
}

# Writes generated files atomically, optionally taking a rollback backup for agent configuration.
write_file_atomically() {
  local content="$1"
  local to="$2"
  local backup="${3:-no}"
  [[ -d "$to" ]] && return 1
  [[ "$backup" == "yes" ]] && backup_config "$to"
  atomic_copy "$content" "$to"
}

# Generates the runtime wrapper with approved non-secret module config and no credential values.
write_wrapper() {
  local wrapper="$1"
  local binary="$2"
  local content
  content="$(mktemp)"
  {
    echo "#!/usr/bin/env bash"
    echo "set -euo pipefail"
    echo "export ATLASSIAN_TLS_VERIFY=$(shell_quote "$atlassian_tls_verify")"
    if [[ -n "$confluence_username" ]]; then
      # Capture before clearing fixed runtime keys so --confluence-password-env=CONFLUENCE_PASSWORD works.
      echo "confluence_password_value=\"\${${confluence_password_env}:-}\""
    fi
    # Confluence is installer-managed here: clear stale fixed runtime values before exporting selected config.
    echo "unset CONFLUENCE_BASE_URL CONFLUENCE_CA_FILE CONFLUENCE_USERNAME CONFLUENCE_PASSWORD"
    [[ "$enable_jira" == "yes" ]] && echo "export JIRA_BASE_URL=$(shell_quote "$jira_base_url")"
    [[ -n "$jira_ca_file" ]] && echo "export JIRA_CA_FILE=$(shell_quote "$jira_ca_file")"
    if [[ -n "$jira_username" ]]; then
      echo "export JIRA_USERNAME=$(shell_quote "$jira_username")"
      # The password stays in the caller's environment; the wrapper only maps the configured variable name.
      echo "if [[ -z \"\${${jira_password_env}:-}\" ]]; then"
      echo "  echo \"JIRA password environment variable ${jira_password_env} is not set\" >&2"
      echo "  exit 1"
      echo "fi"
      echo "export JIRA_PASSWORD=\"\${${jira_password_env}}\""
    fi
    [[ "$enable_confluence" == "yes" ]] && echo "export CONFLUENCE_BASE_URL=$(shell_quote "$confluence_base_url")"
    [[ -n "$confluence_ca_file" ]] && echo "export CONFLUENCE_CA_FILE=$(shell_quote "$confluence_ca_file")"
    if [[ -n "$confluence_username" ]]; then
      echo "export CONFLUENCE_USERNAME=$(shell_quote "$confluence_username")"
      # The password stays in the caller's environment; the wrapper only maps the configured variable name.
      echo "if [[ -z \"\$confluence_password_value\" ]]; then"
      echo "  echo \"CONFLUENCE password environment variable ${confluence_password_env} is not set\" >&2"
      echo "  exit 1"
      echo "fi"
      echo "export CONFLUENCE_PASSWORD=\"\$confluence_password_value\""
    fi
    if [[ "$enable_bitbucket" == "yes" ]]; then
      echo "export BITBUCKET_BASE_URL=$(shell_quote "$bitbucket_base_url")"
      echo "export BITBUCKET_PROJECT_KEY=$(shell_quote "$bitbucket_project_key")"
      [[ -n "$bitbucket_user_slug" ]] && echo "export BITBUCKET_USER_SLUG=$(shell_quote "$bitbucket_user_slug")"
      [[ -n "$bitbucket_ca_file" ]] && echo "export BITBUCKET_CA_FILE=$(shell_quote "$bitbucket_ca_file")"
      # The token value stays in the caller's environment; the wrapper only maps the configured variable name.
      echo "if [[ -z \"\${${bitbucket_token_env}:-}\" ]]; then"
      echo "  echo \"BITBUCKET token environment variable ${bitbucket_token_env} is not set\" >&2"
      echo "  exit 1"
      echo "fi"
      echo "export BITBUCKET_BEARER_TOKEN=\"\${${bitbucket_token_env}}\""
    fi
    echo "exec $(shell_quote "$binary") \"\$@\""
  } >"$content"
  write_file_atomically "$content" "$wrapper"
  rm -f "$content"
}

# Produces the [mcp_servers.atlassian.env] lines Codex needs. Codex's MCP launcher does not pass its
# own ambient environment through to spawned stdio servers (confirmed from Codex's own logs: the
# binary started with every module disabled even with the wrapper script, which itself resolves
# BITBUCKET_TOKEN_ENV-style indirection only from whatever environment the spawning process gives
# it -- Codex gives it none). Callers must have already validated that each resolved
# secret is available before calling this; see the top-level validation block.
codex_env_lines() {
  echo "ATLASSIAN_TLS_VERIFY = \"$(toml_string "$atlassian_tls_verify")\""
  if [[ "$enable_jira" == "yes" ]]; then
    echo "JIRA_BASE_URL = \"$(toml_string "$jira_base_url")\""
  fi
  [[ -n "$jira_ca_file" ]] && echo "JIRA_CA_FILE = \"$(toml_string "$jira_ca_file")\""
  if [[ -n "$jira_username" ]]; then
    echo "JIRA_USERNAME = \"$(toml_string "$jira_username")\""
    echo "JIRA_PASSWORD = \"$(toml_string "${!jira_password_env}")\""
  fi
  if [[ "$enable_confluence" == "yes" ]]; then
    echo "CONFLUENCE_BASE_URL = \"$(toml_string "$confluence_base_url")\""
  fi
  [[ -n "$confluence_ca_file" ]] && echo "CONFLUENCE_CA_FILE = \"$(toml_string "$confluence_ca_file")\""
  if [[ -n "$confluence_username" ]]; then
    echo "CONFLUENCE_USERNAME = \"$(toml_string "$confluence_username")\""
    echo "CONFLUENCE_PASSWORD = \"$(toml_string "${!confluence_password_env}")\""
  fi
  if [[ "$enable_bitbucket" == "yes" ]]; then
    echo "BITBUCKET_BASE_URL = \"$(toml_string "$bitbucket_base_url")\""
    echo "BITBUCKET_PROJECT_KEY = \"$(toml_string "$bitbucket_project_key")\""
    [[ -n "$bitbucket_user_slug" ]] && echo "BITBUCKET_USER_SLUG = \"$(toml_string "$bitbucket_user_slug")\""
    [[ -n "$bitbucket_ca_file" ]] && echo "BITBUCKET_CA_FILE = \"$(toml_string "$bitbucket_ca_file")\""
    echo "BITBUCKET_BEARER_TOKEN = \"$(toml_string "${!bitbucket_token_env}")\""
  fi
}

# Replaces only the installer-managed Codex block, preserving unrelated TOML around it. Command is
# always the built binary, not the wrapper -- see codex_env_lines for why Codex needs resolved
# values written directly into its config instead of relying on the wrapper's runtime indirection.
# This does put the Bitbucket token and, if configured, Jira/Confluence passwords in Codex's
# config file, unlike Claude's; that is a deliberate, Codex-specific exception, not a general policy change.
managed_codex_config() {
  local path="$1"
  local command="$2"
  local body
  body="$(mktemp)"
  if [[ -f "$path" ]]; then
    awk "/# BEGIN ${MARKER}/{skip=1; next} /# END ${MARKER}/{skip=0; next} !skip{print}" "$path" >"$body"
  fi
  {
    cat "$body"
    echo "# BEGIN ${MARKER}"
    echo "[mcp_servers.atlassian]"
    echo "command = \"$(toml_string "$command")\""
    echo "args = []"
    local env_lines
    env_lines="$(codex_env_lines)"
    if [[ -n "$env_lines" ]]; then
      echo "[mcp_servers.atlassian.env]"
      printf '%s\n' "$env_lines"
    fi
    echo "# END ${MARKER}"
  } >"${body}.next"
  echo "${body}.next"
}

# Writes a managed Claude MCP file and refuses to overwrite unmanaged content without --replace.
# Only used for --scope project; --scope local/user register through the claude CLI instead.
managed_claude_config() {
  local path="$1"
  local command="$2"
  local body
  body="$(mktemp)"
  if [[ -e "$path" && "$replace" != "yes" ]] && ! grep -F "atlassian-mcp managed by install-from-remote.sh" "$path" >/dev/null 2>&1; then
    die "refusing to replace unmanaged Claude config at $path; use --replace"
  fi
  cat >"$body" <<JSON
{
  "_comment": "atlassian-mcp managed by install-from-remote.sh",
  "mcpServers": {
    "atlassian": {
      "command": "$(json_string "$command")"
    }
  }
}
JSON
  echo "$body"
}

# Merges the atlassian entry into a JSON-backed agent config (Cursor/Kiro), preserving unrelated
# root keys and unrelated mcpServers entries. Prints the merged JSON on stdout. Exits the
# subshell with 1 (via die) on malformed input or an unaccepted conflict; returns 2 when the
# existing entry is already identical, which callers treat as an idempotent no-op.
merge_mcp_json() {
  local path="$1"
  local entry="$2"
  local existing
  [[ -d "$path" ]] && die "refusing to replace directory config at $path"
  if [[ -f "$path" ]]; then
    existing="$(cat "$path")"
    # A whitespace-only file carries no content to preserve; treat it like a missing config.
    if [[ -z "${existing//[[:space:]]/}" ]]; then
      existing="{}"
    else
      printf '%s' "$existing" | jq -e . >/dev/null 2>&1 || die "existing config at $path is not valid JSON"
    fi
  else
    existing="{}"
  fi
  printf '%s' "$existing" | jq -e 'type == "object"' >/dev/null 2>&1 || die "existing config at $path must be a JSON object"
  printf '%s' "$existing" | jq -e 'if has("mcpServers") then (.mcpServers | type == "object") else true end' >/dev/null 2>&1 ||
    die "mcpServers in $path must be a JSON object"
  if printf '%s' "$existing" | jq -e 'has("mcpServers") and (.mcpServers | has("atlassian"))' >/dev/null 2>&1; then
    if printf '%s' "$existing" | jq -e --argjson entry "$entry" '.mcpServers.atlassian == $entry' >/dev/null 2>&1; then
      return 2
    fi
    [[ "$replace" == "yes" ]] || die "refusing to replace existing atlassian entry in $path; use --replace"
  fi
  printf '%s' "$existing" | jq --argjson entry "$entry" '.mcpServers = ((.mcpServers // {}) + {atlassian: $entry})'
}

# Builds the Cursor MCP server entry and merges it into the Cursor config. Command is the wrapper:
# it already carries the non-secret runtime config and resolves credential env indirection at
# runtime, so no resolved secret values are written into Cursor's JSON.
configure_cursor() {
  local command="$1"
  local entry merged rc content
  entry="$(jq -cn --arg command "$command" '{type: "stdio", command: $command, args: []}')" || return 1
  merged="$(merge_mcp_json "$cursor_config" "$entry")" && rc=0 || rc=$?
  if [[ "$rc" -eq 2 ]]; then
    return 0
  fi
  [[ "$rc" -eq 0 ]] || return 1
  content="$(mktemp)"
  printf '%s\n' "$merged" >"$content"
  write_file_atomically "$content" "$cursor_config" yes || return 1
  rm -f "$content"
}

# Ensures the Claude Code CLI is present before it is used to register the atlassian MCP server.
require_claude_cli() {
  command -v claude >/dev/null 2>&1 || die "claude CLI is required for --scope local/user; install it, use --scope project, or select --agents codex"
}

# Registers/updates the atlassian MCP server via the Claude Code CLI so the entry lands in Claude's
# real config store instead of a hand-written file (writing directly to e.g. ~/.claude/settings.json
# does not register an MCP server with Claude Code).
configure_claude_cli() {
  local command="$1"
  require_claude_cli
  (
    cd "$project_dir" &&
    { claude mcp remove atlassian --scope "$scope" >/dev/null 2>&1 || true; } &&
    claude mcp add atlassian --scope "$scope" -- "$command"
  ) || return 1
  ( cd "$project_dir" && claude mcp get atlassian --scope "$scope" >/dev/null 2>&1 ) ||
    echo "warning: could not verify atlassian MCP registration via claude mcp get" >&2
}

# Resolves user, local, and project config targets for the selected coding agents. Cursor uses the
# same project/workspace file for both local and project scope.
config_paths() {
  case "$scope" in
    user)
      codex_config="$HOME/.codex/config.toml"
      cursor_config="$HOME/.cursor/mcp.json"
      ;;
    local)
      codex_config="$project_dir/.codex/config.toml"
      cursor_config="$project_dir/.cursor/mcp.json"
      ;;
    project)
      codex_config="$project_dir/.codex/config.toml"
      claude_config="$project_dir/.mcp.json"
      cursor_config="$project_dir/.cursor/mcp.json"
      ;;
    *) die "--scope must be local, project, or user" ;;
  esac
}

# Configures the selected agent files idempotently; caller performs rollback on any failure.
# command is the wrapper (used for Claude/manual runs); binary is the installed executable, used
# directly for Codex since its config carries resolved env values instead of relying on the
# wrapper's runtime indirection lookup (see codex_env_lines).
configure_agents() {
  local command="$1"
  local binary="$2"
  local codex_content
  local claude_content
  config_paths
  if [[ "$select_codex" == "yes" ]]; then
    codex_content="$(managed_codex_config "$codex_config" "$binary")" || return 1
    write_file_atomically "$codex_content" "$codex_config" yes || return 1
    rm -f "$codex_content"
  fi
  if [[ "$select_claude" == "yes" ]]; then
    case "$scope" in
      project)
        claude_content="$(managed_claude_config "$claude_config" "$command")" || return 1
        write_file_atomically "$claude_content" "$claude_config" yes || return 1
        rm -f "$claude_content"
        ;;
      *)
        configure_claude_cli "$command" || return 1
        ;;
    esac
  fi
  if [[ "$select_cursor" == "yes" ]]; then
    configure_cursor "$command" || return 1
  fi
}

release_tag=""
binary=""
install_dir="${HOME:-}/.local/bin"
agents=""
scope="user"
project_dir="$(pwd)"
enable_jira="no"
jira_base_url=""
jira_ca_file=""
jira_username=""
jira_password_env="JIRA_PASSWORD"
enable_confluence="no"
confluence_base_url=""
confluence_ca_file=""
confluence_username=""
confluence_password_env="CONFLUENCE_PASSWORD"
enable_bitbucket="no"
bitbucket_base_url=""
bitbucket_project_key=""
bitbucket_user_slug=""
bitbucket_token_env="BITBUCKET_BEARER_TOKEN"
bitbucket_ca_file=""
atlassian_tls_verify="false"
dry_run="no"
replace="no"
non_interactive="no"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --release-tag) release_tag="${2:-}"; shift 2 ;;
    --binary) binary="${2:-}"; shift 2 ;;
    --install-dir) install_dir="${2:-}"; shift 2 ;;
    --agents) agents="${2:-}"; shift 2 ;;
    --scope) scope="${2:-}"; shift 2 ;;
    --project-dir) project_dir="${2:-}"; shift 2 ;;
    --enable-jira) enable_jira="yes"; shift ;;
    --jira-base-url) jira_base_url="${2:-}"; shift 2 ;;
    --jira-ca-file) jira_ca_file="${2:-}"; shift 2 ;;
    --jira-username) jira_username="${2:-}"; shift 2 ;;
    --jira-password-env) jira_password_env="${2:-}"; shift 2 ;;
    --enable-confluence) enable_confluence="yes"; shift ;;
    --confluence-base-url) confluence_base_url="${2:-}"; shift 2 ;;
    --confluence-ca-file) confluence_ca_file="${2:-}"; shift 2 ;;
    --confluence-username) confluence_username="${2:-}"; shift 2 ;;
    --confluence-password-env) confluence_password_env="${2:-}"; shift 2 ;;
    --enable-bitbucket) enable_bitbucket="yes"; shift ;;
    --bitbucket-base-url) bitbucket_base_url="${2:-}"; shift 2 ;;
    --bitbucket-project-key) bitbucket_project_key="${2:-}"; shift 2 ;;
    --bitbucket-user-slug) bitbucket_user_slug="${2:-}"; shift 2 ;;
    --bitbucket-token-env) bitbucket_token_env="${2:-}"; shift 2 ;;
    --bitbucket-ca-file) bitbucket_ca_file="${2:-}"; shift 2 ;;
    --atlassian-tls-verify) atlassian_tls_verify="${2:-}"; shift 2 ;;
    --dry-run) dry_run="yes"; shift ;;
    --replace) replace="yes"; shift ;;
    --non-interactive) non_interactive="yes"; shift ;;
    -h|--help) usage; exit 0 ;;
    *) die "unknown option: $1" ;;
  esac
done

cleanup_download() {
  if [[ -n "$download_dir" ]]; then
    rm -rf "$download_dir"
    download_dir=""
  fi
}

trap 'cleanup_download' EXIT

[[ -z "$binary" || -f "$binary" ]] || die "--binary must point to a readable file"
[[ -z "$release_tag" || "$release_tag" =~ ^v[0-9]+[.][0-9]+[.][0-9]+([-+][A-Za-z0-9._-]+)?$ ]] || die "--release-tag must look like v1.2.3"
if [[ -z "$agents" ]]; then
  [[ "$non_interactive" != "yes" ]] || die "--agents is required with --non-interactive"
  prompt_agents
fi
normalize_agents "$agents"
[[ "$enable_jira" == "yes" || "$enable_confluence" == "yes" || "$enable_bitbucket" == "yes" ]] || die "select at least one module with --enable-jira, --enable-confluence, or --enable-bitbucket"
[[ "$atlassian_tls_verify" == "true" || "$atlassian_tls_verify" == "false" ]] || die "--atlassian-tls-verify must be true or false"
if [[ "$select_cursor" == "yes" || "$select_kiro" == "yes" ]]; then
  command -v jq >/dev/null 2>&1 || die "jq is required when cursor or kiro is selected; install jq or remove cursor/kiro from --agents"
fi

if [[ "$enable_jira" == "yes" ]]; then
  [[ -n "$jira_base_url" ]] || die "--jira-base-url is required with --enable-jira"
  require_url "--jira-base-url" "$jira_base_url"
fi
if [[ -n "$jira_username" ]]; then
  [[ "$enable_jira" == "yes" ]] || die "--jira-username requires --enable-jira"
  validate_token_env_name "--jira-password-env" "$jira_password_env"
  [[ "$non_interactive" != "yes" || -n "${!jira_password_env:-}" ]] || die "$jira_password_env is required for non-interactive installs when --jira-username is set"
  # Codex's config carries the resolved password directly (see codex_env_lines), unlike the wrapper
  # used for Claude/manual runs, which resolves --jira-password-env only at its own runtime -- so
  # Codex needs it available now regardless of --non-interactive.
  [[ "$select_codex" != "yes" || -n "${!jira_password_env:-}" ]] || die "$jira_password_env is required to configure Codex when --jira-username is set"
fi
if [[ "$enable_confluence" == "yes" ]]; then
  [[ -n "$confluence_base_url" ]] || die "--confluence-base-url is required with --enable-confluence"
  require_url "--confluence-base-url" "$confluence_base_url"
fi
if [[ -n "$confluence_username" ]]; then
  [[ "$enable_confluence" == "yes" ]] || die "--confluence-username requires --enable-confluence"
  validate_token_env_name "--confluence-password-env" "$confluence_password_env"
  [[ "$non_interactive" != "yes" || -n "${!confluence_password_env:-}" ]] || die "$confluence_password_env is required for non-interactive installs when --confluence-username is set"
  # Codex's config carries the resolved password directly (see codex_env_lines), unlike the wrapper
  # used for Claude/manual runs, which resolves --confluence-password-env only at its own runtime.
  [[ "$select_codex" != "yes" || -n "${!confluence_password_env:-}" ]] || die "$confluence_password_env is required to configure Codex when --confluence-username is set"
fi
if [[ "$enable_bitbucket" == "yes" ]]; then
  [[ -n "$bitbucket_base_url" ]] || die "--bitbucket-base-url is required with --enable-bitbucket"
  [[ -n "$bitbucket_project_key" ]] || die "--bitbucket-project-key is required with --enable-bitbucket"
  require_url "--bitbucket-base-url" "$bitbucket_base_url"
  validate_token_env_name "--bitbucket-token-env" "$bitbucket_token_env"
  [[ "$non_interactive" != "yes" || -n "${!bitbucket_token_env:-}" ]] || die "$bitbucket_token_env is required for non-interactive Bitbucket installs"
  [[ "$select_codex" != "yes" || -n "${!bitbucket_token_env:-}" ]] || die "$bitbucket_token_env is required to configure Codex"
fi

if [[ "$dry_run" == "yes" ]]; then
  echo "dry-run: validated installer arguments"
  exit 0
fi

release_platform() {
  local os arch
  os="$(uname -s)"
  arch="$(uname -m)"
  if [[ "$os" == "Linux" && ( "$arch" == "x86_64" || "$arch" == "amd64" ) ]]; then
    printf 'linux_amd64\n'
    return
  fi
  die "unsupported platform: $os/$arch (supported: Linux amd64)"
}

resolve_release_tag() {
  if [[ -n "$release_tag" ]]; then
    printf '%s\n' "$release_tag"
    return
  fi
  local json tag
  json="$(curl -fsSL --connect-timeout 15 --max-time 120 "https://api.github.com/repos/chiendao1808/atlassian-mcp/releases/latest")"
  tag="$(printf '%s\n' "$json" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)"
  [[ -n "$tag" ]] || die "could not resolve latest GitHub release tag"
  printf '%s\n' "$tag"
}

download_release_binary() {
  local platform tag version asset checksum_asset base_url expected actual line
  platform="$(release_platform)"
  tag="$(resolve_release_tag)"
  installed_release_tag="$tag"
  version="${tag#v}"
  asset="atlassian-mcp_${version}_${platform}"
  checksum_asset="atlassian-mcp_${version}_checksums.txt"
  base_url="https://github.com/chiendao1808/atlassian-mcp/releases/download/$tag"
  download_dir="$(mktemp -d "${TMPDIR:-/tmp}/atlassian-mcp-release-XXXXXX")"

  curl -fsSL --connect-timeout 15 --max-time 120 -o "$download_dir/$asset" "$base_url/$asset"
  curl -fsSL --connect-timeout 15 --max-time 120 -o "$download_dir/$checksum_asset" "$base_url/$checksum_asset"
  line="$(awk -v name="$asset" '$2 == name || $2 == "*" name { print; exit }' "$download_dir/$checksum_asset")"
  [[ -n "$line" ]] || die "checksum entry not found for $asset"
  expected="${line%%[[:space:]]*}"
  actual="$(sha256sum "$download_dir/$asset" | awk '{print $1}')"
  [[ "$actual" == "$expected" ]] || die "checksum mismatch for $asset"
  chmod +x "$download_dir/$asset"
  built_binary="$download_dir/$asset"
}

if [[ -n "$binary" ]]; then
  built_binary="$binary"
else
  download_release_binary
fi

installed_binary="$install_dir/atlassian-mcp"
wrapper="$install_dir/atlassian-mcp-run"
atomic_copy "$built_binary" "$installed_binary"
write_wrapper "$wrapper" "$installed_binary"

if [[ "$select_claude" == "yes" || "$select_codex" == "yes" || "$select_cursor" == "yes" || "$select_kiro" == "yes" ]]; then
  if ! configure_agents "$wrapper" "$installed_binary"; then
    rollback_configs
    die "failed to configure selected agents"
  fi
  cleanup_backups
fi

if [[ -n "$installed_release_tag" ]]; then
  echo "installed atlassian-mcp ${installed_release_tag} to $installed_binary"
else
  echo "installed atlassian-mcp (local binary) to $installed_binary"
fi
cleanup_download
