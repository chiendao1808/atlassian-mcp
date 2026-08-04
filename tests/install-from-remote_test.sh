#!/usr/bin/env bash
set -euo pipefail

PATH="/usr/bin:/bin:$PATH"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
INSTALLER="$REPO_ROOT/scripts/install-from-remote.sh"
TMP_ROOT="$(mktemp -d)"

trap 'rm -rf "$TMP_ROOT"' EXIT

# Minimal shell harness: each test runs the real installer with fake external tools and asserts observable files, logs, or failures.
fail() {
  echo "FAIL: $*" >&2
  exit 1
}

assert_file() {
  [[ -f "$1" ]] || fail "missing file: $1"
}

assert_path_missing() {
  [[ ! -e "$1" ]] || fail "path still exists: $1"
}

assert_contains() {
  local file="$1"
  local text="$2"
  grep -F -- "$text" "$file" >/dev/null || fail "missing '$text' in $file"
}

assert_not_contains() {
  local file="$1"
  local text="$2"
  if grep -F -- "$text" "$file" >/dev/null 2>&1; then
    fail "unexpected '$text' in $file"
  fi
}

assert_count() {
  local file="$1"
  local text="$2"
  local want="$3"
  local got
  got="$(grep -F -- "$text" "$file" 2>/dev/null | wc -l | tr -d ' ')"
  [[ "$got" == "$want" ]] || fail "count for '$text' in $file = $got, want $want"
}

get_cloned_source_dir() {
  local log="$1"
  local path
  path="$(awk '/^git clone /{print $NF; exit}' "$log")"
  [[ -n "$path" ]] || fail "missing git clone log line"
  printf '%s\n' "$path"
}

# Creates fake git/go/cp/mv commands so installer tests prove command selection without network or host installs.
make_fakes() {
  local dir="$1"
  mkdir -p "$dir"
  cat >"$dir/git" <<'FAKE_GIT'
#!/usr/bin/env bash
set -euo pipefail
echo "git $*" >>"$FAKE_LOG"
case "$1" in
  clone)
    dest="${@: -1}"
    mkdir -p "$dest/cmd/atlassian-mcp"
    printf 'module example.com/atlassian-mcp\n' >"$dest/go.mod"
    ;;
  fetch|checkout)
    ;;
  rev-parse)
    printf 'mocked-ref\n'
    ;;
esac
FAKE_GIT
  cat >"$dir/go" <<'FAKE_GO'
#!/usr/bin/env bash
set -euo pipefail
echo "go $*" >>"$FAKE_LOG"
if [[ "$1" == "build" ]]; then
  out=""
  while [[ $# -gt 0 ]]; do
    case "$1" in
      -o)
        out="$2"
        shift 2
        ;;
      *)
        shift
        ;;
    esac
  done
  [[ -n "$out" ]] || exit 1
  mkdir -p "$(dirname "$out")"
  printf '#!/usr/bin/env bash\necho atlassian-mcp\n' >"$out"
  chmod +x "$out"
fi
FAKE_GO
  cat >"$dir/cp" <<'FAKE_CP'
#!/usr/bin/env bash
echo "cp $*" >>"$FAKE_LOG"
exec /usr/bin/cp "$@"
FAKE_CP
  cat >"$dir/mv" <<'FAKE_MV'
#!/usr/bin/env bash
echo "mv $*" >>"$FAKE_LOG"
exec /usr/bin/mv "$@"
FAKE_MV
  cat >"$dir/claude" <<'FAKE_CLAUDE'
#!/usr/bin/env bash
echo "claude $*" >>"$FAKE_LOG"
case "$1 $2" in
  "mcp remove") exit 1 ;;
  "mcp add") exit 0 ;;
  "mcp get") exit 0 ;;
  *) exit 0 ;;
esac
FAKE_CLAUDE
  chmod +x "$dir/git" "$dir/go" "$dir/cp" "$dir/mv" "$dir/claude"
}

