#!/usr/bin/env bash
set -euo pipefail

PATH="/usr/bin:/bin:$PATH"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
INSTALLER="$REPO_ROOT/scripts/install-from-remote.sh"
TMP_ROOT="$(mktemp -d)"

trap 'rm -rf "$TMP_ROOT"' EXIT

# Resolves a jq binary for the Cursor/Kiro JSON-merge tests. CI (Ubuntu) ships jq on PATH; local
# environments may point JQ_BIN at a static jq. When neither is available, jq-dependent cases are
# skipped (not failed) so the suite still runs on hosts without jq.
JQ_BIN="${JQ_BIN:-$(command -v jq || true)}"

require_jq() {
  if [[ -z "$JQ_BIN" ]]; then
    echo "SKIP (no jq available): $1" >&2
    return 1
  fi
  return 0
}

# Minimal shell harness: run the real installer against fake network/tool boundaries.
fail() {
  echo "FAIL: $*" >&2
  exit 1
}

assert_file() {
  [[ -f "$1" ]] || fail "missing file: $1"
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
  got="$(grep -F -c -- "$text" "$file" 2>/dev/null || :)"
  got="${got:-0}"
  [[ "$got" == "$want" ]] || fail "count for '$text' in $file = $got, want $want"
}

make_release_fixture() {
  local dir="$1"
  local version
  mkdir -p "$dir"
  printf '{"tag_name":"v1.2.3"}\n' >"$dir/latest.json"
  for version in 1.2.3 9.9.9; do
    local asset="atlassian-mcp_${version}_linux_amd64"
    printf '#!/usr/bin/env bash\necho atlassian-mcp %s\n' "$version" >"$dir/$asset"
    chmod +x "$dir/$asset"
    (cd "$dir" && sha256sum "$asset" >"atlassian-mcp_${version}_checksums.txt")
  done
}

# Fake external boundaries. git/go fail loudly because release installs must not build source.
make_fakes() {
  local dir="$1"
  mkdir -p "$dir"
  cat >"$dir/curl" <<'FAKE_CURL'
#!/usr/bin/env bash
set -euo pipefail
echo "curl $*" >>"$FAKE_LOG"
out=""
url=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    -o|--output) out="$2"; shift 2 ;;
    --connect-timeout|--max-time) shift 2 ;;
    -*) shift ;;
    *) url="$1"; shift ;;
  esac
done
name="${url##*/}"
if [[ "$url" == *"/releases/latest" ]]; then
  cat "$FAKE_RELEASE_DIR/latest.json"
  exit 0
fi
if [[ "$name" == *_checksums.txt && "${FAKE_CHECKSUM_MISMATCH:-}" == "1" ]]; then
  printf '0000000000000000000000000000000000000000000000000000000000000000  %s\n' "${name/_checksums.txt/_linux_amd64}" >"$out"
  exit 0
fi
cp "$FAKE_RELEASE_DIR/$name" "$out"
FAKE_CURL
  cat >"$dir/uname" <<'FAKE_UNAME'
#!/usr/bin/env bash
case "${1:-}" in
  -s) printf '%s\n' "${FAKE_UNAME_S:-Linux}" ;;
  -m) printf '%s\n' "${FAKE_UNAME_M:-x86_64}" ;;
  *) printf '%s\n' Linux ;;
esac
FAKE_UNAME
  cat >"$dir/git" <<'FAKE_GIT'
#!/usr/bin/env bash
echo "git $*" >>"$FAKE_LOG"
exit 99
FAKE_GIT
  cat >"$dir/go" <<'FAKE_GO'
#!/usr/bin/env bash
echo "go $*" >>"$FAKE_LOG"
exit 99
FAKE_GO
  cat >"$dir/claude" <<'FAKE_CLAUDE'
#!/usr/bin/env bash
echo "claude $*" >>"$FAKE_LOG"
case "$1 $2" in
  "mcp remove") exit 1 ;;
  *) exit 0 ;;
esac
FAKE_CLAUDE
  chmod +x "$dir/curl" "$dir/uname" "$dir/git" "$dir/go" "$dir/claude"
}

# Builds a bin directory containing the external fakes plus symlinks to the coreutils the
# installer needs, but deliberately WITHOUT jq. Used to prove the jq requirement triggers only
# for Cursor/Kiro. CI ships jq on PATH, so a jq-less PATH must be constructed explicitly.
make_fakes_without_jq() {
  local dir="$1"
  make_fakes "$dir"
  local tool src
  for tool in bash sh cat cp mv rm mkdir chmod mktemp dirname basename grep awk sed sha256sum uname head tr cut sort env printf; do
    src="$(command -v "$tool" 2>/dev/null || true)"
    if [[ -n "$src" && "$tool" != "jq" && ! -e "$dir/$tool" ]]; then
      ln -sf "$src" "$dir/$tool"
    fi
  done
  rm -f "$dir/jq"
}

run_installer() {
  local name="$1"
  shift
  local case_dir="$TMP_ROOT/$name"
  local fake_bin="$case_dir/bin"
  local release_dir="$case_dir/release"
  mkdir -p "$case_dir/home" "$case_dir/install" "$case_dir/project"
  make_fakes "$fake_bin"
  make_release_fixture "$release_dir"
  if [[ -n "$JQ_BIN" ]]; then
    cp "$JQ_BIN" "$fake_bin/jq"
    chmod +x "$fake_bin/jq"
  fi
  FAKE_LOG="$case_dir/commands.log" \
  FAKE_RELEASE_DIR="$release_dir" \
  HOME="$case_dir/home" \
  PATH="$fake_bin:/usr/bin:/bin" \
    bash "$INSTALLER" \
      --install-dir "$case_dir/install" \
      --project-dir "$case_dir/project" \
      --scope project \
      --non-interactive \
      "$@"
}

test_default_release_download_verifies_and_installs_without_source_build() {
  run_installer default-release \
    --agents none \
    --enable-jira \
    --jira-base-url https://jira.internal.example.com/jira
  local dir="$TMP_ROOT/default-release"
  assert_file "$dir/install/atlassian-mcp"
  assert_file "$dir/install/atlassian-mcp-run"
  assert_contains "$dir/install/atlassian-mcp" "atlassian-mcp 1.2.3"
  assert_contains "$dir/commands.log" "releases/latest"
  assert_contains "$dir/commands.log" "atlassian-mcp_1.2.3_linux_amd64"
  assert_contains "$dir/commands.log" "atlassian-mcp_1.2.3_checksums.txt"
  assert_not_contains "$dir/commands.log" "git clone"
  assert_not_contains "$dir/commands.log" "go build"
}

