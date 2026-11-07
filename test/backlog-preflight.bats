#!/usr/bin/env bats

bats_require_minimum_version 1.5.0

# The backlog skills' preflight: one PASS/WARN/FAIL line per prerequisite a
# session needs, in a fixed order, then a count. Stand-ins for gh, ssh-keygen,
# ssh-add and sysctl on PATH answer as the test directs, so no test reaches
# GitHub, an agent or the kernel. Each test gets its own CLAUDE_CONFIG_DIR, a
# repo with a bare origin, and a plugin registry naming a superpowers directory
# whose version matches the one sub-skill-overrides.md quotes.

function setup() {
  load test_helper/common
  PREFLIGHT="${REPO_DIR}/.claude/skills/work-backlog/backlog-preflight"
  CLAUDE_CONFIG_DIR="${BATS_TEST_TMPDIR}/config"
  STUB_LOG="${BATS_TEST_TMPDIR}/stub.log"
  export CLAUDE_CONFIG_DIR STUB_LOG
  mkdir --parents "${CLAUDE_CONFIG_DIR}/plugins" "${BATS_TEST_TMPDIR}/bin"
  make_stubs
  PATH="${BATS_TEST_TMPDIR}/bin:${PATH}"
  make_ready_repo
}

# @description Write the stand-ins. Each records its arguments and answers from
#              an environment variable the test sets: GH_AUTH_RC, SIGN_RC,
#              SSH_ADD_RC, SSH_ADD_OUT (a file whose content is printed) and
#              SYSCTL_VALUE.
# @noargs
function make_stubs() {
  # shellcheck disable=SC2016 # the stand-ins' own expansions must stay literal in the files they write
  printf '%s\n' '#!/usr/bin/env bash' \
    'printf "gh %s\n" "$*" >> "${STUB_LOG}"' \
    'exit "${GH_AUTH_RC:-0}"' > "${BATS_TEST_TMPDIR}/bin/gh"
  # shellcheck disable=SC2016 # as above
  printf '%s\n' '#!/usr/bin/env bash' \
    'printf "ssh-keygen %s\n" "$*" >> "${STUB_LOG}"' \
    'cat > /dev/null' \
    'exit "${SIGN_RC:-0}"' > "${BATS_TEST_TMPDIR}/bin/ssh-keygen"
  # shellcheck disable=SC2016 # as above
  printf '%s\n' '#!/usr/bin/env bash' \
    'printf "ssh-add %s\n" "$*" >> "${STUB_LOG}"' \
    'if [[ -n "${SSH_ADD_OUT:-}" ]]; then cat "${SSH_ADD_OUT}"; fi' \
    'exit "${SSH_ADD_RC:-0}"' > "${BATS_TEST_TMPDIR}/bin/ssh-add"
  # shellcheck disable=SC2016 # as above
  printf '%s\n' '#!/usr/bin/env bash' \
    'printf "sysctl %s\n" "$*" >> "${STUB_LOG}"' \
    'printf "%s\n" "${SYSCTL_VALUE:-1}"' > "${BATS_TEST_TMPDIR}/bin/sysctl"
  chmod +x "${BATS_TEST_TMPDIR}/bin/"*
}

# @description Create a repo on main with one commit pushed to a bare origin,
#              ssh signing configured with a key file and its public half, the
#              public half listed by the ssh-add stand-in, a plugin registry
#              naming a superpowers 7.0.0 directory, and a Notification hook in
#              the user settings: every check passes.
# @noargs
function make_ready_repo() {
  make_repo
  git symbolic-ref HEAD refs/heads/main
  git config user.name 'Test'
  git config user.email 'test@example.invalid'
  git config commit.gpgsign false
  printf 'x\n' > file
  git add file
  git commit --quiet --message 'init'
  git init --quiet --bare "${BATS_TEST_TMPDIR}/origin"
  git remote add origin "${BATS_TEST_TMPDIR}/origin"
  git push --quiet origin main
  # The real checkout ignores .claude/ through the user's global excludes; the
  # fixture has none, so the config and settings files a test writes must not
  # dirty the tree.
  printf '.claude/\n' >> .git/info/exclude
  printf 'private\n' > "${BATS_TEST_TMPDIR}/key"
  printf 'ssh-ed25519 AAAATESTKEY test@example.invalid\n' > "${BATS_TEST_TMPDIR}/key.pub"
  git config gpg.format ssh
  git config user.signingkey "${BATS_TEST_TMPDIR}/key"
  SSH_ADD_OUT="${BATS_TEST_TMPDIR}/key.pub"
  export SSH_ADD_OUT
  register_superpowers '7.0.0'
  jq --null-input '{hooks: {Notification: [{hooks: [{type: "command", command: "notify-send x"}]}]}}' \
    > "${CLAUDE_CONFIG_DIR}/settings.json"
}