# Runs one isolated installer case with HOME, install dir, project dir, and PATH scoped to the test temp tree.
run_installer() {
  local name="$1"
  shift
  local case_dir="$TMP_ROOT/$name"
  local fake_bin="$case_dir/bin"
  mkdir -p "$case_dir/home" "$case_dir/install" "$case_dir/project"
  make_fakes "$fake_bin"
  FAKE_LOG="$case_dir/commands.log" \
  HOME="$case_dir/home" \
  PATH="$fake_bin:/usr/bin:/bin" \
    bash "$INSTALLER" \
      --install-dir "$case_dir/install" \
      --project-dir "$case_dir/project" \
      --scope project \
      --non-interactive \
      "$@"
}

test_https_remotes_checkout_test_build_and_install_atomically() {
  local urls=(
    "https://github.com/acme/atlassian-mcp.git"
    "https://gitlab.com/acme/atlassian-mcp.git"
    "https://bitbucket.internal.example.com/scm/prj/atlassian-mcp.git"
  )
  local url
  for url in "${urls[@]}"; do
    local name="remote-${url//[^A-Za-z0-9]/-}"
    run_installer "$name" \
      --source-repo-url "$url" \
      --source-ref v1.2.3 \
      --agents codex \
      --enable-jira \
      --jira-base-url https://jira.internal.example.com/jira
    local dir="$TMP_ROOT/$name"
    assert_contains "$dir/commands.log" "git clone"
    assert_contains "$dir/commands.log" "$url"
    assert_contains "$dir/commands.log" "git checkout v1.2.3"
    assert_contains "$dir/commands.log" "go test ./..."
    assert_contains "$dir/commands.log" "go build -o"
    assert_contains "$dir/commands.log" "mv"
    assert_file "$dir/install/atlassian-mcp"
    assert_file "$dir/install/atlassian-mcp-run"
    assert_path_missing "$(get_cloned_source_dir "$dir/commands.log")"
  done
}

test_keep_source_preserves_clone_for_debugging() {
  run_installer keep-source \
    --source-repo-url https://github.com/acme/atlassian-mcp.git \
    --keep-source \
    --agents none \
    --enable-jira \
    --jira-base-url https://jira.internal.example.com/jira
  local source_dir
  source_dir="$(get_cloned_source_dir "$TMP_ROOT/keep-source/commands.log")"
  [[ -d "$source_dir" ]] || fail "kept source was removed: $source_dir"
  rm -rf "$source_dir"
}

test_ssh_remote_is_passed_to_git_without_provider_rewrite() {
  run_installer ssh \
    --source-repo-url git@gitlab.internal:tools/atlassian-mcp.git \
    --source-ref main \
    --agents none \
    --enable-jira \
    --jira-base-url https://jira.internal.example.com/jira
  assert_contains "$TMP_ROOT/ssh/commands.log" "git@gitlab.internal:tools/atlassian-mcp.git"
}

test_embedded_credentials_are_rejected_before_git() {
  local dir="$TMP_ROOT/credential-url"
  local fake_bin="$dir/bin"
  mkdir -p "$dir/home" "$dir/install" "$dir/project"
  make_fakes "$fake_bin"
  if FAKE_LOG="$dir/commands.log" HOME="$dir/home" PATH="$fake_bin:/usr/bin:/bin" \
    bash "$INSTALLER" \
      --source-repo-url https://user:pass@github.com/acme/atlassian-mcp.git \
      --agents none \
      --enable-jira \
      --jira-base-url https://jira.internal.example.com/jira \
      --non-interactive >/tmp/installer-credential.out 2>&1; then
    fail "credential URL unexpectedly succeeded"
  fi
  assert_contains /tmp/installer-credential.out "embedded credentials"
  [[ ! -f "$dir/commands.log" ]] || fail "git should not run for credential URL"
}