test_release_tag_pins_exact_asset() {
  run_installer pinned-release \
    --release-tag v9.9.9 \
    --agents none \
    --enable-jira \
    --jira-base-url https://jira.internal.example.com/jira
  local dir="$TMP_ROOT/pinned-release"
  assert_contains "$dir/install/atlassian-mcp" "atlassian-mcp 9.9.9"
  assert_contains "$dir/commands.log" "download/v9.9.9/atlassian-mcp_9.9.9_linux_amd64"
}

test_checksum_mismatch_fails_without_replacing_destination() {
  local dir="$TMP_ROOT/checksum-mismatch"
  mkdir -p "$dir/home" "$dir/install" "$dir/project"
  printf 'old binary\n' >"$dir/install/atlassian-mcp"
  make_fakes "$dir/bin"
  make_release_fixture "$dir/release"
  if FAKE_LOG="$dir/commands.log" FAKE_RELEASE_DIR="$dir/release" FAKE_CHECKSUM_MISMATCH=1 HOME="$dir/home" PATH="$dir/bin:/usr/bin:/bin" \
    bash "$INSTALLER" \
      --install-dir "$dir/install" \
      --project-dir "$dir/project" \
      --scope project \
      --agents none \
      --enable-jira \
      --jira-base-url https://jira.internal.example.com/jira \
      --non-interactive >/tmp/installer-checksum-mismatch.out 2>&1; then
    fail "checksum mismatch unexpectedly succeeded"
  fi
  assert_contains /tmp/installer-checksum-mismatch.out "checksum mismatch"
  assert_contains "$dir/install/atlassian-mcp" "old binary"
}

test_unsupported_platform_errors_before_download() {
  local dir="$TMP_ROOT/unsupported-platform"
  mkdir -p "$dir/home" "$dir/install" "$dir/project"
  make_fakes "$dir/bin"
  make_release_fixture "$dir/release"
  if FAKE_LOG="$dir/commands.log" FAKE_RELEASE_DIR="$dir/release" FAKE_UNAME_M=arm64 HOME="$dir/home" PATH="$dir/bin:/usr/bin:/bin" \
    bash "$INSTALLER" \
      --install-dir "$dir/install" \
      --project-dir "$dir/project" \
      --scope project \
      --agents none \
      --enable-jira \
      --jira-base-url https://jira.internal.example.com/jira \
      --non-interactive >/tmp/installer-unsupported.out 2>&1; then
    fail "unsupported platform unexpectedly succeeded"
  fi
  assert_contains /tmp/installer-unsupported.out "unsupported platform"
  [[ ! -f "$dir/commands.log" ]] || assert_not_contains "$dir/commands.log" "curl"
}

