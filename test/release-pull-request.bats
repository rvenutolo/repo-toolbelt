#!/usr/bin/env bats

# release-pull-request inside throwaway repositories, with the real git,
# git-cliff and markdownlint-cli2 and a stub gh. The stub records every argument
# list it is handed and answers the script's reads from files the test writes,
# so no call reaches GitHub. It knows the exact argument list of every read and
# the leading words of every write; any other call fails, so a read the script
# grows is a failing test here, not a silent blank.

bats_require_minimum_version 1.5.0

# The number the stub gives the open release pull request.
readonly PR='7'
# The commit the stub's createCommitOnBranch answers with.
readonly COMMIT='c0ffeec0ffeec0ffeec0ffeec0ffeec0ffee0001'
# The reads the script makes before it decides anything, as the stub logs them.
readonly READ_BRANCH='api repos/gh-owner/report --jq .default_branch'
readonly READ_HEAD='api repos/gh-owner/report/git/ref/heads/main --jq .object.sha'

function setup() {
  load test_helper/common
  load test_helper/release
  SCRIPT="${REPO_DIR}/.ci/release-pull-request"
  RELEASE_VERSION="${REPO_DIR}/.ci/release-version"
  write_skeleton
}

# @description Create a repository the script can run in: cliff.toml, the
#              .markdownlint-cli2.yaml, the changelog skeleton (committed), one commit, optionally a tag,
#              and `.stub/` beside them with the stub gh; see write_stub.
# @arg $1 tag the tag the first commit carries, or empty for none
# @set RELEASE_REPO string the repository's absolute path
# @set STUB string its stub directory
function pr_repo() {
  local -r tag="${1:-}"
  release_repo
  # Untracked, as cliff.toml is: the script reads it from the working tree.
  cp -- "${REPO_DIR}/.markdownlint-cli2.yaml" "${RELEASE_REPO}/.markdownlint-cli2.yaml"
  cp -- "${SKELETON}" "${RELEASE_REPO}/CHANGELOG.md"
  git -C "${RELEASE_REPO}" add -- CHANGELOG.md
  git -C "${RELEASE_REPO}" commit --quiet --amend --no-edit
  if [[ -n "${tag}" ]]; then
    git -C "${RELEASE_REPO}" tag "${tag}"
  fi
  STUB="${RELEASE_REPO}/.stub"
  write_stub "${STUB}"
  # The stub's own files must not count as changes to the tree.
  printf '.stub/\n' >> "${RELEASE_REPO}/.git/info/exclude"
}

# @description Write a gh stand-in that logs each call and answers the reads
#              from files in its directory:
#
#              - `main-sha`: the default branch's head; absent, the repository's
#                real HEAD
#              - `pr-list.json`: the open pull requests whose head branch is
#                chore/release, as `gh pr list --json` prints them
#              - `pr-title`: that pull request's title
#              - `branch-exists`, `staging-exists`: present when chore/release
#                or chore/release-staging exists
#              - `commit-oid`: what createCommitOnBranch answers, default COMMIT
#
#              A write only logs, and copies the files it was pointed at (the
#              changelog, the pull request's body) into the directory.
# @arg $1 dir directory to put the stub and its files in
function write_stub() {
  local -r dir="$1"
  local q_dir
  q_dir="$(printf '%q' "${dir}")"
  mkdir --parents "${dir}/bin"
  : > "${dir}/calls.log"
  printf '[]\n' > "${dir}/pr-list.json"
  cat > "${dir}/bin/gh" << STUB
#!/usr/bin/env bash
printf '%s\n' "\$*" >> ${q_dir}/calls.log
case "\$*" in
  'api repos/gh-owner/report --jq .default_branch') echo main ;;
  'api repos/gh-owner/report/git/ref/heads/main --jq .object.sha')
    if [[ -f ${q_dir}/main-sha ]]; then
      cat ${q_dir}/main-sha
    else
      git rev-parse HEAD
    fi
    ;;
  'api repos/gh-owner/report/git/ref/heads/chore/release') [[ -e ${q_dir}/branch-exists ]] ;;
  'api repos/gh-owner/report/git/ref/heads/chore/release-staging') [[ -e ${q_dir}/staging-exists ]] ;;
  'pr list --repo gh-owner/report --head chore/release --state open --limit 1000 --json number,isCrossRepository --jq '*)
    # The script's own filter, the last argument, is run over the list.
    jq --raw-output "\${!#}" ${q_dir}/pr-list.json
    ;;
  'pr view ${PR} --repo gh-owner/report --json title --jq .title') cat ${q_dir}/pr-title ;;
  'api --method PATCH '* | 'api --method POST '* | 'api --method DELETE '*) : ;;
  'api graphql '*)
    for arg in "\$@"; do
      if [[ "\${arg}" == 'changelog=@'* ]]; then
        cat -- "\${arg#changelog=@}" > ${q_dir}/changelog.b64
      fi
    done
    if [[ -f ${q_dir}/commit-oid ]]; then
      cat ${q_dir}/commit-oid
    else
      echo '${COMMIT}'
    fi
    ;;
  'pr edit '* | 'pr create '*)
    cat -- "\${!#}" > ${q_dir}/body.md
    ;;
  *)
    printf 'stub gh: unexpected call: %s\n' "\$*" >&2
    exit 1
    ;;