test_service_base_urls_reject_embedded_credentials() {
  local jira="$TMP_ROOT/jira-credential-url"
  mkdir -p "$jira/home" "$jira/install" "$jira/project"
  make_fakes "$jira/bin"
  if FAKE_LOG="$jira/commands.log" HOME="$jira/home" PATH="$jira/bin:/usr/bin:/bin" \
    bash "$INSTALLER" \
      --binary "$REPO_ROOT/go.mod" \
      --install-dir "$jira/install" \
      --project-dir "$jira/project" \
      --scope project \
      --agents none \
      --enable-jira \
      --jira-base-url https://user:pass@jira.internal.example.com/jira \
      --non-interactive >/tmp/installer-jira-credential.out 2>&1; then
    fail "credential Jira URL unexpectedly succeeded"
  fi
  assert_contains /tmp/installer-jira-credential.out "must not include embedded credentials"

  local bitbucket="$TMP_ROOT/bitbucket-credential-url"
  mkdir -p "$bitbucket/home" "$bitbucket/install" "$bitbucket/project"
  make_fakes "$bitbucket/bin"
  export BITBUCKET_TOKEN_FOR_URL_TEST='token-value'
  if FAKE_LOG="$bitbucket/commands.log" HOME="$bitbucket/home" PATH="$bitbucket/bin:/usr/bin:/bin" \
    bash "$INSTALLER" \
      --binary "$REPO_ROOT/go.mod" \
      --install-dir "$bitbucket/install" \
      --project-dir "$bitbucket/project" \
      --scope project \
      --agents none \
      --enable-bitbucket \
      --bitbucket-base-url https://user:pass@bitbucket.internal.example.com/bitbucket \
      --bitbucket-project-key PRJ \
      --bitbucket-token-env BITBUCKET_TOKEN_FOR_URL_TEST \
      --non-interactive >/tmp/installer-bitbucket-credential.out 2>&1; then
    fail "credential Bitbucket URL unexpectedly succeeded"
  fi
  assert_contains /tmp/installer-bitbucket-credential.out "must not include embedded credentials"
}

test_module_validation_and_non_secret_config() {
  export BITBUCKET_SECRET_ENV='super-secret-token'
  export JIRA_SECRET_ENV='super-secret-password'
  run_installer both \
    --binary "$REPO_ROOT/go.mod" \
    --skip-tests \
    --agents both \
    --enable-jira \
    --jira-base-url https://jira.internal.example.com/jira \
    --jira-username jira-svc \
    --jira-password-env JIRA_SECRET_ENV \
    --enable-bitbucket \
    --bitbucket-base-url https://bitbucket.internal.example.com/bitbucket \
    --bitbucket-project-key PRJ \
    --bitbucket-user-slug svc-atlassian-mcp \
    --bitbucket-token-env BITBUCKET_SECRET_ENV \
    --atlassian-tls-verify true
  local dir="$TMP_ROOT/both"
  # The wrapper (used for Claude/manual runs) only ever holds the indirection variable *names*,
  # resolving actual secret values at its own runtime -- never the resolved values themselves.
  assert_not_contains "$dir/install/atlassian-mcp-run" "super-secret-token"
  assert_not_contains "$dir/install/atlassian-mcp-run" "super-secret-password"
  assert_contains "$dir/install/atlassian-mcp-run" "JIRA_USERNAME"
  assert_contains "$dir/install/atlassian-mcp-run" "\${JIRA_SECRET_ENV}"
  assert_contains "$dir/install/atlassian-mcp-run" "\${BITBUCKET_SECRET_ENV}"
  # Codex does not inherit ambient environment for spawned stdio servers, so its config carries the
  # binary directly plus the resolved values (this is the one deliberate exception to the
  # no-secrets-in-agent-config rule, documented in codex_env_lines).
  assert_contains "$dir/project/.codex/config.toml" "command = \"$dir/install/atlassian-mcp\""
  assert_contains "$dir/project/.codex/config.toml" '[mcp_servers.atlassian.env]'
  assert_contains "$dir/project/.codex/config.toml" 'BITBUCKET_BEARER_TOKEN = "super-secret-token"'
  assert_contains "$dir/project/.codex/config.toml" 'JIRA_USERNAME = "jira-svc"'
  assert_contains "$dir/project/.codex/config.toml" 'JIRA_PASSWORD = "super-secret-password"'
  assert_not_contains "$dir/project/.codex/config.toml" "BITBUCKET_SECRET_ENV"
  assert_not_contains "$dir/project/.codex/config.toml" "JIRA_SECRET_ENV"
  # Claude's config is untouched by any of this and must stay completely secret-free.
  assert_contains "$dir/project/.mcp.json" "\"command\": \"$dir/install/atlassian-mcp-run\""
  assert_not_contains "$dir/project/.mcp.json" "BITBUCKET_SECRET_ENV"
  assert_not_contains "$dir/project/.mcp.json" "JIRA_SECRET_ENV"
  assert_not_contains "$dir/project/.mcp.json" "super-secret-token"
  assert_not_contains "$dir/project/.mcp.json" "super-secret-password"

  local missing="$TMP_ROOT/missing"
  mkdir -p "$missing/home" "$missing/install" "$missing/project"
  make_fakes "$missing/bin"
  if FAKE_LOG="$missing/commands.log" HOME="$missing/home" PATH="$missing/bin:/usr/bin:/bin" \
    bash "$INSTALLER" \
      --binary "$REPO_ROOT/go.mod" \
      --agents none \
      --enable-bitbucket \
      --bitbucket-base-url https://bitbucket.internal.example.com/bitbucket \
      --bitbucket-token-env BITBUCKET_SECRET_ENV \
      --non-interactive >/tmp/installer-missing.out 2>&1; then
    fail "missing Bitbucket project key unexpectedly succeeded"
  fi
  assert_contains /tmp/installer-missing.out "--bitbucket-project-key is required"
}

