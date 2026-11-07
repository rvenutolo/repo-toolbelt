#!/usr/bin/env bats

# The backlog skills' issue index: it writes the full issue JSON to a file and
# prints one TSV row per issue. A stand-in `gh` on PATH serves fixture JSON
# and records its arguments, so no test reaches GitHub.

function setup() {
  load test_helper/common
  BACKLOG_INDEX="${REPO_DIR}/.claude/skills/work-backlog/backlog-index"
  FIXTURES="${BATS_TEST_TMPDIR}/fixtures"
  GH_LOG="${BATS_TEST_TMPDIR}/gh.log"
  OUT="${BATS_TEST_TMPDIR}/out/issues.json"
  mkdir "${FIXTURES}" "${BATS_TEST_TMPDIR}/bin"
  export FIXTURES GH_LOG
  # shellcheck disable=SC2016 # the stand-in's own expansions must stay literal in the file it writes
  printf '%s\n' '#!/usr/bin/env bash' \
    'printf "%s\n" "$*" >> "${GH_LOG}"' \
    'if [[ "$1 $2" == "issue list" ]]; then exec cat "${FIXTURES}/list.json"; fi' \
    'if [[ "$1 $2" == "issue view" && -f "${FIXTURES}/$3.json" ]]; then exec cat "${FIXTURES}/$3.json"; fi' \
    'if [[ "$1 $2" == "pr list" && -z "${GH_PR_LIST_FAILS:-}" ]]; then' \
    '  if [[ -f "${FIXTURES}/prs.json" ]]; then exec cat "${FIXTURES}/prs.json"; fi' \
    '  exec printf "[]\n"' \
    'fi' \
    'printf "gh: not found\n" >&2' \
    'exit 1' > "${BATS_TEST_TMPDIR}/bin/gh"
  chmod +x "${BATS_TEST_TMPDIR}/bin/gh"
  PATH="${BATS_TEST_TMPDIR}/bin:${PATH}"
}

# @description Print one issue as JSON from a `number|title|labels-csv|body`
#              spec, with the given state.
# @arg $1 state OPEN or CLOSED
# @arg $2 spec the issue spec
function issue_json() {
  local -r state="$1"
  local -r spec="$2"
  jq --null-input --arg state "${state}" --arg spec "${spec}" '
    ($spec | split("|")) as $f
    | {number: ($f[0] | tonumber), state: $state, title: $f[1],
       labels: [($f[2] | split(",")[] | select(. != "") | {name: .})],
       body: ($f[3:] | join("|")), comments: []}'
}

# @description Give one issue's fixtures the given comments, replacing any it
#              had, in the list fixture and in its view fixture when present.
# @arg $1 number the issue number
# @arg $@ comment bodies, one per argument
function add_comments() {
  local -r number="$1"
  shift
  local comments
  comments="$(printf '%s\n' "$@" | jq --raw-input '{body: .}' | jq --slurp '.')"
  local file
  for file in "${FIXTURES}/list.json" "${FIXTURES}/${number}.json"; do
    if [[ -f "${file}" ]]; then
      jq --argjson n "${number}" --argjson c "${comments}" \
        'if type == "array" then map(if .number == $n then .comments = $c else . end) else .comments = $c end' \
        "${file}" > "${file}.tmp"
      mv "${file}.tmp" "${file}"
    fi
  done
}

# @description Write the `gh pr list` fixture: merged pull requests whose
#              bodies are the arguments, numbered from 101.
# @arg $@ one PR body per argument
function pr_list_fixture() {
  local body
  local number=101
  for body in "$@"; do
    jq --null-input --argjson n "${number}" --arg body "${body}" '{number: $n, body: $body}'
    number=$((number + 1))
  done | jq --slurp '.' > "${FIXTURES}/prs.json"
}

# @description Write the `gh issue list` fixture: a JSON array of the open
#              issues given as `number|title|labels-csv|body` arguments.
# @arg $@ one issue per argument
function list_fixture() {
  local issue
  for issue in "$@"; do
    issue_json OPEN "${issue}"
  done | jq --slurp '.' > "${FIXTURES}/list.json"
}

# @description Write a `gh issue view` fixture for one issue.
# @arg $1 state OPEN or CLOSED
# @arg $2 spec the issue spec, `number|title|labels-csv|body`
function view_fixture() {
  local -r state="$1"
  local -r spec="$2"
  issue_json "${state}" "${spec}" > "${FIXTURES}/${spec%%|*}.json"
}