esac
STUB
  chmod +x "${dir}/bin/gh"
}

# @description Tell the stub that a release pull request is open.
# @arg $1 title the pull request's title
function open_pull_request() {
  local -r title="$1"
  printf '[{"number":%s,"isCrossRepository":false}]\n' "${PR}" > "${STUB}/pr-list.json"
  printf '%s\n' "${title}" > "${STUB}/pr-title"
  : > "${STUB}/branch-exists"
}

# @description Run the script in RELEASE_REPO with the stub gh first on PATH.
#              Leaves `status`, `output` and `stderr` from bats, and CALLS, the
#              stub's call log.
# @arg $@ arguments passed to the script
function run_script() {
  : > "${STUB}/calls.log"
  : > "${STUB}/changelog.b64"
  : > "${STUB}/body.md"
  cd "${RELEASE_REPO}" || return 1
  # The stub shadows the real gh; each test runs in its own process.
  PATH="${STUB}/bin:${PATH}" GITHUB_REPOSITORY='gh-owner/report' run --separate-stderr "${SCRIPT}" "$@"
  CALLS="$(< "${STUB}/calls.log")"
}

# @description Fail unless the call log contains a fixed string.
# @arg $1 needle
function assert_call() {
  local -r needle="$1"
  if [[ "${CALLS}" != *"${needle}"* ]]; then
    fail "no gh call contains '${needle}' (calls: ${CALLS})"
  fi
}

# @description Fail when the call log contains a fixed string.
# @arg $1 needle
function refute_call() {
  local -r needle="$1"
  if [[ "${CALLS}" == *"${needle}"* ]]; then
    fail "a gh call contains '${needle}' (calls: ${CALLS})"
  fi
}

# @description Fail unless the run made exactly these calls, in order. This is
#              how "wrote nothing" is asserted.
# @arg $@ the expected calls; none when gh must not be called at all
function assert_only_calls() {
  local want=''
  local call
  for call in "$@"; do
    want+="${want:+$'\n'}${call}"
  done
  if [[ "${CALLS}" != "${want}" ]]; then
    fail "gh calls were: ${CALLS:-(none)}; want exactly: ${want:-(none)}"
  fi
}

# @description Print the changelog the last run committed.
function committed_changelog() {
  base64 --decode < "${STUB}/changelog.b64"
}