test_non_interactive_bitbucket_requires_token_env_value() {
  local dir="$TMP_ROOT/missing-token"
  mkdir -p "$dir/home" "$dir/install" "$dir/project"
  make_fakes "$dir/bin"
  unset UNSET_BITBUCKET_TOKEN
  if FAKE_LOG="$dir/commands.log" HOME="$dir/home" PATH="$dir/bin:/usr/bin:/bin" \
    bash "$INSTALLER" \
      --binary "$REPO_ROOT/go.mod" \
      --install-dir "$dir/install" \
      --project-dir "$dir/project" \
      --scope project \
      --agents none \
      --enable-bitbucket \
      --bitbucket-base-url https://bitbucket.internal.example.com/bitbucket \
      --bitbucket-project-key PRJ \
      --bitbucket-token-env UNSET_BITBUCKET_TOKEN \
      --non-interactive >/tmp/installer-missing-token.out 2>&1; then
    fail "missing non-interactive Bitbucket token env unexpectedly succeeded"
  fi
  assert_contains /tmp/installer-missing-token.out "UNSET_BITBUCKET_TOKEN is required"
}

test_jira_username_requires_enable_jira_and_password_env() {
  local no_jira="$TMP_ROOT/jira-username-without-enable"
  mkdir -p "$no_jira/home" "$no_jira/install" "$no_jira/project"
  make_fakes "$no_jira/bin"
  export BITBUCKET_SECRET_ENV='token'
  if FAKE_LOG="$no_jira/commands.log" HOME="$no_jira/home" PATH="$no_jira/bin:/usr/bin:/bin" \
    bash "$INSTALLER" \
      --binary "$REPO_ROOT/go.mod" \
      --install-dir "$no_jira/install" \
      --project-dir "$no_jira/project" \
      --scope project \
      --agents none \
      --enable-bitbucket \
      --bitbucket-base-url https://bitbucket.internal.example.com/bitbucket \
      --bitbucket-project-key PRJ \
      --bitbucket-token-env BITBUCKET_SECRET_ENV \
      --jira-username jira-svc \
      --non-interactive >/tmp/installer-jira-username-no-enable.out 2>&1; then
    fail "--jira-username without --enable-jira unexpectedly succeeded"
  fi
  assert_contains /tmp/installer-jira-username-no-enable.out "--jira-username requires --enable-jira"

  local missing_password="$TMP_ROOT/jira-username-missing-password"
  mkdir -p "$missing_password/home" "$missing_password/install" "$missing_password/project"
  make_fakes "$missing_password/bin"
  unset UNSET_JIRA_PASSWORD
  if FAKE_LOG="$missing_password/commands.log" HOME="$missing_password/home" PATH="$missing_password/bin:/usr/bin:/bin" \
    bash "$INSTALLER" \
      --binary "$REPO_ROOT/go.mod" \
      --install-dir "$missing_password/install" \
      --project-dir "$missing_password/project" \
      --scope project \
      --agents none \
      --enable-jira \
      --jira-base-url https://jira.internal.example.com/jira \
      --jira-username jira-svc \
      --jira-password-env UNSET_JIRA_PASSWORD \
      --non-interactive >/tmp/installer-jira-missing-password.out 2>&1; then
    fail "missing Jira password env was not rejected"
  fi
  assert_contains /tmp/installer-jira-missing-password.out "UNSET_JIRA_PASSWORD is required"
}