test_binary_override_keeps_config_behavior_and_skips_download() {
  export BITBUCKET_SECRET_ENV='super-secret-token'
  export JIRA_SECRET_ENV='super-secret-password'
  export CONFLUENCE_SECRET_ENV='super-secret-confluence-password'
  run_installer binary-config \
    --binary "$REPO_ROOT/go.mod" \
    --agents both \
    --enable-jira \
    --jira-base-url https://jira.internal.example.com/jira \
    --jira-username jira-svc \
    --jira-password-env JIRA_SECRET_ENV \
    --enable-confluence \
    --confluence-base-url https://confluence.internal.example.com/confluence \
    --confluence-username confluence-svc \
    --confluence-password-env CONFLUENCE_SECRET_ENV \
    --enable-bitbucket \
    --bitbucket-base-url https://bitbucket.internal.example.com/bitbucket \
    --bitbucket-project-key PRJ \
    --bitbucket-token-env BITBUCKET_SECRET_ENV
  local dir="$TMP_ROOT/binary-config"
  assert_not_contains "$dir/commands.log" "curl"
  assert_contains "$dir/project/.codex/config.toml" 'BITBUCKET_BEARER_TOKEN = "super-secret-token"'
  assert_contains "$dir/project/.codex/config.toml" 'JIRA_PASSWORD = "super-secret-password"'
  assert_contains "$dir/project/.codex/config.toml" 'CONFLUENCE_PASSWORD = "super-secret-confluence-password"'
  assert_not_contains "$dir/install/atlassian-mcp-run" "super-secret-token"
  assert_not_contains "$dir/project/.mcp.json" "super-secret-token"
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

test_jira_and_confluence_username_require_module_and_password_env() {
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

  local missing_jira="$TMP_ROOT/jira-username-missing-password"
  mkdir -p "$missing_jira/home" "$missing_jira/install" "$missing_jira/project"
  make_fakes "$missing_jira/bin"
  unset UNSET_JIRA_PASSWORD
  if FAKE_LOG="$missing_jira/commands.log" HOME="$missing_jira/home" PATH="$missing_jira/bin:/usr/bin:/bin" \
    bash "$INSTALLER" \
      --binary "$REPO_ROOT/go.mod" \
      --install-dir "$missing_jira/install" \
      --project-dir "$missing_jira/project" \
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

  local no_confluence="$TMP_ROOT/confluence-username-without-enable"
  mkdir -p "$no_confluence/home" "$no_confluence/install" "$no_confluence/project"
  make_fakes "$no_confluence/bin"
  if FAKE_LOG="$no_confluence/commands.log" HOME="$no_confluence/home" PATH="$no_confluence/bin:/usr/bin:/bin" \
    bash "$INSTALLER" \
      --binary "$REPO_ROOT/go.mod" \
      --install-dir "$no_confluence/install" \
      --project-dir "$no_confluence/project" \
      --scope project \
      --agents none \
      --enable-jira \
      --jira-base-url https://jira.internal.example.com/jira \
      --confluence-username confluence-svc \
      --non-interactive >/tmp/installer-confluence-username-no-enable.out 2>&1; then
    fail "--confluence-username without --enable-confluence unexpectedly succeeded"
  fi
  assert_contains /tmp/installer-confluence-username-no-enable.out "--confluence-username requires --enable-confluence"

  local missing_confluence="$TMP_ROOT/confluence-username-missing-password"
  mkdir -p "$missing_confluence/home" "$missing_confluence/install" "$missing_confluence/project"
  make_fakes "$missing_confluence/bin"
  unset UNSET_CONFLUENCE_PASSWORD
  if FAKE_LOG="$missing_confluence/commands.log" HOME="$missing_confluence/home" PATH="$missing_confluence/bin:/usr/bin:/bin" \
    bash "$INSTALLER" \
      --binary "$REPO_ROOT/go.mod" \
      --install-dir "$missing_confluence/install" \
      --project-dir "$missing_confluence/project" \
      --scope project \
      --agents none \
      --enable-confluence \
      --confluence-base-url https://confluence.internal.example.com/confluence \
      --confluence-username confluence-svc \
      --confluence-password-env UNSET_CONFLUENCE_PASSWORD \
      --non-interactive >/tmp/installer-confluence-missing-password.out 2>&1; then
    fail "missing Confluence password env was not rejected"
  fi
  assert_contains /tmp/installer-confluence-missing-password.out "UNSET_CONFLUENCE_PASSWORD is required"
}

test_confluence_wrapper_clears_stale_fixed_env_and_preserves_password_alias() {
  local disabled="$TMP_ROOT/confluence-disabled-wrapper"
  mkdir -p "$disabled/home" "$disabled/install" "$disabled/project" "$disabled/bin"
  make_fakes "$disabled/bin"
  local disabled_binary="$disabled/fake-atlassian-mcp"
  cat >"$disabled_binary" <<'FAKE_BINARY'
#!/usr/bin/env bash
for name in CONFLUENCE_BASE_URL CONFLUENCE_CA_FILE CONFLUENCE_USERNAME CONFLUENCE_PASSWORD; do
  if [[ -n "${!name:-}" ]]; then
    printf '%s=%s\n' "$name" "${!name}"
  fi
done
FAKE_BINARY
  chmod +x "$disabled_binary"
  FAKE_LOG="$disabled/commands.log" HOME="$disabled/home" PATH="$disabled/bin:/usr/bin:/bin" \
    bash "$INSTALLER" \
      --binary "$disabled_binary" \
      --install-dir "$disabled/install" \
      --project-dir "$disabled/project" \
      --scope project \
      --agents none \
      --enable-jira \
      --jira-base-url https://jira.internal.example.com/jira \
      --non-interactive
  local disabled_output
  disabled_output="$(CONFLUENCE_BASE_URL=stale CONFLUENCE_CA_FILE=stale-ca CONFLUENCE_USERNAME=stale-user CONFLUENCE_PASSWORD=stale-password "$disabled/install/atlassian-mcp-run")"
  [[ -z "$disabled_output" ]] || fail "disabled Confluence wrapper leaked stale env: $disabled_output"

  local alias_case="$TMP_ROOT/confluence-password-alias-wrapper"
  mkdir -p "$alias_case/home" "$alias_case/install" "$alias_case/project" "$alias_case/bin"
  make_fakes "$alias_case/bin"
  local alias_binary="$alias_case/fake-atlassian-mcp"
  cp "$disabled_binary" "$alias_binary"
  FAKE_LOG="$alias_case/commands.log" HOME="$alias_case/home" PATH="$alias_case/bin:/usr/bin:/bin" CONFLUENCE_PASSWORD=alias-secret \
    bash "$INSTALLER" \
      --binary "$alias_binary" \
      --install-dir "$alias_case/install" \
      --project-dir "$alias_case/project" \
      --scope project \
      --agents none \
      --enable-confluence \
      --confluence-base-url https://confluence.internal.example.com/confluence \
      --confluence-username confluence-svc \
      --confluence-password-env CONFLUENCE_PASSWORD \
      --non-interactive
  local alias_output
  alias_output="$(CONFLUENCE_PASSWORD=alias-secret "$alias_case/install/atlassian-mcp-run")"
  [[ "$alias_output" == *"CONFLUENCE_USERNAME=confluence-svc"* ]] || fail "Confluence username missing with password alias: $alias_output"
  [[ "$alias_output" == *"CONFLUENCE_PASSWORD=alias-secret"* ]] || fail "Confluence password alias was unset before use: $alias_output"
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
    --agents both \
    --dry-run \
    --enable-jira \
    --jira-base-url https://jira.internal.example.com/jira
  [[ ! -e "$TMP_ROOT/dry-run/install/atlassian-mcp" ]] || fail "dry-run installed binary"
  [[ ! -e "$TMP_ROOT/dry-run/project/.codex/config.toml" ]] || fail "dry-run wrote codex config"
}

test_agent_selection_contract() {
  local valid
  # Selections that do not require jq run on every host.
  for valid in both none claude codex claude,codex; do
    local dir="$TMP_ROOT/sel-valid-$valid"
    mkdir -p "$dir/home" "$dir/install" "$dir/project"
    make_fakes "$dir/bin"
    FAKE_LOG="$dir/commands.log" HOME="$dir/home" PATH="$dir/bin:/usr/bin:/bin" \
      bash "$INSTALLER" \
        --binary "$REPO_ROOT/go.mod" \
        --install-dir "$dir/install" \
        --project-dir "$dir/project" \
        --scope project \
        --agents "$valid" \
        --enable-jira \
        --jira-base-url https://jira.internal.example.com/jira \
        --non-interactive >/tmp/installer-sel-valid.out 2>&1 ||
      fail "--agents $valid unexpectedly failed: $(cat /tmp/installer-sel-valid.out)"
  done

  # both remains Claude + Codex only and never selects Cursor or Kiro.
  local both_dir="$TMP_ROOT/sel-valid-both"
  assert_file "$both_dir/project/.codex/config.toml"
  assert_file "$both_dir/project/.mcp.json"
  [[ ! -e "$both_dir/project/.cursor" ]] || fail "both created Cursor config"
  [[ ! -e "$both_dir/project/.kiro" ]] || fail "both created Kiro config"

  # claude,codex selects Claude and Codex but not Cursor or Kiro.
  local cc_dir="$TMP_ROOT/sel-valid-claude,codex"
  assert_file "$cc_dir/project/.codex/config.toml"
  assert_file "$cc_dir/project/.mcp.json"
  [[ ! -e "$cc_dir/project/.cursor" ]] || fail "claude,codex created Cursor config"

  # none registers no agent config.
  local none_dir="$TMP_ROOT/sel-valid-none"
  [[ ! -e "$none_dir/project/.codex/config.toml" ]] || fail "none created Codex config"
  [[ ! -e "$none_dir/project/.mcp.json" ]] || fail "none created Claude config"

  # Selections that include Cursor/Kiro require jq; skip them when jq is unavailable.
  if require_jq "cursor/kiro selection cases"; then
    for valid in cursor kiro cursor,kiro claude,cursor claude,codex,cursor,kiro all cursor,cursor; do
      local dir="$TMP_ROOT/sel-valid-$valid"
      mkdir -p "$dir/home" "$dir/install" "$dir/project"
      make_fakes "$dir/bin"
      cp "$JQ_BIN" "$dir/bin/jq"
      chmod +x "$dir/bin/jq"
      FAKE_LOG="$dir/commands.log" HOME="$dir/home" PATH="$dir/bin:/usr/bin:/bin" \
        bash "$INSTALLER" \
          --binary "$REPO_ROOT/go.mod" \
          --install-dir "$dir/install" \
          --project-dir "$dir/project" \
          --scope project \
          --agents "$valid" \
          --enable-jira \
          --jira-base-url https://jira.internal.example.com/jira \
          --non-interactive >/tmp/installer-sel-valid.out 2>&1 ||
        fail "--agents $valid unexpectedly failed: $(cat /tmp/installer-sel-valid.out)"
    done
    # all selects all four agents.
    local all_dir="$TMP_ROOT/sel-valid-all"
    assert_file "$all_dir/project/.codex/config.toml"
    assert_file "$all_dir/project/.mcp.json"
    assert_file "$all_dir/project/.cursor/mcp.json"
    assert_file "$all_dir/project/.kiro/settings/mcp.json"

    # claude,cursor selects Claude and Cursor but not Codex.
    local mixed_dir="$TMP_ROOT/sel-valid-claude,cursor"
    assert_file "$mixed_dir/project/.mcp.json"
    assert_file "$mixed_dir/project/.cursor/mcp.json"
    [[ ! -e "$mixed_dir/project/.codex/config.toml" ]] || fail "claude,cursor created Codex config"

    # cursor,cursor deduplicates to a single Cursor registration.
    local dup_dir="$TMP_ROOT/sel-valid-cursor,cursor"
    assert_file "$dup_dir/project/.cursor/mcp.json"
    assert_count "$dup_dir/project/.cursor/mcp.json" '"atlassian":' 1
  fi

  local invalid
  for invalid in invalid none,cursor all,cursor both,kiro cursor, ""; do
    local dir="$TMP_ROOT/sel-invalid-${invalid:-empty}"
    mkdir -p "$dir/home" "$dir/install" "$dir/project"
    make_fakes "$dir/bin"
    if FAKE_LOG="$dir/commands.log" HOME="$dir/home" PATH="$dir/bin:/usr/bin:/bin" \
      bash "$INSTALLER" \
        --binary "$REPO_ROOT/go.mod" \
        --install-dir "$dir/install" \
        --project-dir "$dir/project" \
        --scope project \
        --agents "$invalid" \
        --enable-jira \
        --jira-base-url https://jira.internal.example.com/jira \
        --non-interactive >/tmp/installer-sel-invalid.out 2>&1; then
      fail "--agents '$invalid' unexpectedly succeeded"
    fi
    [[ ! -e "$dir/install/atlassian-mcp" ]] || fail "--agents '$invalid' installed before validation"
    [[ ! -e "$dir/project/.codex/config.toml" ]] || fail "--agents '$invalid' wrote config before validation"
  done
  if FAKE_LOG="$TMP_ROOT/sel-msg/commands.log" HOME="$TMP_ROOT" PATH="/usr/bin:/bin" \
    bash "$INSTALLER" \
      --binary "$REPO_ROOT/go.mod" \
      --install-dir "$TMP_ROOT/sel-msg" \
      --project-dir "$TMP_ROOT/sel-msg" \
      --scope project \
      --agents invalid \
      --enable-jira \
      --jira-base-url https://jira.internal.example.com/jira \
      --non-interactive >/tmp/installer-sel-msg.out 2>&1; then
    fail "--agents invalid unexpectedly succeeded"
  fi
  assert_contains /tmp/installer-sel-msg.out "must be one of: claude, codex, cursor, kiro"
}

test_cursor_config_paths_and_scope_mapping() {
  require_jq "cursor scope mapping" || return 0
  local scope
  for scope in user local project; do
    local dir="$TMP_ROOT/cursor-scope-$scope"
    mkdir -p "$dir/home" "$dir/install" "$dir/project"
    make_fakes "$dir/bin"
    cp "$JQ_BIN" "$dir/bin/jq"
    chmod +x "$dir/bin/jq"
    FAKE_LOG="$dir/commands.log" HOME="$dir/home" PATH="$dir/bin:/usr/bin:/bin" \
      bash "$INSTALLER" \
        --binary "$REPO_ROOT/go.mod" \
        --install-dir "$dir/install" \
        --project-dir "$dir/project" \
        --scope "$scope" \
        --agents cursor \
        --enable-jira \
        --jira-base-url https://jira.internal.example.com/jira \
        --non-interactive >/tmp/installer-cursor-scope.out 2>&1 ||
      fail "cursor --scope $scope unexpectedly failed: $(cat /tmp/installer-cursor-scope.out)"
    case "$scope" in
      user)
        assert_file "$dir/home/.cursor/mcp.json"
        [[ ! -e "$dir/project/.cursor" ]] || fail "cursor user scope wrote project config"
        ;;
      *)
        assert_file "$dir/project/.cursor/mcp.json"
        [[ ! -e "$dir/home/.cursor" ]] || fail "cursor --scope $scope wrote home config"
        ;;
    esac
  done
}