# @description Point the plugin registry at a superpowers directory of the
#              given version, creating the directory.
# @arg $1 version the version segment of the directory name
function register_superpowers() {
  local -r version="$1"
  mkdir --parents "${BATS_TEST_TMPDIR}/sp/${version}"
  jq --null-input --arg dir "${BATS_TEST_TMPDIR}/sp/${version}" \
    '{plugins: {"superpowers@m": [{installPath: $dir}]}}' \
    > "${CLAUDE_CONFIG_DIR}/plugins/installed_plugins.json"
}

# @description Print the check names of the output in order, one per line.
# @noargs
function check_names() {
  awk '$1 == "PASS" || $1 == "WARN" || $1 == "FAIL" { sub(":", "", $2); print $2 }' <<< "${output}"
}

@test 'exits 2 with usage for an unknown argument' {
  run "${PREFLIGHT}" --bogus
  assert_failure 2
  assert_output --partial 'usage: backlog-preflight [--parallel]'
}

@test 'exits 2 when CLAUDE_CONFIG_DIR is not set' {
  unset CLAUDE_CONFIG_DIR
  run "${PREFLIGHT}"
  assert_failure 2
  assert_output --partial 'CLAUDE_CONFIG_DIR is not set'
}

@test 'exits 2 outside a git repository' {
  cd "${BATS_TEST_TMPDIR}"
  run "${PREFLIGHT}"
  assert_failure 2
  assert_output --partial 'not inside a git repository'
}

@test 'a ready checkout passes every check, in the documented order, and exits 0' {
  run "${PREFLIGHT}"
  assert_success
  assert_equal "$(check_names)" "$(printf '%s\n' config-dir gh-auth clean-tree on-main main-current signing \
    agent-key ip-forward superpowers config notification-hook state)"
  refute_line --regexp '^(WARN|FAIL) '
  assert_line 'preflight: 0 fail, 0 warn'
}

@test 'every check line has the PASS|WARN|FAIL <check>: <detail> shape and the count comes last' {
  run "${PREFLIGHT}"
  local line
  while IFS= read -r line; do
    [[ "${line}" =~ ^(PASS|WARN|FAIL)\ [a-z-]+:\ .+$ || "${line}" == 'preflight: 0 fail, 0 warn' ]]
  done <<< "${output}"
  assert_equal "${lines[-1]}" 'preflight: 0 fail, 0 warn'
}

@test 'fails config-dir when CLAUDE_CONFIG_DIR is not a directory, and still runs the other checks' {
  CLAUDE_CONFIG_DIR="${BATS_TEST_TMPDIR}/missing" run "${PREFLIGHT}"
  assert_failure 1
  assert_line --regexp '^FAIL config-dir: .*missing'
  assert_line --regexp '^PASS clean-tree: '
}

@test 'fails gh-auth when gh auth status fails, and asks for github.com' {
  GH_AUTH_RC=1 run "${PREFLIGHT}"
  assert_failure 1
  assert_line --regexp '^FAIL gh-auth: '
  grep --quiet --fixed-strings 'gh auth status --hostname github.com' "${STUB_LOG}"
}

@test 'fails clean-tree on a dirty tree, counting the paths' {
  printf 'y\n' > file
  printf 'z\n' > other
  run "${PREFLIGHT}"
  assert_failure 1
  assert_line --regexp '^FAIL clean-tree: 2 changed path'
}