# @description Print the given column of the row for one issue number.
# @arg $1 number the issue number
# @arg $2 column the 1-based column
function cell() {
  awk --field-separator '\t' --assign n="$1" --assign c="$2" '$1 == n { print $c }' <<< "${output}"
}

@test 'exits 2 with usage for an argument that is not an issue number' {
  run "${BACKLOG_INDEX}" --bogus
  assert_failure 2
  assert_output --partial 'usage: backlog-index [--out <file>] [<issue>...]'
}

@test 'exits 2 for issue number 0' {
  run "${BACKLOG_INDEX}" 0
  assert_failure 2
}

@test 'exits 2 when --out has no value' {
  run "${BACKLOG_INDEX}" --out
  assert_failure 2
}

@test 'prints the header line first' {
  list_fixture '1|feat: a||body'
  run "${BACKLOG_INDEX}" --out "${OUT}"
  assert_success
  assert_line --index 0 \
    "$(printf 'number\tstate\tkind\tlabels\ttitle\trepro\trefs\tumbrella\tcomments\tbytes\tpieces')"
}

@test 'writes the full issue JSON to the --out file' {
  list_fixture '1|feat: a|enhancement|first body' '2|fix: b||second body'
  run "${BACKLOG_INDEX}" --out "${OUT}"
  assert_success
  assert_equal "$(jq --sort-keys '.' "${OUT}")" "$(jq --sort-keys '.' "${FIXTURES}/list.json")"
}

@test 'writes to .claude/tmp/issues.json under the main checkout by default, even from a worktree' {
  list_fixture '1|feat: a||body'
  make_repo
  git -c user.name=t -c user.email=t@t commit --quiet --allow-empty --message init
  git worktree add --quiet "${BATS_TEST_TMPDIR}/wt" --detach
  cd "${BATS_TEST_TMPDIR}/wt"
  run "${BACKLOG_INDEX}"
  assert_success
  assert_equal "$(jq '.[0].number' "${FIXTURE_REPO}/.claude/tmp/issues.json")" '1'
  assert [ ! -e "${BATS_TEST_TMPDIR}/wt/.claude/tmp/issues.json" ]
}

@test 'writes to .claude/tmp/issues.json under the working directory outside a git repo' {
  list_fixture '1|feat: a||body'
  cd "${BATS_TEST_TMPDIR}"
  run "${BACKLOG_INDEX}"
  assert_success
  assert_equal "$(jq '.[0].number' "${BATS_TEST_TMPDIR}/.claude/tmp/issues.json")" '1'
}

@test 'writes a body over 16000 bytes to issue-<N>.md beside the index' {
  local long
  long="$(head --bytes=16001 /dev/zero | tr '\0' 'x')"
  list_fixture "1|feat: a||${long}" '2|feat: b||short'
  run "${BACKLOG_INDEX}" --out "${OUT}"
  assert_success
  assert_equal "$(cat "${BATS_TEST_TMPDIR}/out/issue-1.md")" "${long}"
  assert [ ! -e "${BATS_TEST_TMPDIR}/out/issue-2.md" ]
}

@test 'lists every open issue when no issue is named' {
  list_fixture '1|feat: a||body'
  run "${BACKLOG_INDEX}" --out "${OUT}"
  assert_success
  run grep --count '^issue list --state open' "${GH_LOG}"
  assert_output '1'
}

@test 'sorts the rows by issue number' {
  list_fixture '30|feat: c||x' '4|feat: a||x' '12|feat: b||x'
  run "${BACKLOG_INDEX}" --out "${OUT}"
  assert_success
  assert_equal "$(tail --lines=+2 <<< "${output}" | cut --fields=1 | paste --serial --delimiters=,)" '4,12,30'
}

@test 'classifies a bug label as a bug whatever the title' {
  list_fixture '1|feat: looks like a feature|bug|x'
  run "${BACKLOG_INDEX}" --out "${OUT}"
  assert_equal "$(cell 1 3)" 'bug'
}

@test 'classifies fix: titles, scoped or breaking, as bugs' {
  list_fixture '1|fix: a||x' '2|fix(ci): b||x' '3|fix!: c||x'
  run "${BACKLOG_INDEX}" --out "${OUT}"
  assert_equal "$(cell 1 3) $(cell 2 3) $(cell 3 3)" 'bug bug bug'
}