test_cursor_merge_preserves_existing_json() {
  require_jq "cursor merge preservation" || return 0
  local dir="$TMP_ROOT/cursor-merge"
  mkdir -p "$dir/home" "$dir/install" "$dir/project/.cursor"
  make_fakes "$dir/bin"
  cp "$JQ_BIN" "$dir/bin/jq"
  chmod +x "$dir/bin/jq"
  cat >"$dir/project/.cursor/mcp.json" <<'JSON'
{
  "mcpServers": {
    "existing": {
      "command": "existing-server"
    }
  },
  "customSetting": true
}
JSON
  FAKE_LOG="$dir/commands.log" HOME="$dir/home" PATH="$dir/bin:/usr/bin:/bin" \
    bash "$INSTALLER" \
      --binary "$REPO_ROOT/go.mod" \
      --install-dir "$dir/install" \
      --project-dir "$dir/project" \
      --scope project \
      --agents cursor \
      --enable-jira \
      --jira-base-url https://jira.internal.example.com/jira \
      --non-interactive >/tmp/installer-cursor-merge.out 2>&1 ||
    fail "cursor merge unexpectedly failed: $(cat /tmp/installer-cursor-merge.out)"
  local cfg="$dir/project/.cursor/mcp.json"
  assert_contains "$cfg" '"existing"'
  assert_contains "$cfg" '"customSetting"'
  assert_contains "$cfg" '"atlassian"'
  assert_contains "$cfg" '"type": "stdio"'
  assert_contains "$cfg" 'atlassian-mcp-run'
  "$JQ_BIN" -e '.mcpServers.atlassian.args == []' "$cfg" >/dev/null || fail "cursor args is not an empty array"
  "$JQ_BIN" -e '.mcpServers.existing.command == "existing-server"' "$cfg" >/dev/null || fail "existing server entry changed"
  "$JQ_BIN" -e '.customSetting == true' "$cfg" >/dev/null || fail "unrelated root key changed"
}