@test 'a feature since the tag files a new pull request for the next minor version, and never merges it' {
  pr_repo v1.0.0
  commit_empty "${RELEASE_REPO}" 'feat: add widget'
  local head
  head="$(git -C "${RELEASE_REPO}" rev-parse HEAD)"
  run_script
  assert_success
  assert_call '--field headline=chore(release): v1.1.0'
  assert_call "--field oid=${head}"
  # The commit is made on the staging branch, created at the base, and the
  # release branch, which does not exist yet, is created at that commit.
  assert_call '--field branch=chore/release-staging'
  assert_call "api --method POST repos/gh-owner/report/git/refs --field ref=refs/heads/chore/release-staging --field sha=${head}"
  assert_call "api --method POST repos/gh-owner/report/git/refs --field ref=refs/heads/chore/release --field sha=${COMMIT}"
  refute_call "heads/chore/release --field sha=${head}"
  refute_call '--method PATCH'
  assert_call 'api --method DELETE repos/gh-owner/report/git/refs/heads/chore/release-staging'
  assert_call 'pr create --repo gh-owner/report --base main --head chore/release --title chore(release): v1.1.0 --body-file '
  refute_call 'pr edit'
  refute_call 'pr merge'
  refute_call '--auto'
  # The changelog travels as a file, never as one argument.
  assert_call '--field changelog=@'

  committed_changelog > "${BATS_TEST_TMPDIR}/CHANGELOG.md"
  run "${RELEASE_VERSION}" top "${BATS_TEST_TMPDIR}/CHANGELOG.md"
  assert_output 'v1.1.0'
  run grep --count --fixed-strings --line-regexp -e '- add widget' -e '# Changelog' "${BATS_TEST_TMPDIR}/CHANGELOG.md"
  assert_output 2

  # The body is the section as committed, then what merging does.
  local section body
  section="$("${RELEASE_VERSION}" notes "${BATS_TEST_TMPDIR}/CHANGELOG.md" v1.1.0)"
  body="$(< "${STUB}/body.md")"
  assert_equal "${body%%Merging*}" $'## v1.1.0\n\n'"${section}"$'\n\n'
  [[ "${body}" == *'Merging this pull request publishes v1.1.0: the release workflow builds the image'* ]]

  # The working tree's changelog is the input, never the output.
  run cmp -- "${SKELETON}" "${RELEASE_REPO}/CHANGELOG.md"
  assert_success
  run markdownlint-cli2 --config "${REPO_DIR}/.markdownlint-cli2.yaml" "${BATS_TEST_TMPDIR}/CHANGELOG.md"
  assert_success
  # The commit lands on a branch commitlint judges.
  run commitlint --config "${REPO_DIR}/.commitlintrc.yml" <<< 'chore(release): v1.1.0'
  assert_success
}

@test 'the committed changelog has the fixes markdownlint makes' {
  pr_repo v1.0.0
  # Three trailing spaces: MD009, which --fix strips.
  printf '# Changelog\n\nPreamble.   \n' > "${RELEASE_REPO}/CHANGELOG.md"
  git -C "${RELEASE_REPO}" commit --quiet --all --message 'docs: a preamble'
  commit_empty "${RELEASE_REPO}" 'fix: a'
  run_script
  assert_success
  committed_changelog > "${BATS_TEST_TMPDIR}/CHANGELOG.md"
  run grep --count --line-regexp 'Preamble\.' "${BATS_TEST_TMPDIR}/CHANGELOG.md"
  assert_output 1
}

@test 'a changelog markdownlint rejects and cannot fix is refused before any write' {
  pr_repo v1.0.0
  # MD013 at 120 columns, which --fix leaves as it is.
  printf '# Changelog\n\n%s\n' "$(printf 'word%s ' {1..30})" > "${RELEASE_REPO}/CHANGELOG.md"
  git -C "${RELEASE_REPO}" commit --quiet --all --message 'docs: a long line'
  commit_empty "${RELEASE_REPO}" 'fix: a'
  run_script
  assert_failure 1
  assert_stderr --partial 'MD013'
  assert_stderr --partial 'markdownlint rejects the assembled changelog'
  assert_only_calls "${READ_BRANCH}" "${READ_HEAD}" \
    'pr list --repo gh-owner/report --head chore/release --state open --limit 1000 --json number,isCrossRepository --jq [.[] | select(.isCrossRepository == false)][0].number // empty'
}

@test 'a changelog whose first section is not the version is refused before any write' {
  pr_repo v1.0.0
  commit_empty "${RELEASE_REPO}" 'fix: a'
  mkdir "${BATS_TEST_TMPDIR}/bin"
  # A formatter that leaves a file with no release section in it. Its last
  # argument is the file, behind the `:` that makes it a literal.
  cat > "${BATS_TEST_TMPDIR}/bin/markdownlint-cli2" << 'STUB'
#!/usr/bin/env bash
printf '# Changelog\n' > "${!#:1}"
STUB
  chmod +x "${BATS_TEST_TMPDIR}/bin/markdownlint-cli2"
  PATH="${BATS_TEST_TMPDIR}/bin:${PATH}" run_script
  assert_failure 1
  assert_stderr --partial "the assembled changelog's first section is missing, not v1.0.1"
  assert_only_calls "${READ_BRANCH}" "${READ_HEAD}" \
    'pr list --repo gh-owner/report --head chore/release --state open --limit 1000 --json number,isCrossRepository --jq [.[] | select(.isCrossRepository == false)][0].number // empty'
}

