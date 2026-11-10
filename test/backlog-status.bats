#!/usr/bin/env bats

bats_require_minimum_version 1.5.0

# The backlog skills' read-only progress view: what current-session.md, the
# parallel run's files, GitHub and git say about a session, from any directory
# in the checkout or one of its worktrees. A stand-in `gh` on PATH serves
# fixture JSON, so no test reaches GitHub.

function setup() {
  load test_helper/common
  STATUS="${REPO_DIR}/.claude/skills/work-backlog/backlog-status"
  FIXTURES="${BATS_TEST_TMPDIR}/fixtures"
  GH_LOG="${BATS_TEST_TMPDIR}/gh.log"
  mkdir "${FIXTURES}" "${BATS_TEST_TMPDIR}/bin"
  export FIXTURES GH_LOG
  # shellcheck disable=SC2016 # the stand-in's own expansions must stay literal in the file it writes
  printf '%s\n' '#!/usr/bin/env bash' \
    'printf "%s\n" "$*" >> "${GH_LOG}"' \
    'if [[ -n "${GH_RC:-}" ]]; then exit "${GH_RC}"; fi' \
    'if [[ "$1 $2" == "pr list" ]]; then exec cat "${FIXTURES}/prs.json"; fi' \
    'if [[ "$1 $2" == "pr checks" && -f "${FIXTURES}/checks-$3.json" ]]; then' \
    '  exec cat "${FIXTURES}/checks-$3.json"' \
    'fi' \
    'printf "gh: not found\n" >&2' \
    'exit 1' > "${BATS_TEST_TMPDIR}/bin/gh"
  chmod +x "${BATS_TEST_TMPDIR}/bin/gh"
  PATH="${BATS_TEST_TMPDIR}/bin:${PATH}"
  make_repo
  git config user.name 'Test'
  git config user.email 'test@example.invalid'
  git config commit.gpgsign false
  printf 'x\n' > file
  git add file
  git commit --quiet --message 'init'
  mkdir --parents .claude/backlog/parallel
  printf '[]\n' > "${FIXTURES}/prs.json"
}

# @description Write a current-session.md for a bug session with two issues,
#              timestamped lines and an `at:` line.
# @noargs
function write_session() {
  printf '%s\n' 'session-date: 2026-03-04' \
    '2026-03-04T10:00:00Z flow: bug' \
    '2026-03-04T10:00:01Z issue 160' \
    '2026-03-04T10:00:01Z issue 159 after 160' \
    '2026-03-04T10:00:02Z halt answers: Q1 a' \
    '2020-01-01T00:00:00Z started: 2020-01-01T00:00:00Z' \
    'at: SKILL.md step 7' \
    '2026-03-04T10:40:00Z #160 pr 201' \
    '2026-03-04T11:00:00Z #160 merged' > .claude/backlog/current-session.md
}

# @description Print the lines of one section of the output, without the
#              heading, up to the next heading.
# @arg $1 heading the section heading
function section() {
  local -r heading="$1"
  awk --assign h="${heading}" '
    /^[A-Z]/ { on = ($0 == h); next }
    on { print }' <<< "${output}"
}

@test 'exits 2 with usage when given an argument' {
  run "${STATUS}" --bogus
  assert_failure 2
  assert_output --partial 'usage: backlog-status'
}

@test 'exits 2 when neither the directory nor the script is inside a git repository' {
  mkdir "${BATS_TEST_TMPDIR}/loose"
  cp "${STATUS}" "${BATS_TEST_TMPDIR}/loose/backlog-status"
  cd "${BATS_TEST_TMPDIR}"
  run "${BATS_TEST_TMPDIR}/loose/backlog-status"
  assert_failure 2
  assert_output --partial 'not inside a git repository'
}

@test 'run from outside any repository, reads the checkout the script lives in' {
  write_session
  mkdir --parents .claude/skills/work-backlog
  cp "${STATUS}" .claude/skills/work-backlog/backlog-status
  mkdir "${BATS_TEST_TMPDIR}/elsewhere"
  cd "${BATS_TEST_TMPDIR}/elsewhere"
  run "${FIXTURE_REPO}/.claude/skills/work-backlog/backlog-status"
  assert_success
  assert_line '  session-date: 2026-03-04'
  assert_line "  ${FIXTURE_REPO} master"
}

@test 'prints every section, and no session in progress, when there is no record' {
  run "${STATUS}"
  assert_success
  assert_line 'Session'
  assert_line '  no session in progress'
  assert_line 'Issues'
  assert_line 'Parallel'
  assert_line 'Pull requests'
  assert_line 'Worktrees'
  assert_line 'Findings'
  assert_equal "$(section 'Pull requests')" '  none (no session in progress)'
  assert_equal "$(section 'Findings')" '  0 pending'
  assert_line 'Stop'
}