test_cursor_conflict_replace_and_idempotency() {
  require_jq "cursor conflict and idempotency" || return 0
  local dir="$TMP_ROOT/cursor-conflict"
  mkdir -p "$dir/home" "$dir/install" "$dir/project/.cursor"
  make_fakes "$dir/bin"
  cp "$JQ_BIN" "$dir/bin/jq"
  chmod +x "$dir/bin/jq"
  cat >"$dir/project/.cursor/mcp.json" <<'JSON'
{
  "mcpServers": {
    "atlassian": {
      "type": "stdio",
      "command": "/somewhere/else",
      "args": []
    }
  },
  "customSetting": true
}
JSON
  if FAKE_LOG="$dir/commands.log" HOME="$dir/home" PATH="$dir/bin:/usr/bin:/bin" \
    bash "$INSTALLER" \
      --binary "$REPO_ROOT/go.mod" \
      --install-dir "$dir/install" \
      --project-dir "$dir/project" \
      --scope project \
      --agents cursor \
      --enable-jira \
      --jira-base-url https://jira.internal.example.com/jira \
      --non-interactive >/tmp/installer-cursor-conflict.out 2>&1; then
    fail "cursor conflict without --replace unexpectedly succeeded"
  fi
  assert_contains /tmp/installer-cursor-conflict.out "use --replace"
  assert_contains "$dir/project/.cursor/mcp.json" "/somewhere/else"

  FAKE_LOG="$dir/commands.log" HOME="$dir/home" PATH="$dir/bin:/usr/bin:/bin" \
    bash "$INSTALLER" \
      --binary "$REPO_ROOT/go.mod" \
      --install-dir "$dir/install" \
      --project-dir "$dir/project" \
      --scope project \
      --agents cursor \
      --replace \
      --enable-jira \
      --jira-base-url https://jira.internal.example.com/jira \
      --non-interactive >/tmp/installer-cursor-replace.out 2>&1 ||
    fail "cursor replace unexpectedly failed: $(cat /tmp/installer-cursor-replace.out)"
  assert_not_contains "$dir/project/.cursor/mcp.json" "/somewhere/else"
  assert_contains "$dir/project/.cursor/mcp.json" '"customSetting"'
  "$JQ_BIN" -e '.customSetting == true' "$dir/project/.cursor/mcp.json" >/dev/null || fail "replace dropped unrelated root key"

  # Re-running the identical install is idempotent and does not duplicate the entry.
  FAKE_LOG="$dir/commands.log" HOME="$dir/home" PATH="$dir/bin:/usr/bin:/bin" \
    bash "$INSTALLER" \
      --binary "$REPO_ROOT/go.mod" \
      --install-dir "$dir/install" \
      --project-dir "$dir/project" \
      --scope project \
      --agents cursor \
      --enable-jira \
      --jira-base-url https://jira.internal.example.com/jira \
      --non-interactive >/tmp/installer-cursor-idem.out 2>&1 ||
    fail "cursor reinstall unexpectedly failed: $(cat /tmp/installer-cursor-idem.out)"
  assert_count "$dir/project/.cursor/mcp.json" '"atlassian"' 1
}

test_cursor_rejects_malformed_existing_config() {
  require_jq "cursor malformed config" || return 0
  local case_name cfg
  for case_name in malformed root-array servers-array directory; do
    local dir="$TMP_ROOT/cursor-bad-$case_name"
    mkdir -p "$dir/home" "$dir/install" "$dir/project/.cursor"
    make_fakes "$dir/bin"
    cp "$JQ_BIN" "$dir/bin/jq"
    chmod +x "$dir/bin/jq"
    cfg="$dir/project/.cursor/mcp.json"
    case "$case_name" in
      malformed) printf '{not json' >"$cfg" ;;
      root-array) printf '[]' >"$cfg" ;;
      servers-array) printf '{"mcpServers": []}' >"$cfg" ;;
      directory) mkdir -p "$cfg" ;;
    esac
    if FAKE_LOG="$dir/commands.log" HOME="$dir/home" PATH="$dir/bin:/usr/bin:/bin" \
      bash "$INSTALLER" \
        --binary "$REPO_ROOT/go.mod" \
        --install-dir "$dir/install" \
        --project-dir "$dir/project" \
        --scope project \
        --agents cursor \
        --enable-jira \
        --jira-base-url https://jira.internal.example.com/jira \
        --non-interactive >/tmp/installer-cursor-bad.out 2>&1; then
      fail "cursor with $case_name config unexpectedly succeeded"
    fi
  done
}

test_cursor_command_escaping() {
  require_jq "cursor command escaping" || return 0
  local dir="$TMP_ROOT/cursor-escaping"
  local install_dir="$dir/install \"quote\" \\slash with space"
  mkdir -p "$dir/home" "$install_dir" "$dir/project"
  make_fakes "$dir/bin"
  cp "$JQ_BIN" "$dir/bin/jq"
  chmod +x "$dir/bin/jq"
  FAKE_LOG="$dir/commands.log" HOME="$dir/home" PATH="$dir/bin:/usr/bin:/bin" \
    bash "$INSTALLER" \
      --binary "$REPO_ROOT/go.mod" \
      --install-dir "$install_dir" \
      --project-dir "$dir/project" \
      --scope project \
      --agents cursor \
      --enable-jira \
      --jira-base-url https://jira.internal.example.com/jira \
      --non-interactive >/tmp/installer-cursor-escaping.out 2>&1 ||
    fail "cursor escaping install unexpectedly failed: $(cat /tmp/installer-cursor-escaping.out)"
  local cfg="$dir/project/.cursor/mcp.json"
  "$JQ_BIN" -e '.mcpServers.atlassian.command | endswith("atlassian-mcp-run")' "$cfg" >/dev/null ||
    fail "cursor command does not point at the wrapper"
  "$JQ_BIN" -r '.mcpServers.atlassian.command' "$cfg" >"$dir/command.txt"
  assert_contains "$dir/command.txt" "$install_dir/atlassian-mcp-run"
}

test_kiro_config_paths_and_scope_mapping() {
  require_jq "kiro scope mapping" || return 0
  local scope
  for scope in user local project; do
    local dir="$TMP_ROOT/kiro-scope-$scope"
    mkdir -p "$dir/home" "$dir/install" "$dir/project"
    make_fakes "$dir/bin"
    cp "$JQ_BIN" "$dir/bin/jq"
    chmod +x "$dir/bin/jq"
    FAKE_LOG="$dir/commands.log" HOME="$dir/home" PATH="$dir/bin:/usr/bin:/bin" \
      bash "$INSTALLER" \
        --binary "$REPO_ROOT/go.mod" \
        --install-dir "$dir/install" \
        --project-dir "$dir/project" \
        --scope "$scope" \
        --agents kiro \
        --enable-jira \
        --jira-base-url https://jira.internal.example.com/jira \
        --non-interactive >/tmp/installer-kiro-scope.out 2>&1 ||
      fail "kiro --scope $scope unexpectedly failed: $(cat /tmp/installer-kiro-scope.out)"
    case "$scope" in
      user)
        assert_file "$dir/home/.kiro/settings/mcp.json"
        [[ ! -e "$dir/project/.kiro" ]] || fail "kiro user scope wrote project config"
        ;;
      *)
        assert_file "$dir/project/.kiro/settings/mcp.json"
        [[ ! -e "$dir/home/.kiro" ]] || fail "kiro --scope $scope wrote home config"
        ;;
    esac
  done
}