@test 'a release branch left behind is moved onto the new commit, in order, and a pull request opened' {
  pr_repo v1.0.0
  : > "${STUB}/branch-exists"
  commit_empty "${RELEASE_REPO}" 'feat: add widget'
  local head
  head="$(git -C "${RELEASE_REPO}" rev-parse HEAD)"
  run_script
  assert_success
  local -r stage="api --method POST repos/gh-owner/report/git/refs --field ref=refs/heads/chore/release-staging --field sha=${head}"
  local -r move="api --method PATCH repos/gh-owner/report/git/refs/heads/chore/release --field sha=${COMMIT} --field force=true"
  assert_call "${stage}"
  assert_call "${move}"
  refute_call 'ref=refs/heads/chore/release --field'
  refute_call "heads/chore/release --field sha=${head}"
  refute_call 'pr edit'
  local rest="${CALLS#*"${stage}"}"
  rest="${rest#*'api graphql '}"
  rest="${rest#*"${move}"}"
  rest="${rest#*'api --method DELETE repos/gh-owner/report/git/refs/heads/chore/release-staging'}"
  [[ "${rest}" == *'pr create --repo gh-owner/report --base main --head chore/release --title chore(release): v1.1.0 '* ]]
}

@test 'a staging branch left behind is moved back onto the base' {
  pr_repo v1.0.0
  open_pull_request 'chore(release): v1.1.0'
  : > "${STUB}/staging-exists"
  commit_empty "${RELEASE_REPO}" 'feat: add widget'
  local head
  head="$(git -C "${RELEASE_REPO}" rev-parse HEAD)"
  run_script
  assert_success
  assert_call "api --method PATCH repos/gh-owner/report/git/refs/heads/chore/release-staging --field sha=${head} --field force=true"
  refute_call '--method POST'
  assert_call "--field oid=${head}"
  assert_call "pr edit ${PR} "
}

@test 'a commit the API does not name is a failure, and the branch and pull request are left alone' {
  pr_repo v1.0.0
  open_pull_request 'chore(release): v1.1.0'
  : > "${STUB}/commit-oid"
  commit_empty "${RELEASE_REPO}" 'feat: add widget'
  run_script
  assert_failure 1
  assert_stderr --partial 'createCommitOnBranch returned no commit'
  refute_call 'heads/chore/release --field'
  refute_call 'pr edit'
  refute_call 'pr create'
}

@test 'a subject full of metacharacters reaches the changelog and the body as literal text' {
  pr_repo v1.0.0
  # shellcheck disable=SC2016 # the point: nothing in the subject expands
  commit_empty "${RELEASE_REPO}" 'feat: say "hi" to `x`, $(echo PAYLOAD_RAN), $HOME and <!-- x -->'
  run_script
  assert_success
  local changelog body text
  changelog="$(committed_changelog)"
  body="$(< "${STUB}/body.md")"
  for text in "${changelog}" "${body}"; do
    # shellcheck disable=SC2016 # literal on purpose, as above
    [[ "${text}" == *'say "hi" to `x`, $(echo PAYLOAD_RAN), $HOME and \<!-- x -->'* ]]
  done
  printf '%s\n' "${changelog}" > "${BATS_TEST_TMPDIR}/CHANGELOG.md"
  run "${RELEASE_VERSION}" top "${BATS_TEST_TMPDIR}/CHANGELOG.md"
  assert_output 'v1.1.0'
}

@test 'nothing releasable and no pull request is nothing to do' {
  pr_repo v1.0.0
  commit_empty "${RELEASE_REPO}" 'docs: tidy'
  run_script
  assert_success
  assert_stderr --partial 'nothing to release'
  assert_only_calls "${READ_BRANCH}" "${READ_HEAD}" \
    'pr list --repo gh-owner/report --head chore/release --state open --limit 1000 --json number,isCrossRepository --jq [.[] | select(.isCrossRepository == false)][0].number // empty'
}

@test 'a version given by hand is the version' {
  pr_repo v1.0.0
  commit_empty "${RELEASE_REPO}" 'feat: add widget'
  run_script --version v3.0.0
  assert_success
  assert_call '--field headline=chore(release): v3.0.0'
  assert_call '--title chore(release): v3.0.0'
  local changelog
  changelog="$(committed_changelog)"
  [[ "${changelog}" == *'## v3.0.0'* && "${changelog}" == *'- add widget'* ]]
}