@test 'the Session section reads the date, flow, started time with elapsed, and the at line' {
  write_session
  run "${STATUS}"
  assert_success
  assert_line '  session-date: 2026-03-04'
  assert_line '  flow: bug'
  assert_line --regexp '^  started: 2020-01-01T00:00:00Z \(elapsed [0-9]+h[0-9]{2}m\)$'
  assert_line '  at: SKILL.md step 7'
}

@test 'the Session section prints only the lines the record has, and Issues says none' {
  printf 'session-date: 2026-03-04\n' > .claude/backlog/current-session.md
  run "${STATUS}"
  assert_success
  assert_equal "$(section 'Session')" '  session-date: 2026-03-04'
  assert_equal "$(section 'Issues')" '  none'
}

@test 'the Session section says when the started time cannot be read' {
  printf 'session-date: 2026-03-04\n2026-03-04T10:00:00Z started: soon\n' > .claude/backlog/current-session.md
  run "${STATUS}"
  assert_line '  started: soon (unreadable time)'
}

@test 'the Session section reads the deadline from run-args.json with the time left or passed' {
  write_session
  printf '{"deadline": "2020-01-02T00:00:00Z"}\n' > .claude/backlog/parallel/run-args.json
  run "${STATUS}"
  assert_line --regexp '^  deadline: 2020-01-02T00:00:00Z \(passed [0-9]+h[0-9]{2}m ago\)$'
  printf '{"deadline": "2999-01-01T00:00:00Z"}\n' > .claude/backlog/parallel/run-args.json
  run "${STATUS}"
  assert_line --regexp '^  deadline: 2999-01-01T00:00:00Z \([0-9]+h[0-9]{2}m left\)$'
}

@test 'the Issues section lists each issue line with its last timestamped event' {
  write_session
  run "${STATUS}"
  assert_equal "$(section 'Issues')" "$(printf '%s\n' \
    '  issue 160' \
    '    last: 2026-03-04T11:00:00Z #160 merged' \
    '  issue 159 after 160' \
    '    no event yet')"
}

@test 'the Issues section matches an event by its #N token, never by digits in a timestamp or another number' {
  printf '%s\n' 'session-date: 2026-02-03' \
    '2026-02-03T14:02:11Z flow: bug' \
    '2026-02-03T14:02:11Z issue 31' \
    '2026-02-03T14:02:11Z issue 1' \
    '2026-02-03T14:31:50Z #99 pr 57' \
    '2026-02-03T14:40:00Z #131 merged' \
    'at: merging.md step 1' > .claude/backlog/current-session.md
  run "${STATUS}"
  assert_equal "$(section 'Issues')" "$(printf '%s\n' \
    '  issue 31' \
    '    no event yet' \
    '  issue 1' \
    '    no event yet')"
}

@test 'the Issues section lists a #N progress line as an event of issue N, not as an issue' {
  printf '%s\n' 'session-date: 2026-02-03' \
    '2026-02-03T14:02:11Z issue 42 kind bug' \
    '2026-02-03T14:31:50Z #42 pr 57' \
    '2026-02-03T14:50:00Z #42 review round 2' > .claude/backlog/current-session.md
  run "${STATUS}"
  assert_equal "$(section 'Issues')" "$(printf '%s\n' \
    '  issue 42 kind bug' \
    '    last: 2026-02-03T14:50:00Z #42 review round 2')"
}

@test 'the Parallel section lists each issue file with state, pr and the first park-note line' {
  printf 'state: parked\npr: 201\npark-note: Rebase conflicted in lints.toml.\nruling: x\n' \
    > .claude/backlog/parallel/160.md
  printf 'state: merged\npr: 202\n' > .claude/backlog/parallel/159.md
  printf 'items\n' > .claude/backlog/parallel/summary.md
  printf '## a\n' > .claude/backlog/parallel/160-findings.md
  run "${STATUS}"
  assert_equal "$(section 'Parallel')" "$(printf '%s\n' \
    '  159: merged pr=202' \
    '  160: parked pr=201 park-note: Rebase conflicted in lints.toml.')"
}

@test 'the Parallel section says when the run has written nothing' {
  run "${STATUS}"
  assert_equal "$(section 'Parallel')" '  none'
}

@test 'the Pull requests section lists the session PRs with a check summary' {
  write_session
  jq --null-input '[{number: 201, title: "fix: a", headRefName: "fix/160-a"},
                    {number: 202, title: "feat: b", headRefName: "feat/159-b"}]' > "${FIXTURES}/prs.json"
  jq --null-input '[{bucket: "pass"}, {bucket: "pass"}, {bucket: "skipping"}]' > "${FIXTURES}/checks-201.json"
  jq --null-input '[{bucket: "fail"}, {bucket: "pending"}, {bucket: "cancel"}]' > "${FIXTURES}/checks-202.json"
  run "${STATUS}"
  assert_success
  assert_equal "$(section 'Pull requests')" "$(printf '%s\n' \
    '  201 fix: a (fix/160-a) pass=2 fail=0 pending=0 skipping=1 cancel=0' \
    '  202 feat: b (feat/159-b) pass=0 fail=1 pending=1 skipping=0 cancel=1')"
  grep --quiet --fixed-strings -- 'pr list --state open --search "Backlog-Session: 2026-03-04" in:body' "${GH_LOG}"
}

