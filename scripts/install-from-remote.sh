#!/usr/bin/env bash
set -euo pipefail

MARKER="atlassian-mcp managed block"
BACKUP_PATHS=()
BACKUP_FILES=()
BACKUP_EXISTS=()
install_succeeded="no"

# Prints a fatal user-facing validation or execution error and terminates the installer.
die() {
  echo "install-from-remote.sh: $*" >&2
  exit 1
}

# Documents the public Bash interface; this is the contract used by raw-url bootstrap callers.
usage() {
  cat <<'USAGE'
Usage: scripts/install-from-remote.sh [options]

Installs atlassian-mcp from a provider-neutral Git remote or a prebuilt binary,
then writes non-secret wrapper and agent configuration for Codex and/or Claude.

Required source:
  --source-repo-url URL                required unless --binary is used
  --binary FILE_PATH                   install prebuilt binary instead of cloning/building

Common options:
  --source-ref REF                     default: main
  --source-clone-depth N               default: 1
  --keep-source
  --install-dir DIRECTORY              default: $HOME/.local/bin
  --agents claude|codex|both|none      interactive when omitted unless --non-interactive
  --scope local|project|user           default: user
  --project-dir DIRECTORY              default: current directory
  --enable-jira
  --jira-base-url URL                  required with --enable-jira
  --jira-ca-file FILE_PATH
  --enable-bitbucket
  --bitbucket-base-url URL             required with --enable-bitbucket
  --bitbucket-project-key KEY          required with --enable-bitbucket
  --bitbucket-user-slug SLUG
  --bitbucket-token-env VARIABLE_NAME  default: BITBUCKET_BEARER_TOKEN
  --bitbucket-ca-file FILE_PATH
  --atlassian-tls-verify true|false    default: false
  --skip-tests
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

# Rejects credential-bearing source URLs before any Git command can log or persist them.
reject_embedded_credentials() {
  local url="$1"
  case "$url" in
    http://*@*|https://*@*) die "--source-repo-url must not include embedded credentials" ;;
  esac
}

# Ensures the token environment indirection can be embedded safely in the generated wrapper.
validate_token_env_name() {
  [[ "$1" =~ ^[A-Za-z_][A-Za-z0-9_]*$ ]] || die "--bitbucket-token-env must be a shell variable name"
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
    [[ "$enable_jira" == "yes" ]] && echo "export JIRA_BASE_URL=$(shell_quote "$jira_base_url")"
    [[ -n "$jira_ca_file" ]] && echo "export JIRA_CA_FILE=$(shell_quote "$jira_ca_file")"
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

# Replaces only the installer-managed Codex block, preserving unrelated TOML around it.
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
    echo "# END ${MARKER}"
  } >"${body}.next"
  echo "${body}.next"
}

# Writes a managed Claude MCP file and refuses to overwrite unmanaged content without --replace.
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

# Resolves user, local, and project config targets for the selected coding agents.
config_paths() {
  case "$scope" in
    user)
      codex_config="$HOME/.codex/config.toml"
      claude_config="$HOME/.claude/atlassian-mcp.mcp.json"
      ;;
    local|project)
      codex_config="$project_dir/.codex/config.toml"
      claude_config="$project_dir/.mcp.json"
      ;;
    *) die "--scope must be local, project, or user" ;;
  esac
}

# Configures the selected agent files idempotently; caller performs rollback on any failure.
configure_agents() {
  local command="$1"
  local codex_content
  local claude_content
  config_paths
  case "$agents" in
    codex|both)
      codex_content="$(managed_codex_config "$codex_config" "$command")" || return 1
      write_file_atomically "$codex_content" "$codex_config" yes || return 1
      rm -f "$codex_content"
      ;;
  esac
  case "$agents" in
    claude|both)
      claude_content="$(managed_claude_config "$claude_config" "$command")" || return 1
      write_file_atomically "$claude_content" "$claude_config" yes || return 1
      rm -f "$claude_content"
      ;;
  esac
}

# Clones the provider-neutral source remote and checks out the requested branch, tag, or commit.
clone_source() {
  source_dir="$(mktemp -d "${TMPDIR:-/tmp}/atlassian-mcp-src-XXXXXX")"
  git clone --depth "$source_clone_depth" "$source_repo_url" "$source_dir"
  (
    cd "$source_dir"
    git fetch --depth "$source_clone_depth" origin "$source_ref" >/dev/null 2>&1 || true
    git checkout "$source_ref"
    git rev-parse --verify HEAD >/dev/null
  )
}

# Runs repository tests unless skipped, then builds cmd/atlassian-mcp into a temporary binary.
build_binary() {
  local out
  out="$(mktemp)"
  if [[ "$skip_tests" != "yes" ]]; then
    (cd "$source_dir" && go test ./...)
  fi
  (cd "$source_dir" && go build -o "$out" ./cmd/atlassian-mcp)
  built_binary="$out"
}

# Removes cloned source after install, unless --keep-source was requested for debugging.
cleanup_source() {
  if [[ -n "${source_dir:-}" && "$keep_source" != "yes" ]]; then
    local old_source_dir="$source_dir"
    rm -rf "$old_source_dir"
    if [[ -e "$old_source_dir" ]]; then
      echo "warning: could not clean cloned source $old_source_dir" >&2
    else
      echo "cleaned cloned source $old_source_dir"
      source_dir=""
    fi
  fi
  return 0
}