@test 'a version given over commits that release nothing gets a section that says so' {
  pr_repo v1.0.0
  commit_empty "${RELEASE_REPO}" 'docs: tidy'
  run_script --version v3.0.0
  assert_success
  local changelog
  changelog="$(committed_changelog)"
  [[ "${changelog}" == *$'## v3.0.0\n\nNo user-facing changes.'* ]]
  [[ "${changelog}" != *'tidy'* ]]
}

@test 'a version given on this run replaces the open pull request version, even a higher one' {
  pr_repo v1.0.0
  open_pull_request 'chore(release): v3.0.0'
  commit_empty "${RELEASE_REPO}" 'feat: add widget'
  run_script --version v2.0.0
  assert_success
  assert_call '--field headline=chore(release): v2.0.0'
  assert_call "pr edit ${PR} --repo gh-owner/report --title chore(release): v2.0.0 --body-file "
  refute_call 'v3.0.0'
  refute_call 'pr create'
}

@test 'the open pull request version is not lowered by a push' {
  pr_repo v1.0.0
  open_pull_request 'chore(release): v3.0.0'
  commit_empty "${RELEASE_REPO}" 'fix: pad columns'
  run_script
  assert_success
  assert_stderr --partial 'keeping v3.0.0'
  assert_call '--field headline=chore(release): v3.0.0'
  assert_call "pr edit ${PR} --repo gh-owner/report --title chore(release): v3.0.0 --body-file "
  assert_call "api --method PATCH repos/gh-owner/report/git/refs/heads/chore/release --field sha=${COMMIT} --field force=true"
  refute_call 'pr create'
  refute_call 'ref=refs/heads/chore/release --field'
}

@test 'with only non-releasing commits the open pull request is still refreshed under its version' {
  pr_repo v1.0.0
  open_pull_request 'chore(release): v3.0.0'
  commit_empty "${RELEASE_REPO}" 'docs: tidy'
  run_script
  assert_success
  assert_call '--field headline=chore(release): v3.0.0'
  assert_call "pr edit ${PR} "
  refute_call 'pr create'
}

@test 'the open pull request version is raised when the commits call for more' {
  pr_repo v1.0.0
  open_pull_request 'chore(release): v1.0.1'
  commit_empty "${RELEASE_REPO}" 'fix: pad columns'
  commit_empty "${RELEASE_REPO}" 'feat: add widget'
  run_script
  assert_success
  assert_call '--field headline=chore(release): v1.1.0'
  assert_call "pr edit ${PR} --repo gh-owner/report --title chore(release): v1.1.0 --body-file "
  refute_call 'pr create'
}

@test 'an open pull request whose title names no version holds nothing to keep' {
  pr_repo v1.0.0
  open_pull_request 'chore(release): v9.0'
  commit_empty "${RELEASE_REPO}" 'fix: pad columns'
  run_script
  assert_success
  assert_call '--field headline=chore(release): v1.0.1'
  assert_call "pr edit ${PR} "
}

@test 'an open pull request whose title is not a release title holds nothing to keep' {
  pr_repo v1.0.0
  open_pull_request 'feat: something else'
  commit_empty "${RELEASE_REPO}" 'fix: pad columns'
  run_script
  assert_success
  assert_call '--field headline=chore(release): v1.0.1'
}

@test 'an open pull request version that has been released since holds nothing to keep' {
  pr_repo v1.0.0
  open_pull_request 'chore(release): v1.0.0'
  commit_empty "${RELEASE_REPO}" 'fix: pad columns'
  run_script
  assert_success
  assert_call '--field headline=chore(release): v1.0.1'
}

@test 'a pull request from a fork is never adopted' {
  pr_repo v1.0.0
  printf '[{"number":99,"isCrossRepository":true}]\n' > "${STUB}/pr-list.json"
  printf 'chore(release): v99.0.0\n' > "${STUB}/pr-title"
  commit_empty "${RELEASE_REPO}" 'feat: add widget'
  run_script
  assert_success
  assert_call 'pr create --repo gh-owner/report --base main --head chore/release --title chore(release): v1.1.0 '
  refute_call 'pr edit'
  refute_call 'pr view'
  refute_call 'v99.0.0'
}