test_agent_config_escapes_wrapper_path_for_toml_and_json() {
  local dir="$TMP_ROOT/config-escaping"
  local install_dir="$dir/install \"quote\" \\slash"
  mkdir -p "$dir/home" "$install_dir" "$dir/project"
  make_fakes "$dir/bin"
  FAKE_LOG="$dir/commands.log" HOME="$dir/home" PATH="$dir/bin:/usr/bin:/bin" \
    bash "$INSTALLER" \
      --binary "$REPO_ROOT/go.mod" \
      --install-dir "$install_dir" \
      --project-dir "$dir/project" \
      --scope project \
      --agents both \
      --enable-jira \
      --jira-base-url https://jira.internal.example.com/jira \
      --non-interactive >/tmp/installer-config-escaping.out 2>&1
  assert_contains "$dir/project/.codex/config.toml" 'install \"quote\" \\slash/atlassian-mcp"'
  assert_contains "$dir/project/.mcp.json" 'install \"quote\" \\slash/atlassian-mcp-run'
}

test_piped_installer_without_agents_fails_without_terminal() {
  local dir="$TMP_ROOT/piped-installer"
  mkdir -p "$dir/home" "$dir/install" "$dir/project"
  make_fakes "$dir/bin"
  if { cat "$INSTALLER"; printf '\ncodex\n'; } | FAKE_LOG="$dir/commands.log" HOME="$dir/home" PATH="$dir/bin:/usr/bin:/bin" \
    bash -s -- \
      --binary "$REPO_ROOT/go.mod" \
      --install-dir "$dir/install" \
      --project-dir "$dir/project" \
      --scope project \
      --enable-jira \
      --jira-base-url https://jira.internal.example.com/jira >/tmp/installer-piped.out 2>&1; then
    fail "piped installer unexpectedly consumed stdin as the agent choice"
  fi
  assert_contains /tmp/installer-piped.out "--agents requires a terminal"
  [[ ! -e "$dir/project/.codex/config.toml" ]] || fail "piped installer wrote codex config without terminal agent choice"
}

test_claude_cli_registers_scope_local_and_user() {
  local scope
  for scope in user local; do
    local dir="$TMP_ROOT/claude-cli-$scope"
    mkdir -p "$dir/home" "$dir/install" "$dir/project"
    make_fakes "$dir/bin"
    FAKE_LOG="$dir/commands.log" HOME="$dir/home" PATH="$dir/bin:/usr/bin:/bin" \
      bash "$INSTALLER" \
        --binary "$REPO_ROOT/go.mod" \
        --install-dir "$dir/install" \
        --project-dir "$dir/project" \
        --scope "$scope" \
        --agents claude \
        --enable-jira \
        --jira-base-url https://jira.internal.example.com/jira \
        --non-interactive
    assert_contains "$dir/commands.log" "claude mcp add atlassian --scope $scope --"
    assert_contains "$dir/commands.log" "claude mcp get atlassian --scope $scope"
    [[ ! -e "$dir/home/.claude/settings.json" ]] || fail "installer wrote a Claude settings file for --scope $scope instead of using claude mcp add"
    [[ ! -e "$dir/project/.mcp.json" ]] || fail "installer wrote .mcp.json for --scope $scope instead of using claude mcp add"
  done
}

test_claude_cli_missing_binary_errors_clearly() {
  local dir="$TMP_ROOT/claude-cli-missing"
  mkdir -p "$dir/home" "$dir/install" "$dir/project"
  make_fakes "$dir/bin"
  rm -f "$dir/bin/claude"
  if FAKE_LOG="$dir/commands.log" HOME="$dir/home" PATH="$dir/bin:/usr/bin:/bin" \
    bash "$INSTALLER" \
      --binary "$REPO_ROOT/go.mod" \
      --install-dir "$dir/install" \
      --project-dir "$dir/project" \
      --scope user \
      --agents claude \
      --enable-jira \
      --jira-base-url https://jira.internal.example.com/jira \
      --non-interactive >/tmp/installer-claude-missing.out 2>&1; then
    fail "installer unexpectedly succeeded without claude CLI"
  fi
  assert_contains /tmp/installer-claude-missing.out "claude CLI is required"
}