@test 'classifies feat: titles and enhancement labels as features' {
  list_fixture '1|feat: a||x' '2|Add a thing|enhancement|x' '3|feat(lint): c||x'
  run "${BACKLOG_INDEX}" --out "${OUT}"
  assert_equal "$(cell 1 3) $(cell 2 3) $(cell 3 3)" 'feature feature feature'
}

@test 'marks an issue with no kind signal as a judgment call' {
  list_fixture '1|ci: save caches||x' '2|Renovate has never run||x' '3|fixes the thing||x'
  run "${BACKLOG_INDEX}" --out "${OUT}"
  assert_equal "$(cell 1 3) $(cell 2 3) $(cell 3 3)" '? ? ?'
}

@test 'prints the state of every issue' {
  list_fixture '1|feat: a||x'
  run "${BACKLOG_INDEX}" --out "${OUT}"
  assert_equal "$(cell 1 2)" 'OPEN'
}

@test 'prints CLOSED for a named issue that is closed' {
  view_fixture CLOSED '5|fix: a||x'
  view_fixture OPEN '9|feat: b||y'
  run "${BACKLOG_INDEX}" --out "${OUT}" 5 9
  assert_success
  assert_equal "$(cell 5 2) $(cell 9 2)" 'CLOSED OPEN'
}

@test 'joins the label names with commas' {
  list_fixture '1|feat: a|enhancement,ci|x'
  run "${BACKLOG_INDEX}" --out "${OUT}"
  assert_equal "$(cell 1 4)" 'enhancement,ci'
}

@test 'prints the title' {
  list_fixture '1|feat: add the widget||x'
  run "${BACKLOG_INDEX}" --out "${OUT}"
  assert_equal "$(cell 1 5)" 'feat: add the widget'
}

@test 'marks a body that describes a reproduction' {
  list_fixture '1|fix: a||## Steps to reproduce' '2|fix: b||Reproduction: run it' '3|fix: c||no steps here' \
    '4|fix: d||Repro: run it twice' '5|fix: e||It reproduces on main' '6|fix: f||REPRODUCED with go test'
  run "${BACKLOG_INDEX}" --out "${OUT}"
  assert_equal "$(cell 1 6),$(cell 2 6),$(cell 3 6),$(cell 4 6),$(cell 5 6),$(cell 6 6)" 'yes,yes,,yes,yes,yes'
}

@test 'marks a body that reports a check failing on main or an audit flagging the repo' {
  # shellcheck disable=SC2016 # the backticks are markdown in the fixture body, not a command substitution
  list_fixture '1|ci: a||github-repos-audit flags this repo' '2|ci: b||the audit flags the repository' \
    '3|ci: c||check-examples fails on main' '4|ci: d||the gate fails on `main` today' '5|ci: e||nothing wrong'
  run "${BACKLOG_INDEX}" --out "${OUT}"
  assert_equal "$(cell 1 6),$(cell 2 6),$(cell 3 6),$(cell 4 6),$(cell 5 6)" 'yes,yes,yes,yes,'
}

@test 'does not read a feature body that says reproducible as a reproduction' {
  list_fixture '1|feat: a||Builds must be reproducible' '2|feat: b||ensures reproducibility of the image'
  run "${BACKLOG_INDEX}" --out "${OUT}"
  assert_equal "$(cell 1 6),$(cell 2 6)" ','
}

@test 'lists the other open issues a body names, once each and in order' {
  list_fixture '7|feat: a||See #12, #3 and #12 again; this is #7.' '3|feat: b||x' '12|feat: c||x'
  run "${BACKLOG_INDEX}" --out "${OUT}"
  assert_equal "$(cell 7 7)" '3,12'
}

@test 'drops a reference to an issue that is not open when every open issue is indexed' {
  list_fixture '7|feat: a||depends on #3 and on #99, which is closed' '3|feat: b||x'
  run "${BACKLOG_INDEX}" --out "${OUT}"
  assert_equal "$(cell 7 7)" '3'
}

@test 'keeps every reference when only named issues are indexed' {
  view_fixture OPEN '7|feat: a||depends on #3 and on #99'
  run "${BACKLOG_INDEX}" --out "${OUT}" 7
  assert_equal "$(cell 7 7)" '3,99'
}

@test 'does not read an issue number inside a fenced code block as a reference' {
  list_fixture $'7|feat: a||See #3.\n```text\nCloses #1\n```\nAnd #12.' '1|feat: x||x' '3|feat: b||x' '12|feat: c||x'
  run "${BACKLOG_INDEX}" --out "${OUT}"
  assert_equal "$(cell 7 7)" '3,12'
}