@test 'a pull request from a fork listed first is passed over for our own' {
  pr_repo v1.0.0
  open_pull_request 'chore(release): v3.0.0'
  printf '[{"number":99,"isCrossRepository":true},{"number":%s,"isCrossRepository":false}]\n' "${PR}" \
    > "${STUB}/pr-list.json"
  commit_empty "${RELEASE_REPO}" 'fix: pad columns'
  run_script
  assert_success
  assert_call "pr edit ${PR} --repo gh-owner/report --title chore(release): v3.0.0 "
  refute_call ' 99 '
  refute_call 'pr create'
}

@test 'a pull request whose origin is not stated is not taken for ours' {
  pr_repo v1.0.0
  printf '[{"number":98}]\n' > "${STUB}/pr-list.json"
  commit_empty "${RELEASE_REPO}" 'feat: add widget'
  run_script
  assert_success
  assert_call 'pr create --repo gh-owner/report --base main --head chore/release --title chore(release): v1.1.0 '
  refute_call 'pr view'
  refute_call 'pr edit'
}

@test 'a version that is not above the last release, or whose tag exists, is refused before any write' {
  pr_repo v1.0.0
  commit_empty "${RELEASE_REPO}" 'feat: add widget'
  run_script --version v1.0.0
  assert_failure 1
  assert_stderr --partial 'is not above the last release'
  assert_only_calls "${READ_BRANCH}" "${READ_HEAD}"
  run_script --version v0.9.0
  assert_failure 1
  assert_only_calls "${READ_BRANCH}" "${READ_HEAD}"

  git -C "${RELEASE_REPO}" switch --quiet --create side
  commit_empty "${RELEASE_REPO}" 'feat!: side'
  git -C "${RELEASE_REPO}" tag v2.0.0
  git -C "${RELEASE_REPO}" switch --quiet main
  run_script --version v2.0.0
  assert_failure 1
  assert_stderr --partial 'the tag v2.0.0 already exists'
  assert_only_calls "${READ_BRANCH}" "${READ_HEAD}"
}

@test 'a version that can never be released is refused even while another release is being published' {
  pr_repo v1.0.0
  commit_empty "${RELEASE_REPO}" 'feat: add widget'
  git -C "${RELEASE_REPO}" switch --quiet --create side
  commit_empty "${RELEASE_REPO}" 'feat!: side'
  git -C "${RELEASE_REPO}" tag v2.0.0
  git -C "${RELEASE_REPO}" switch --quiet main
  printf '# Changelog\n\n## v1.1.0\n\n### Features\n\n- add widget\n' > "${RELEASE_REPO}/CHANGELOG.md"
  git -C "${RELEASE_REPO}" commit --quiet --all --message 'chore(release): v1.1.0'
  local version
  for version in v1.0.0 v0.9.0 v2.0.0; do
    run_script --version "${version}"
    assert_failure 1
    assert_only_calls "${READ_BRANCH}" "${READ_HEAD}"
  done
}

@test 'a computed version whose tag exists elsewhere is refused' {
  pr_repo v1.0.0
  git -C "${RELEASE_REPO}" switch --quiet --create side
  commit_empty "${RELEASE_REPO}" 'chore: side'
  git -C "${RELEASE_REPO}" tag v1.0.1
  git -C "${RELEASE_REPO}" switch --quiet main
  commit_empty "${RELEASE_REPO}" 'fix: a'
  run_script
  assert_failure 1
  assert_stderr --partial 'the tag v1.0.1 already exists'
  refute_call 'graphql'
}

@test 'a malformed call is refused before anything is asked of GitHub' {
  pr_repo v1.0.0
  commit_empty "${RELEASE_REPO}" 'feat: add widget'
  local version
  for version in '1.1.0' 'v1.1' 'v1.1.0-rc.1' ''; do
    run_script --version "${version}"
    assert_failure 2
    assert_only_calls
  done
  run_script --version
  assert_failure 2
  assert_stderr --partial '--version needs a version'
  assert_only_calls
  run_script --bogus
  assert_failure 2
  assert_stderr --partial 'unknown argument --bogus'
  assert_only_calls
}

@test 'outside a git repository the script fails before asking GitHub' {
  pr_repo v1.0.0
  mkdir "${BATS_TEST_TMPDIR}/not-a-repo"
  : > "${STUB}/calls.log"
  cd "${BATS_TEST_TMPDIR}/not-a-repo" || return 1
  PATH="${STUB}/bin:${PATH}" GITHUB_REPOSITORY='gh-owner/report' run --separate-stderr "${SCRIPT}"
  assert_failure 1
  assert_stderr --partial 'not inside a git repository'
  assert_equal "$(< "${STUB}/calls.log")" ''
}