test_kiro_merge_preserves_existing_json() {
  require_jq "kiro merge preservation" || return 0
  local dir="$TMP_ROOT/kiro-merge"
  mkdir -p "$dir/home" "$dir/install" "$dir/project/.kiro/settings"
  make_fakes "$dir/bin"
  cp "$JQ_BIN" "$dir/bin/jq"
  chmod +x "$dir/bin/jq"
  cat >"$dir/project/.kiro/settings/mcp.json" <<'JSON'
{
  "mcpServers": {
    "another-server": {
      "command": "existing"
    }
  },
  "customSetting": true
}
JSON
  FAKE_LOG="$dir/commands.log" HOME="$dir/home" PATH="$dir/bin:/usr/bin:/bin" \
    bash "$INSTALLER" \
      --binary "$REPO_ROOT/go.mod" \
      --install-dir "$dir/install" \
      --project-dir "$dir/project" \
      --scope project \
      --agents kiro \
      --enable-jira \
      --jira-base-url https://jira.internal.example.com/jira \
      --non-interactive >/tmp/installer-kiro-merge.out 2>&1 ||
    fail "kiro merge unexpectedly failed: $(cat /tmp/installer-kiro-merge.out)"
  local cfg="$dir/project/.kiro/settings/mcp.json"
  "$JQ_BIN" -e '.mcpServers["another-server"].command == "existing"' "$cfg" >/dev/null || fail "another-server entry changed"
  "$JQ_BIN" -e '.customSetting == true' "$cfg" >/dev/null || fail "unrelated root key changed"
  "$JQ_BIN" -e '.mcpServers.atlassian.disabled == false' "$cfg" >/dev/null || fail "kiro disabled is not false"
  "$JQ_BIN" -e '.mcpServers.atlassian.args == []' "$cfg" >/dev/null || fail "kiro args is not an empty array"
  "$JQ_BIN" -e '.mcpServers.atlassian.command | endswith("atlassian-mcp-run")' "$cfg" >/dev/null || fail "kiro command does not point at the wrapper"
  "$JQ_BIN" -e '.mcpServers.atlassian | has("autoApprove") | not' "$cfg" >/dev/null || fail "kiro entry must not set autoApprove"
  "$JQ_BIN" -e '.mcpServers.atlassian | has("type") | not' "$cfg" >/dev/null || fail "kiro entry must not set type"
}

test_kiro_conflict_replace_and_idempotency() {
  require_jq "kiro conflict and idempotency" || return 0
  local dir="$TMP_ROOT/kiro-conflict"
  mkdir -p "$dir/home" "$dir/install" "$dir/project/.kiro/settings"
  make_fakes "$dir/bin"
  cp "$JQ_BIN" "$dir/bin/jq"
  chmod +x "$dir/bin/jq"
  cat >"$dir/project/.kiro/settings/mcp.json" <<'JSON'
{
  "mcpServers": {
    "atlassian": {
      "command": "/somewhere/else",
      "args": [],
      "disabled": true
    }
  },
  "customSetting": true
}
JSON
  if FAKE_LOG="$dir/commands.log" HOME="$dir/home" PATH="$dir/bin:/usr/bin:/bin" \
    bash "$INSTALLER" \
      --binary "$REPO_ROOT/go.mod" \
      --install-dir "$dir/install" \
      --project-dir "$dir/project" \
      --scope project \
      --agents kiro \
      --enable-jira \
      --jira-base-url https://jira.internal.example.com/jira \
      --non-interactive >/tmp/installer-kiro-conflict.out 2>&1; then
    fail "kiro conflict without --replace unexpectedly succeeded"
  fi
  assert_contains /tmp/installer-kiro-conflict.out "use --replace"
  assert_contains "$dir/project/.kiro/settings/mcp.json" "/somewhere/else"

  FAKE_LOG="$dir/commands.log" HOME="$dir/home" PATH="$dir/bin:/usr/bin:/bin" \
    bash "$INSTALLER" \
      --binary "$REPO_ROOT/go.mod" \
      --install-dir "$dir/install" \
      --project-dir "$dir/project" \
      --scope project \
      --agents kiro \
      --replace \
      --enable-jira \
      --jira-base-url https://jira.internal.example.com/jira \
      --non-interactive >/tmp/installer-kiro-replace.out 2>&1 ||
    fail "kiro replace unexpectedly failed: $(cat /tmp/installer-kiro-replace.out)"
  assert_not_contains "$dir/project/.kiro/settings/mcp.json" "/somewhere/else"
  "$JQ_BIN" -e '.customSetting == true' "$dir/project/.kiro/settings/mcp.json" >/dev/null || fail "replace dropped unrelated root key"

  FAKE_LOG="$dir/commands.log" HOME="$dir/home" PATH="$dir/bin:/usr/bin:/bin" \
    bash "$INSTALLER" \
      --binary "$REPO_ROOT/go.mod" \
      --install-dir "$dir/install" \
      --project-dir "$dir/project" \
      --scope project \
      --agents kiro \
      --enable-jira \
      --jira-base-url https://jira.internal.example.com/jira \
      --non-interactive >/tmp/installer-kiro-idem.out 2>&1 ||
    fail "kiro reinstall unexpectedly failed: $(cat /tmp/installer-kiro-idem.out)"
  assert_count "$dir/project/.kiro/settings/mcp.json" '"atlassian":' 1
}

test_kiro_only_install_does_not_touch_other_agents() {
  require_jq "kiro isolation" || return 0
  local dir="$TMP_ROOT/kiro-only"
  mkdir -p "$dir/home" "$dir/install" "$dir/project"
  make_fakes "$dir/bin"
  cp "$JQ_BIN" "$dir/bin/jq"
  chmod +x "$dir/bin/jq"
  FAKE_LOG="$dir/commands.log" HOME="$dir/home" PATH="$dir/bin:/usr/bin:/bin" \
    bash "$INSTALLER" \
      --binary "$REPO_ROOT/go.mod" \
      --install-dir "$dir/install" \
      --project-dir "$dir/project" \
      --scope project \
      --agents kiro \
      --enable-jira \
      --jira-base-url https://jira.internal.example.com/jira \
      --non-interactive >/tmp/installer-kiro-only.out 2>&1 ||
    fail "kiro-only install unexpectedly failed: $(cat /tmp/installer-kiro-only.out)"
  assert_file "$dir/project/.kiro/settings/mcp.json"
  [[ ! -e "$dir/project/.cursor" ]] || fail "kiro-only install created Cursor config"
  [[ ! -e "$dir/project/.codex" ]] || fail "kiro-only install created Codex config"
  [[ ! -e "$dir/project/.mcp.json" ]] || fail "kiro-only install created Claude config"
  assert_not_contains "$dir/commands.log" "claude mcp"
}