@test 'the Pull requests section says gh unavailable for a PR whose checks cannot be read' {
  write_session
  jq --null-input '[{number: 203, title: "fix: c", headRefName: "fix/3-c"}]' > "${FIXTURES}/prs.json"
  run "${STATUS}"
  assert_success
  assert_equal "$(section 'Pull requests')" '  203 fix: c (fix/3-c) gh unavailable'
}

@test 'the Pull requests section says none when the search is empty' {
  write_session
  run "${STATUS}"
  assert_equal "$(section 'Pull requests')" '  none'
}

@test 'the Pull requests section says gh unavailable when gh fails, and the rest still prints' {
  write_session
  GH_RC=1 run "${STATUS}"
  assert_success
  assert_equal "$(section 'Pull requests')" '  gh unavailable'
  assert_line 'Worktrees'
}

@test 'the Worktrees section shows each worktree with its branch or detached state and its lock' {
  git worktree add --quiet --detach "${BATS_TEST_TMPDIR}/wt-detached"
  git worktree add --quiet "${BATS_TEST_TMPDIR}/wt-branch" -b topic
  git worktree lock --reason "claude agent x (pid 4000000 start 1)" "${BATS_TEST_TMPDIR}/wt-branch"
  git worktree add --quiet --detach "${BATS_TEST_TMPDIR}/wt-alive"
  git worktree lock --reason "claude agent y (pid $$ start 1)" "${BATS_TEST_TMPDIR}/wt-alive"
  git worktree add --quiet --detach "${BATS_TEST_TMPDIR}/wt-plain-lock"
  git worktree lock "${BATS_TEST_TMPDIR}/wt-plain-lock"
  run "${STATUS}"
  assert_success
  assert_line "  ${FIXTURE_REPO} master"
  assert_line "  ${BATS_TEST_TMPDIR}/wt-detached detached"
  assert_line "  ${BATS_TEST_TMPDIR}/wt-branch topic locked (pid 4000000, dead)"
  assert_line "  ${BATS_TEST_TMPDIR}/wt-alive detached locked (pid $$, alive)"
  assert_line "  ${BATS_TEST_TMPDIR}/wt-plain-lock detached locked"
}

@test 'the Branches section lists the branches merged into main and those not merged' {
  git branch --move master main
  git branch done-work
  git switch --quiet --create open-work
  printf 'y\n' > other
  git add other
  git commit --quiet --message 'more'
  git switch --quiet main
  run "${STATUS}"
  assert_success
  assert_equal "$(section 'Branches')" "$(printf '%s\n' \
    '  merged into main: done-work' \
    '  not merged: open-work')"
}

@test 'the Branches section says none when main is the only branch' {
  git branch --move master main
  run "${STATUS}"
  assert_equal "$(section 'Branches')" "$(printf '%s\n' \
    '  merged into main: none' \
    '  not merged: none')"
}

@test 'the Branches section says so when there is no main branch' {
  run "${STATUS}"
  assert_success
  assert_equal "$(section 'Branches')" '  no main branch'
}

@test 'the Findings section counts the pending entries' {
  printf '## a\n\n- File: x\n\n## b\n' > .claude/backlog/pending-findings.md
  run "${STATUS}"
  assert_equal "$(section 'Findings')" '  2 pending'
}

@test 'the Stop section says how to stop when no stop is asked for' {
  run "${STATUS}"
  assert_equal "$(section 'Stop')" '  touch .claude/backlog/stop (starts no new issue; work in progress finishes, or in a parallel run stops before its merge)'
}

@test 'the Stop section says requested when the stop file exists' {
  : > .claude/backlog/stop
  run "${STATUS}"
  assert_equal "$(section 'Stop')" '  requested (.claude/backlog/stop exists)'
}

@test 'reads the main checkout from inside a worktree' {
  write_session
  git worktree add --quiet --detach "${BATS_TEST_TMPDIR}/wt"
  cd "${BATS_TEST_TMPDIR}/wt"
  run "${STATUS}"
  assert_success
  assert_line '  session-date: 2026-03-04'
}

@test 'reads the main checkout from a subdirectory' {
  write_session
  mkdir sub
  cd sub
  run "${STATUS}"
  assert_line '  session-date: 2026-03-04'
}