test_rerun_is_idempotent_and_config_failure_rolls_back() {
  run_installer idem \
    --binary "$REPO_ROOT/go.mod" \
    --agents codex \
    --enable-jira \
    --jira-base-url https://jira.internal.example.com/jira
  run_installer idem \
    --binary "$REPO_ROOT/go.mod" \
    --agents codex \
    --enable-jira \
    --jira-base-url https://jira.internal.example.com/jira
  assert_count "$TMP_ROOT/idem/project/.codex/config.toml" "atlassian-mcp managed block" 2
  assert_count "$TMP_ROOT/idem/project/.codex/config.toml" "command =" 1

  local dir="$TMP_ROOT/rollback"
  mkdir -p "$dir/home" "$dir/install" "$dir/project/.codex"
  make_fakes "$dir/bin"
  printf 'original config\n' >"$dir/project/.codex/config.toml"
  mkdir -p "$dir/project/.mcp.json"
  if FAKE_LOG="$dir/commands.log" HOME="$dir/home" PATH="$dir/bin:/usr/bin:/bin" \
    bash "$INSTALLER" \
      --binary "$REPO_ROOT/go.mod" \
      --install-dir "$dir/install" \
      --project-dir "$dir/project" \
      --scope project \
      --agents both \
      --enable-jira \
      --jira-base-url https://jira.internal.example.com/jira \
      --non-interactive >/tmp/installer-rollback.out 2>&1; then
    fail "config failure unexpectedly succeeded"
  fi
  assert_contains "$dir/project/.codex/config.toml" "original config"
}

test_dry_run_validates_without_side_effects() {
  run_installer dry-run \
    --source-repo-url https://github.com/acme/atlassian-mcp.git \
    --agents both \
    --dry-run \
    --enable-jira \
    --jira-base-url https://jira.internal.example.com/jira
  [[ ! -e "$TMP_ROOT/dry-run/install/atlassian-mcp" ]] || fail "dry-run installed binary"
  [[ ! -e "$TMP_ROOT/dry-run/project/.codex/config.toml" ]] || fail "dry-run wrote codex config"
}

test_final_paths_and_readme_bootstrap_contract() {
  assert_file "$REPO_ROOT/scripts/install-from-remote.sh"
  [[ ! -e "$REPO_ROOT/install-from-remote.sh" ]] || fail "root bash installer should not exist"
  assert_contains "$REPO_ROOT/README.md" "https://raw.githubusercontent.com/chiendao1808/atlassian-mcp/\${INSTALLER_REF}/scripts/install-from-remote.sh"
  assert_contains "$REPO_ROOT/README.md" "curl -fsSL \"\$INSTALLER_URL\" |"
  assert_contains "$REPO_ROOT/README.md" "--source-repo-url https://github.com/chiendao1808/atlassian-mcp.git"
  assert_not_contains "$REPO_ROOT/README.md" "user:password@"
}

for test_name in \
  test_https_remotes_checkout_test_build_and_install_atomically \
  test_keep_source_preserves_clone_for_debugging \
  test_ssh_remote_is_passed_to_git_without_provider_rewrite \
  test_embedded_credentials_are_rejected_before_git \
  test_service_base_urls_reject_embedded_credentials \
  test_module_validation_and_non_secret_config \
  test_non_interactive_bitbucket_requires_token_env_value \
  test_jira_username_requires_enable_jira_and_password_env \
  test_agent_config_escapes_wrapper_path_for_toml_and_json \
  test_piped_installer_without_agents_fails_without_terminal \
  test_claude_cli_registers_scope_local_and_user \
  test_claude_cli_missing_binary_errors_clearly \
  test_rerun_is_idempotent_and_config_failure_rolls_back \
  test_dry_run_validates_without_side_effects \
  test_final_paths_and_readme_bootstrap_contract
do
  "$test_name"
  echo "ok $test_name"
done

echo "PASS install-from-remote_test.sh"