@test 'does not read headings or anchors as issue references' {
  list_fixture '7|feat: a||##2 heading, a page#5 anchor'
  run "${BACKLOG_INDEX}" --out "${OUT}"
  assert_equal "$(cell 7 7)" ''
}

@test 'marks an umbrella issue' {
  list_fixture '1|feat: a||Umbrella for lints' '2|feat: b||Filed as sub-issues' '3|feat: c||- [ ] do it in #9'
  run "${BACKLOG_INDEX}" --out "${OUT}"
  assert_equal "$(cell 1 8),$(cell 2 8),$(cell 3 8)" 'yes,yes,yes'
}

@test 'does not mark a plain task list as an umbrella' {
  list_fixture '1|feat: a||- [ ] write it'
  run "${BACKLOG_INDEX}" --out "${OUT}"
  assert_equal "$(cell 1 8)" ''
}

@test 'counts the comments of each issue' {
  list_fixture '1|feat: a||x' '2|feat: b||y'
  add_comments 1 'first comment' 'second comment'
  run "${BACKLOG_INDEX}" --out "${OUT}"
  assert_equal "$(cell 1 9),$(cell 2 9)" '2,0'
}

@test 'counts the comments of a named issue and keeps their bodies in the index' {
  view_fixture OPEN '5|fix: a||x'
  add_comments 5 'the one comment'
  run "${BACKLOG_INDEX}" --out "${OUT}" 5
  assert_equal "$(cell 5 9)" '1'
  assert_equal "$(jq --raw-output '.[0].comments[0].body' "${OUT}")" 'the one comment'
}

@test 'prints the body length in bytes' {
  list_fixture '1|feat: a||héllo'
  run "${BACKLOG_INDEX}" --out "${OUT}"
  assert_equal "$(cell 1 10)" '6'
}

@test 'counts the merged pull requests whose body holds a whole Part of line for the issue' {
  list_fixture '7|feat: a||x' '8|feat: b||x' '9|feat: c||x'
  pr_list_fixture $'Part of #7\nBacklog-Session: 2026-01-01' 'Part of #7' 'see Part of #8 here' 'Part of #88'
  run "${BACKLOG_INDEX}" --out "${OUT}"
  assert_success
  assert_equal "$(cell 7 11),$(cell 8 11),$(cell 9 11)" '2,0,0'
}

@test 'lists the merged pull requests once per run, searching for the Part of phrase' {
  list_fixture '1|feat: a||x' '2|feat: b||x'
  run "${BACKLOG_INDEX}" --out "${OUT}"
  assert_success
  run grep --count '^pr list --state merged --limit 1000 --search "Part of #" in:body --json number,body$' "${GH_LOG}"
  assert_output '1'
}

@test 'exits 1 when gh fails to list the merged pull requests' {
  list_fixture '1|feat: a||x'
  export GH_PR_LIST_FAILS=1
  run "${BACKLOG_INDEX}" --out "${OUT}"
  assert_failure 1
  assert_output --partial 'gh failed to list the merged pull requests'
}

@test 'keeps a title with a tab on one row' {
  list_fixture "1|feat: a$(printf '\t')b||x"
  run "${BACKLOG_INDEX}" --out "${OUT}"
  assert_success
  assert_equal "${#lines[@]}" '2'
}

@test 'views only the named issues' {
  view_fixture OPEN '5|fix: a||x'
  view_fixture OPEN '9|feat: b||y'
  run "${BACKLOG_INDEX}" --out "${OUT}" 9 5
  assert_success
  assert_equal "$(tail --lines=+2 <<< "${output}" | cut --fields=1,3 | paste --serial --delimiters=' ')" \
    "$(printf '5\tbug 9\tfeature')"
  run grep --count '^issue list' "${GH_LOG}"
  assert_output '0'
}

@test 'exits 1 when gh fails to list the issues' {
  run "${BACKLOG_INDEX}" --out "${OUT}"
  assert_failure 1
  assert_output --partial 'gh failed to list the issues'
}

@test 'exits 1 when gh fails to view one named issue' {
  view_fixture OPEN '5|fix: a||x'
  run "${BACKLOG_INDEX}" --out "${OUT}" 5 6
  assert_failure 1
  assert_output --partial 'gh failed to list the issues'
}