@test 'fails on-main on another branch, naming it' {
  git switch --quiet --create topic
  run "${PREFLIGHT}"
  assert_failure 1
  assert_line --regexp '^FAIL on-main: on topic'
}

@test 'fails on-main on a detached HEAD' {
  git switch --quiet --detach
  run "${PREFLIGHT}"
  assert_failure 1
  assert_line --regexp '^FAIL on-main: detached'
}

@test 'fails main-current when origin cannot be fetched' {
  git remote remove origin
  run "${PREFLIGHT}"
  assert_failure 1
  assert_line --regexp '^FAIL main-current: git fetch origin main failed'
}

@test 'fails main-current when a local commit is ahead of origin/main, saying to move it off main' {
  printf 'y\n' > file
  git commit --quiet --all --message 'local'
  run "${PREFLIGHT}"
  assert_failure 1
  assert_line --regexp '^FAIL main-current: .*move it off main'
}

@test 'passes main-current when local main is behind origin/main' {
  git clone --quiet --branch main "${BATS_TEST_TMPDIR}/origin" "${BATS_TEST_TMPDIR}/other"
  git -C "${BATS_TEST_TMPDIR}/other" -c user.name=t -c user.email=t@example.invalid \
    -c commit.gpgsign=false commit --quiet --allow-empty --message 'remote'
  git -C "${BATS_TEST_TMPDIR}/other" push --quiet origin main
  run "${PREFLIGHT}"
  assert_success
  assert_line --regexp '^PASS main-current: '
}

@test 'fails signing when gpg.format is not ssh' {
  git config gpg.format openpgp
  run "${PREFLIGHT}"
  assert_failure 1
  assert_line --regexp '^FAIL signing: gpg.format is openpgp'
}

@test 'fails signing when the key file is missing' {
  rm --force -- "${BATS_TEST_TMPDIR}/key"
  run "${PREFLIGHT}"
  assert_failure 1
  assert_line --regexp '^FAIL signing: .*not a readable file'
}

@test 'fails signing when a test signature fails, having signed with the configured key' {
  SIGN_RC=1 run "${PREFLIGHT}"
  assert_failure 1
  assert_line --regexp '^FAIL signing: .*test signature failed'
  grep --quiet --fixed-strings -- "-Y sign -n git -f ${BATS_TEST_TMPDIR}/key" "${STUB_LOG}"
}

@test 'passes agent-key when ssh-add lists the public half, always with the lifetime reminder' {
  run "${PREFLIGHT}"
  assert_line --regexp '^PASS agent-key: .*lifetime cannot be read; confirm the key was added with no lifetime'
}

@test 'warns agent-key when ssh-add does not list the key' {
  printf 'ssh-ed25519 AAAAOTHER other\n' > "${BATS_TEST_TMPDIR}/other.pub"
  SSH_ADD_OUT="${BATS_TEST_TMPDIR}/other.pub" run "${PREFLIGHT}"
  assert_success
  assert_line --regexp '^WARN agent-key: not listed by ssh-add -L; lifetime cannot be read'
  assert_line 'preflight: 0 fail, 1 warn'
}

@test 'warns agent-key when no public key lies beside the signing key' {
  rm --force -- "${BATS_TEST_TMPDIR}/key.pub"
  run "${PREFLIGHT}"
  assert_success
  assert_line --regexp '^PASS signing: '
  assert_line --regexp "^WARN agent-key: no public key beside ${BATS_TEST_TMPDIR}/key; lifetime cannot be read"
}

@test 'warns agent-key when ssh-add fails' {
  SSH_ADD_RC=2 run "${PREFLIGHT}"
  assert_success
  assert_line --regexp '^WARN agent-key: '
}

@test 'warns ip-forward when the value is not 1, naming the one step that needs it' {
  SYSCTL_VALUE=0 run "${PREFLIGHT}"
  assert_success
  assert_line --regexp '^WARN ip-forward: .*0.*; needed only by \./\.ci/test-image$'
}

@test 'fails superpowers when no plugin is installed' {
  rm --force -- "${CLAUDE_CONFIG_DIR}/plugins/installed_plugins.json"
  run "${PREFLIGHT}"
  assert_failure 1
  assert_line --regexp '^FAIL superpowers: no installed superpowers plugin'
}