test_multi_agent_failure_rolls_back_prior_writes() {
  require_jq "multi-agent rollback" || return 0
  # Cursor config is a valid existing file; Kiro target is a directory. The installer must fail,
  # restore the Cursor file byte-for-byte, and leave no partial Kiro config or temp/backup files.
  local dir="$TMP_ROOT/rollback-cursor-kiro"
  mkdir -p "$dir/home" "$dir/install" "$dir/project/.cursor" "$dir/project/.kiro/settings/mcp.json"
  make_fakes "$dir/bin"
  cp "$JQ_BIN" "$dir/bin/jq"
  chmod +x "$dir/bin/jq"
  printf '{"mcpServers": {"keep": {"command": "keep-me"}}}\n' >"$dir/project/.cursor/mcp.json"
  local original
  original="$(cat "$dir/project/.cursor/mcp.json")"
  if FAKE_LOG="$dir/commands.log" HOME="$dir/home" PATH="$dir/bin:/usr/bin:/bin" \
    bash "$INSTALLER" \
      --binary "$REPO_ROOT/go.mod" \
      --install-dir "$dir/install" \
      --project-dir "$dir/project" \
      --scope project \
      --agents cursor,kiro \
      --enable-jira \
      --jira-base-url https://jira.internal.example.com/jira \
      --non-interactive >/tmp/installer-rollback-ck.out 2>&1; then
    fail "cursor,kiro with directory Kiro target unexpectedly succeeded"
  fi
  [[ "$(cat "$dir/project/.cursor/mcp.json")" == "$original" ]] || fail "Cursor config was not restored to its original content"
  local leftover
  leftover="$(find "$dir/project" -name '*.bak.*' -o -name '*.tmp.*' 2>/dev/null)"
  [[ -z "$leftover" ]] || fail "leftover backup/tmp files after rollback: $leftover"

  # all: a Kiro failure must roll back the preceding Codex, Claude, and Cursor writes.
  local all_dir="$TMP_ROOT/rollback-all"
  mkdir -p "$all_dir/home" "$all_dir/install" "$all_dir/project/.cursor" "$all_dir/project/.kiro/settings/mcp.json"
  make_fakes "$all_dir/bin"
  cp "$JQ_BIN" "$all_dir/bin/jq"
  chmod +x "$all_dir/bin/jq"
  printf '{"mcpServers": {"keep": {"command": "keep-me"}}}\n' >"$all_dir/project/.cursor/mcp.json"
  mkdir -p "$all_dir/project/.codex"
  printf 'original codex config\n' >"$all_dir/project/.codex/config.toml"
  if FAKE_LOG="$all_dir/commands.log" HOME="$all_dir/home" PATH="$all_dir/bin:/usr/bin:/bin" \
    bash "$INSTALLER" \
      --binary "$REPO_ROOT/go.mod" \
      --install-dir "$all_dir/install" \
      --project-dir "$all_dir/project" \
      --scope project \
      --agents all \
      --enable-jira \
      --jira-base-url https://jira.internal.example.com/jira \
      --non-interactive >/tmp/installer-rollback-all.out 2>&1; then
    fail "all with directory Kiro target unexpectedly succeeded"
  fi
  assert_contains "$all_dir/project/.codex/config.toml" "original codex config"
  [[ ! -e "$all_dir/project/.mcp.json" ]] || fail "Claude config was not removed after rollback"
  [[ "$(cat "$all_dir/project/.cursor/mcp.json")" == '{"mcpServers": {"keep": {"command": "keep-me"}}}' ]] || fail "Cursor config was not restored after all rollback"
  leftover="$(find "$all_dir/project" -name '*.bak.*' -o -name '*.tmp.*' 2>/dev/null)"
  [[ -z "$leftover" ]] || fail "leftover backup/tmp files after all rollback: $leftover"
}

test_dry_run_all_creates_no_cursor_or_kiro_config() {
  require_jq "dry-run all" || return 0
  local dir="$TMP_ROOT/dry-run-all"
  mkdir -p "$dir/home" "$dir/install" "$dir/project"
  make_fakes "$dir/bin"
  cp "$JQ_BIN" "$dir/bin/jq"
  chmod +x "$dir/bin/jq"
  FAKE_LOG="$dir/commands.log" HOME="$dir/home" PATH="$dir/bin:/usr/bin:/bin" \
    bash "$INSTALLER" \
      --binary "$REPO_ROOT/go.mod" \
      --install-dir "$dir/install" \
      --project-dir "$dir/project" \
      --scope project \
      --agents all \
      --dry-run \
      --enable-jira \
      --jira-base-url https://jira.internal.example.com/jira \
      --non-interactive >/tmp/installer-dry-run-all.out 2>&1 ||
    fail "dry-run all unexpectedly failed: $(cat /tmp/installer-dry-run-all.out)"
  [[ ! -e "$dir/project/.cursor" ]] || fail "dry-run created Cursor config"
  [[ ! -e "$dir/project/.kiro" ]] || fail "dry-run created Kiro config"
  [[ ! -e "$dir/project/.codex" ]] || fail "dry-run created Codex config"
  [[ ! -e "$dir/install/atlassian-mcp" ]] || fail "dry-run installed binary"
}

test_cursor_and_kiro_configs_contain_no_secrets() {
  require_jq "secret leakage" || return 0
  export BITBUCKET_SECRET_ENV='super-secret-token'
  export JIRA_SECRET_ENV='super-secret-password'
  export CONFLUENCE_SECRET_ENV='super-secret-confluence-password'
  local dir="$TMP_ROOT/secrets-json"
  mkdir -p "$dir/home" "$dir/install" "$dir/project"
  make_fakes "$dir/bin"
  cp "$JQ_BIN" "$dir/bin/jq"
  chmod +x "$dir/bin/jq"
  FAKE_LOG="$dir/commands.log" HOME="$dir/home" PATH="$dir/bin:/usr/bin:/bin" \
    bash "$INSTALLER" \
      --binary "$REPO_ROOT/go.mod" \
      --install-dir "$dir/install" \
      --project-dir "$dir/project" \
      --scope project \
      --agents all \
      --enable-jira \
      --jira-base-url https://jira.internal.example.com/jira \
      --jira-username jira-svc \
      --jira-password-env JIRA_SECRET_ENV \
      --enable-confluence \
      --confluence-base-url https://confluence.internal.example.com/confluence \
      --confluence-username confluence-svc \
      --confluence-password-env CONFLUENCE_SECRET_ENV \
      --enable-bitbucket \
      --bitbucket-base-url https://bitbucket.internal.example.com/bitbucket \
      --bitbucket-project-key PRJ \
      --bitbucket-token-env BITBUCKET_SECRET_ENV \
      --non-interactive >/tmp/installer-secrets-json.out 2>&1 ||
    fail "all-agents install with secrets unexpectedly failed: $(cat /tmp/installer-secrets-json.out)"
  local cfg
  for cfg in "$dir/project/.cursor/mcp.json" "$dir/project/.kiro/settings/mcp.json" "$dir/install/atlassian-mcp-run"; do
    assert_not_contains "$cfg" "super-secret-token"
    assert_not_contains "$cfg" "super-secret-password"
    assert_not_contains "$cfg" "super-secret-confluence-password"
  done
  # Codex remains the deliberate exception that carries resolved secrets.
  assert_contains "$dir/project/.codex/config.toml" 'BITBUCKET_BEARER_TOKEN = "super-secret-token"'
}

