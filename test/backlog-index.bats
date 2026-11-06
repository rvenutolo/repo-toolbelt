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
       body: ($f[3:] | join("|"))}'
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
  assert_line --index 0 "$(printf 'number\tstate\tkind\tlabels\ttitle\trepro\trefs\tumbrella\tbytes')"
}

@test 'writes the full issue JSON to the --out file' {
  list_fixture '1|feat: a|enhancement|first body' '2|fix: b||second body'
  run "${BACKLOG_INDEX}" --out "${OUT}"
  assert_success
  assert_equal "$(jq --sort-keys '.' "${OUT}")" "$(jq --sort-keys '.' "${FIXTURES}/list.json")"
}

@test 'writes to .claude/tmp/issues.json under the working directory by default' {
  list_fixture '1|feat: a||body'
  cd "${BATS_TEST_TMPDIR}"
  run "${BACKLOG_INDEX}"
  assert_success
  assert_equal "$(jq '.[0].number' "${BATS_TEST_TMPDIR}/.claude/tmp/issues.json")" '1'
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

@test 'does not read a feature body that says reproducible as a reproduction' {
  list_fixture '1|feat: a||Builds must be reproducible' '2|feat: b||ensures reproducibility of the image'
  run "${BACKLOG_INDEX}" --out "${OUT}"
  assert_equal "$(cell 1 6),$(cell 2 6)" ','
}

@test 'lists the other issues a body names, once each and in order' {
  list_fixture '7|feat: a||See #12, #3 and #12 again; this is #7.'
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

@test 'prints the body length in bytes' {
  list_fixture '1|feat: a||héllo'
  run "${BACKLOG_INDEX}" --out "${OUT}"
  assert_equal "$(cell 1 9)" '6'
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