@test 'between the merge of a release pull request and its tag, the next release is not proposed' {
  pr_repo v1.0.0
  commit_empty "${RELEASE_REPO}" 'feat: add widget'
  printf '# Changelog\n\n## v1.1.0\n\n### Features\n\n- add widget\n' > "${RELEASE_REPO}/CHANGELOG.md"
  git -C "${RELEASE_REPO}" commit --quiet --all --message 'chore(release): v1.1.0'
  commit_empty "${RELEASE_REPO}" 'fix: pad columns'
  run_script
  assert_success
  assert_stderr --partial 'release v1.1.0 is being published'
  assert_only_calls "${READ_BRANCH}" "${READ_HEAD}"
}

@test 'a withdrawn version can be proposed again, by hand or as computed' {
  pr_repo v1.0.0
  commit_empty "${RELEASE_REPO}" 'feat: add widget'
  printf '# Changelog\n\n## v1.1.0\n\n### Features\n\n- add widget\n' > "${RELEASE_REPO}/CHANGELOG.md"
  git -C "${RELEASE_REPO}" commit --quiet --all --message 'chore(release): v1.1.0'
  cp -- "${SKELETON}" "${RELEASE_REPO}/CHANGELOG.md"
  git -C "${RELEASE_REPO}" commit --quiet --all --message 'chore: withdraw v1.1.0'
  run_script --version v1.1.0
  assert_success
  assert_call '--field headline=chore(release): v1.1.0'
  run_script
  assert_success
  assert_call '--field headline=chore(release): v1.1.0'
}

@test 'a first changelog heading that is not a version is a failure, and nothing is written' {
  pr_repo v1.0.0
  commit_empty "${RELEASE_REPO}" 'feat: add widget'
  printf '# Changelog\n\n## Unreleased\n\n- add widget\n' > "${RELEASE_REPO}/CHANGELOG.md"
  git -C "${RELEASE_REPO}" commit --quiet --all --message 'docs: by hand'
  run_script
  assert_failure 1
  assert_only_calls "${READ_BRANCH}" "${READ_HEAD}"
}

@test 'an uncommitted changelog edit is refused before any write' {
  pr_repo v1.0.0
  commit_empty "${RELEASE_REPO}" 'feat: add widget'
  printf '\nA line nobody committed.\n' >> "${RELEASE_REPO}/CHANGELOG.md"
  run_script
  assert_failure 1
  assert_stderr --partial 'CHANGELOG.md is not as it was committed'
  assert_only_calls "${READ_BRANCH}" "${READ_HEAD}"
}

@test 'a stale checkout is a failure, or nothing to do under --skip-if-stale' {
  pr_repo v1.0.0
  commit_empty "${RELEASE_REPO}" 'feat: add widget'
  printf 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa0001\n' > "${STUB}/main-sha"
  run_script
  assert_failure 1
  assert_stderr --partial 'HEAD is '
  assert_only_calls "${READ_BRANCH}" "${READ_HEAD}"
  run_script --skip-if-stale
  assert_success
  assert_stderr --partial 'a newer push supersedes this run'
  assert_only_calls "${READ_BRANCH}" "${READ_HEAD}"
}

@test '--skip-if-stale on a current checkout changes nothing, and still refuses a bad version' {
  pr_repo v1.0.0
  commit_empty "${RELEASE_REPO}" 'feat: add widget'
  run_script --skip-if-stale
  assert_success
  assert_call 'pr create --repo gh-owner/report --base main --head chore/release --title chore(release): v1.1.0 '
  run_script --skip-if-stale --version v1.0.0
  assert_failure 1
  assert_only_calls "${READ_BRANCH}" "${READ_HEAD}"
}

@test 'a missing GITHUB_REPOSITORY fails before asking GitHub' {
  pr_repo v1.0.0
  commit_empty "${RELEASE_REPO}" 'feat: add widget'
  : > "${STUB}/calls.log"
  cd "${RELEASE_REPO}" || return 1
  PATH="${STUB}/bin:${PATH}" run --separate-stderr env --unset=GITHUB_REPOSITORY "${SCRIPT}"
  assert_failure 1
  assert_stderr --partial 'GITHUB_REPOSITORY is not set'
  assert_equal "$(< "${STUB}/calls.log")" ''
}