@test 'warns superpowers when the installed version differs from the one the overrides quote' {
  register_superpowers '6.9.0'
  run "${PREFLIGHT}"
  assert_success
  assert_line --regexp '^WARN superpowers: installed 6\.9\.0, sub-skill-overrides.md quotes 7\.0\.0'
}

@test 'passes config as defaults when the file is absent' {
  run "${PREFLIGHT}"
  assert_line 'PASS config: defaults'
}

@test 'passes config with valid keys, counting them' {
  mkdir --parents .claude/backlog
  printf 'shakedown: off\nmax-hours: 6\nmax-parked: 1\n' > .claude/backlog/config
  run "${PREFLIGHT}"
  assert_success
  assert_line 'PASS config: 3 keys'
}

@test 'passes config with blank lines between keys, counting only the keys' {
  mkdir --parents .claude/backlog
  printf 'shakedown: off\n\nmax-hours: 6\n' > .claude/backlog/config
  run "${PREFLIGHT}"
  assert_success
  assert_line 'PASS config: 2 keys'
}

@test 'fails config on an unknown key, naming the line' {
  mkdir --parents .claude/backlog
  printf 'shakedown: on\ncap: 4\n' > .claude/backlog/config
  run "${PREFLIGHT}"
  assert_failure 1
  assert_line 'FAIL config: line 2 "cap: 4" is not shakedown|max-hours|max-parked: <value>'
}

@test 'fails config on a bad value, naming the line' {
  mkdir --parents .claude/backlog
  printf 'max-hours: soon\n' > .claude/backlog/config
  run "${PREFLIGHT}"
  assert_failure 1
  assert_line --regexp '^FAIL config: line 1 "max-hours: soon"'
}

@test 'warns notification-hook when no settings file has one, showing a notify-send example' {
  rm --force -- "${CLAUDE_CONFIG_DIR}/settings.json"
  run "${PREFLIGHT}"
  assert_success
  assert_line --regexp '^WARN notification-hook: none set; .*notify-send'
}

@test 'passes notification-hook from the repo settings.local.json, ignoring a settings file that is not JSON' {
  printf 'not json\n' > "${CLAUDE_CONFIG_DIR}/settings.json"
  mkdir --parents .claude
  jq --null-input '{hooks: {Notification: [{hooks: [{type: "command", command: "x"}]}]}}' \
    > .claude/settings.local.json
  run "${PREFLIGHT}"
  assert_success
  assert_line --regexp '^PASS notification-hook: .*\.claude/settings\.local\.json'
}

@test 'the state line lists the session record, the parallel files and the pending findings' {
  mkdir --parents .claude/backlog/parallel
  printf 'session-date: 2026-01-01\n' > .claude/backlog/current-session.md
  printf 'state: parked\n' > .claude/backlog/parallel/7.md
  printf '## a\n\n## b\n' > .claude/backlog/pending-findings.md
  run "${PREFLIGHT}"
  assert_line 'PASS state: current-session.md present; parallel: 1 file; pending findings: 2'
}

@test 'the state line says when nothing is in progress' {
  run "${PREFLIGHT}"
  assert_line 'PASS state: current-session.md absent; parallel: 0 files; pending findings: 0'
}

@test '--parallel adds the workflow-tool reminder as the last check' {
  run "${PREFLIGHT}" --parallel
  assert_success
  assert_equal "$(check_names | tail --lines=1)" 'workflow-tool'
  assert_line --regexp '^WARN workflow-tool: cannot be checked from a shell; the session confirms the Workflow tool'
  assert_line 'preflight: 0 fail, 1 warn'
}

@test 'without --parallel there is no workflow-tool line' {
  run "${PREFLIGHT}"
  refute_line --partial 'workflow-tool'
}

@test 'the count sums every FAIL and WARN and the exit is 1 with any FAIL' {
  git config gpg.format openpgp
  printf 'y\n' > file
  SYSCTL_VALUE=0 run "${PREFLIGHT}"
  assert_failure 1
  assert_line 'preflight: 2 fail, 1 warn'
}