test_cursor_only_install_skips_claude_and_codex() {
  require_jq "cursor isolation" || return 0
  local dir="$TMP_ROOT/cursor-only"
  mkdir -p "$dir/home" "$dir/install" "$dir/project"
  make_fakes "$dir/bin"
  cp "$JQ_BIN" "$dir/bin/jq"
  chmod +x "$dir/bin/jq"
  FAKE_LOG="$dir/commands.log" HOME="$dir/home" PATH="$dir/bin:/usr/bin:/bin" \
    bash "$INSTALLER" \
      --binary "$REPO_ROOT/go.mod" \
      --install-dir "$dir/install" \
      --project-dir "$dir/project" \
      --scope project \
      --agents cursor \
      --enable-jira \
      --jira-base-url https://jira.internal.example.com/jira \
      --non-interactive >/tmp/installer-cursor-only.out 2>&1 ||
    fail "cursor-only install unexpectedly failed: $(cat /tmp/installer-cursor-only.out)"
  assert_file "$dir/project/.cursor/mcp.json"
  [[ ! -e "$dir/project/.codex" ]] || fail "cursor-only install created Codex config"
  [[ ! -e "$dir/project/.mcp.json" ]] || fail "cursor-only install created Claude config"
  [[ ! -e "$dir/project/.kiro" ]] || fail "cursor-only install created Kiro config"
  assert_not_contains "$dir/commands.log" "claude mcp"
}

test_jq_required_only_for_cursor_or_kiro() {
  # Missing jq with cursor selected fails with a clear error before any install side effect.
  local dir="$TMP_ROOT/jq-missing-cursor"
  mkdir -p "$dir/home" "$dir/install" "$dir/project"
  make_fakes_without_jq "$dir/bin"
  if FAKE_LOG="$dir/commands.log" HOME="$dir/home" PATH="$dir/bin" \
    bash "$INSTALLER" \
      --binary "$REPO_ROOT/go.mod" \
      --install-dir "$dir/install" \
      --project-dir "$dir/project" \
      --scope project \
      --agents cursor \
      --enable-jira \
      --jira-base-url https://jira.internal.example.com/jira \
      --non-interactive >/tmp/installer-jq-missing.out 2>&1; then
    fail "cursor without jq unexpectedly succeeded"
  fi
  assert_contains /tmp/installer-jq-missing.out "jq is required"
  [[ ! -e "$dir/install/atlassian-mcp" ]] || fail "missing jq still installed the binary"

  # Claude/Codex-only selections must not require jq even when it is absent.
  local ok_dir="$TMP_ROOT/jq-missing-claude-codex"
  mkdir -p "$ok_dir/home" "$ok_dir/install" "$ok_dir/project"
  make_fakes_without_jq "$ok_dir/bin"
  FAKE_LOG="$ok_dir/commands.log" HOME="$ok_dir/home" PATH="$ok_dir/bin" \
    bash "$INSTALLER" \
      --binary "$REPO_ROOT/go.mod" \
      --install-dir "$ok_dir/install" \
      --project-dir "$ok_dir/project" \
      --scope project \
      --agents both \
      --enable-jira \
      --jira-base-url https://jira.internal.example.com/jira \
      --non-interactive >/tmp/installer-jq-ok.out 2>&1 ||
    fail "both without jq unexpectedly failed: $(cat /tmp/installer-jq-ok.out)"
  assert_file "$ok_dir/project/.codex/config.toml"
  assert_file "$ok_dir/project/.mcp.json"
}

test_final_paths_and_readme_bootstrap_contract() {
  assert_file "$REPO_ROOT/scripts/install-from-remote.sh"
  [[ ! -e "$REPO_ROOT/install-from-remote.sh" ]] || fail "root bash installer should not exist"
  assert_contains "$REPO_ROOT/README.md" "https://raw.githubusercontent.com/chiendao1808/atlassian-mcp/\${INSTALLER_REF}/scripts/install-from-remote.sh"
  assert_contains "$REPO_ROOT/README.md" "INSTALLER_REF='main'"
  assert_contains "$REPO_ROOT/README.md" "--release-tag v1.0.4"
  assert_not_contains "$REPO_ROOT/README.md" "--source-repo-url"
}

for test_name in \
  test_default_release_download_verifies_and_installs_without_source_build \
  test_release_tag_pins_exact_asset \
  test_checksum_mismatch_fails_without_replacing_destination \
  test_unsupported_platform_errors_before_download \
  test_binary_override_keeps_config_behavior_and_skips_download \
  test_service_base_urls_reject_embedded_credentials \
  test_non_interactive_bitbucket_requires_token_env_value \
  test_jira_and_confluence_username_require_module_and_password_env \
  test_confluence_wrapper_clears_stale_fixed_env_and_preserves_password_alias \
  test_agent_config_escapes_wrapper_path_for_toml_and_json \
  test_piped_installer_without_agents_fails_without_terminal \
  test_claude_cli_registers_scope_local_and_user \
  test_claude_cli_missing_binary_errors_clearly \
  test_rerun_is_idempotent_and_config_failure_rolls_back \
  test_dry_run_validates_without_side_effects \
  test_agent_selection_contract \
  test_cursor_config_paths_and_scope_mapping \
  test_cursor_merge_preserves_existing_json \
  test_cursor_conflict_replace_and_idempotency \
  test_cursor_rejects_malformed_existing_config \
  test_cursor_command_escaping \
  test_kiro_config_paths_and_scope_mapping \
  test_kiro_merge_preserves_existing_json \
  test_kiro_conflict_replace_and_idempotency \
  test_kiro_only_install_does_not_touch_other_agents \
  test_multi_agent_failure_rolls_back_prior_writes \
  test_dry_run_all_creates_no_cursor_or_kiro_config \
  test_cursor_and_kiro_configs_contain_no_secrets \
  test_cursor_only_install_skips_claude_and_codex \
  test_jq_required_only_for_cursor_or_kiro \
  test_final_paths_and_readme_bootstrap_contract
do
  "$test_name"
  echo "ok $test_name"
done

echo "PASS install-from-remote_test.sh"