source_repo_url=""
source_ref="main"
source_clone_depth="1"
keep_source="no"
binary=""
install_dir="${HOME:-}/.local/bin"
agents=""
scope="user"
project_dir="$(pwd)"
enable_jira="no"
jira_base_url=""
jira_ca_file=""
enable_bitbucket="no"
bitbucket_base_url=""
bitbucket_project_key=""
bitbucket_user_slug=""
bitbucket_token_env="BITBUCKET_BEARER_TOKEN"
bitbucket_ca_file=""
atlassian_tls_verify="false"
skip_tests="no"
dry_run="no"
replace="no"
non_interactive="no"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --source-repo-url) source_repo_url="${2:-}"; shift 2 ;;
    --source-ref) source_ref="${2:-}"; shift 2 ;;
    --source-clone-depth) source_clone_depth="${2:-}"; shift 2 ;;
    --keep-source) keep_source="yes"; shift ;;
    --binary) binary="${2:-}"; shift 2 ;;
    --install-dir) install_dir="${2:-}"; shift 2 ;;
    --agents) agents="${2:-}"; shift 2 ;;
    --scope) scope="${2:-}"; shift 2 ;;
    --project-dir) project_dir="${2:-}"; shift 2 ;;
    --enable-jira) enable_jira="yes"; shift ;;
    --jira-base-url) jira_base_url="${2:-}"; shift 2 ;;
    --jira-ca-file) jira_ca_file="${2:-}"; shift 2 ;;
    --enable-bitbucket) enable_bitbucket="yes"; shift ;;
    --bitbucket-base-url) bitbucket_base_url="${2:-}"; shift 2 ;;
    --bitbucket-project-key) bitbucket_project_key="${2:-}"; shift 2 ;;
    --bitbucket-user-slug) bitbucket_user_slug="${2:-}"; shift 2 ;;
    --bitbucket-token-env) bitbucket_token_env="${2:-}"; shift 2 ;;
    --bitbucket-ca-file) bitbucket_ca_file="${2:-}"; shift 2 ;;
    --atlassian-tls-verify) atlassian_tls_verify="${2:-}"; shift 2 ;;
    --skip-tests) skip_tests="yes"; shift ;;
    --dry-run) dry_run="yes"; shift ;;
    --replace) replace="yes"; shift ;;
    --non-interactive) non_interactive="yes"; shift ;;
    -h|--help) usage; exit 0 ;;
    *) die "unknown option: $1" ;;
  esac
done

trap '[[ "$install_succeeded" == "yes" ]] || cleanup_source' EXIT

[[ -n "$binary" || -n "$source_repo_url" ]] || die "--source-repo-url is required unless --binary is used"
[[ -n "$binary" || ! "$source_repo_url" =~ ^https?:// ]] || reject_embedded_credentials "$source_repo_url"
[[ -n "$binary" || "$source_repo_url" == https://* || "$source_repo_url" == http://* || "$source_repo_url" == git@* || "$source_repo_url" == ssh://* ]] || die "--source-repo-url must be HTTPS or SSH"
[[ -z "$binary" || -f "$binary" ]] || die "--binary must point to a readable file"
if [[ -z "$agents" ]]; then
  [[ "$non_interactive" != "yes" ]] || die "--agents is required with --non-interactive"
  prompt_agents
fi
[[ "$agents" == "claude" || "$agents" == "codex" || "$agents" == "both" || "$agents" == "none" ]] || die "--agents must be claude, codex, both, or none"
[[ "$enable_jira" == "yes" || "$enable_bitbucket" == "yes" ]] || die "select at least one module with --enable-jira or --enable-bitbucket"
[[ "$atlassian_tls_verify" == "true" || "$atlassian_tls_verify" == "false" ]] || die "--atlassian-tls-verify must be true or false"

if [[ "$enable_jira" == "yes" ]]; then
  [[ -n "$jira_base_url" ]] || die "--jira-base-url is required with --enable-jira"
  require_url "--jira-base-url" "$jira_base_url"
fi
if [[ "$enable_bitbucket" == "yes" ]]; then
  [[ -n "$bitbucket_base_url" ]] || die "--bitbucket-base-url is required with --enable-bitbucket"
  [[ -n "$bitbucket_project_key" ]] || die "--bitbucket-project-key is required with --enable-bitbucket"
  require_url "--bitbucket-base-url" "$bitbucket_base_url"
  validate_token_env_name "$bitbucket_token_env"
  [[ "$non_interactive" != "yes" || -n "${!bitbucket_token_env:-}" ]] || die "$bitbucket_token_env is required for non-interactive Bitbucket installs"
fi

if [[ "$dry_run" == "yes" ]]; then
  echo "dry-run: validated installer arguments"
  exit 0
fi

if [[ -n "$binary" ]]; then
  built_binary="$binary"
else
  clone_source
  build_binary
fi

installed_binary="$install_dir/atlassian-mcp"
wrapper="$install_dir/atlassian-mcp-run"
atomic_copy "$built_binary" "$installed_binary"
write_wrapper "$wrapper" "$installed_binary"

if [[ "$agents" != "none" ]]; then
  if ! configure_agents "$wrapper"; then
    rollback_configs
    die "failed to configure selected agents"
  fi
  cleanup_backups
fi

echo "installed atlassian-mcp to $installed_binary"
install_succeeded="yes"
cleanup_source
