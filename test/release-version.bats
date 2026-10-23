#!/usr/bin/env bats

# release-version against throwaway repositories with the real git and the real
# git-cliff. No stub: what is under test is what git-cliff makes of this repo's
# cliff.toml, and a stub could only repeat the expectation back.

bats_require_minimum_version 1.5.0

function setup() {
  load test_helper/common
  load test_helper/release
  SCRIPT="${REPO_DIR}/.ci/release-version"
  write_skeleton
  cd "${BATS_TEST_TMPDIR}" || return 1
}

# @description Run release-version in a directory, stdout and stderr apart.
# @arg $1 dir the directory to run in
# @arg $@ arguments passed to release-version
function run_in() {
  local -r dir="$1"
  shift
  cd "${dir}" || return 1
  run --separate-stderr "${SCRIPT}" "$@"
}

@test 'validate accepts exactly vMAJOR.MINOR.PATCH' {
  local version
  for version in 'v1.2.3' 'v0.0.0' 'v10.20.30'; do
    run_in "${BATS_TEST_TMPDIR}" validate "${version}"
    assert_success
    assert_output ''
  done
}

@test 'validate rejects anything else as a usage error' {
  local version
  for version in '1.2.3' 'v1.2' 'v1.2.3-rc.1' 'v01.2.3' 'vx.y.z' '' 'v1.2.3.4' ' v1.2.3' $'v1.2.3\nv1.2.4'; do
    run_in "${BATS_TEST_TMPDIR}" validate "${version}"
    assert_failure 2
    assert_stderr --partial 'is not vMAJOR.MINOR.PATCH'
  done
}

@test 'last-tag prints nothing when there is no tag' {
  release_repo
  run_in "${RELEASE_REPO}" last-tag
  assert_success
  assert_output ''
}

@test 'last-tag orders by number and ignores tags that are not releases or not reachable' {
  release_repo
  git -C "${RELEASE_REPO}" tag v1.9.0
  commit_empty "${RELEASE_REPO}" 'fix: a'
  git -C "${RELEASE_REPO}" tag v1.10.0
  run_in "${RELEASE_REPO}" last-tag
  assert_output 'v1.10.0'

  commit_empty "${RELEASE_REPO}" 'fix: b'
  git -C "${RELEASE_REPO}" tag nightly
  git -C "${RELEASE_REPO}" tag v2.0.0-rc.1
  git -C "${RELEASE_REPO}" tag v3.0
  run_in "${RELEASE_REPO}" last-tag
  assert_output 'v1.10.0'

  git -C "${RELEASE_REPO}" switch --quiet --create side
  commit_empty "${RELEASE_REPO}" 'fix: c'
  git -C "${RELEASE_REPO}" tag v9.0.0
  git -C "${RELEASE_REPO}" switch --quiet main
  run_in "${RELEASE_REPO}" last-tag
  assert_success
  assert_output 'v1.10.0'
}

@test 'last-tag prefers the higher version over the newer tag, from a subdirectory too' {
  release_repo v2.0.0
  commit_empty "${RELEASE_REPO}" 'fix: a'
  git -C "${RELEASE_REPO}" tag v1.9.1
  run_in "${RELEASE_REPO}" last-tag
  assert_output 'v2.0.0'
  mkdir "${RELEASE_REPO}/sub"
  run_in "${RELEASE_REPO}/sub" last-tag
  assert_success
  assert_output 'v2.0.0'
}

@test 'last-tag outside a repository fails' {
  mkdir "${BATS_TEST_TMPDIR}/not-a-repo"
  run_in "${BATS_TEST_TMPDIR}/not-a-repo" last-tag
  assert_failure 1
  assert_stderr --partial 'not inside a git repository'
}

@test 'last-tag in a repository with no commit fails' {
  git init --quiet "${BATS_TEST_TMPDIR}/empty"
  run_in "${BATS_TEST_TMPDIR}/empty" last-tag
  assert_failure 1
  assert_stderr --partial 'the repository has no commit'
}

@test 'a shallow clone is refused by every subcommand that reads history' {
  release_repo v1.0.0
  commit_empty "${RELEASE_REPO}" 'fix: a'
  commit_empty "${RELEASE_REPO}" 'fix: b'
  local -r shallow="${BATS_TEST_TMPDIR}/shallow"
  git clone --quiet --depth 1 "file://${RELEASE_REPO}" "${shallow}"
  cp -- "${REPO_DIR}/cliff.toml" "${shallow}/cliff.toml"
  run_in "${shallow}" last-tag
  assert_failure 1
  assert_stderr --partial 'shallow clone'
  run_in "${shallow}" next
  assert_failure 1
  assert_stderr --partial 'shallow clone'
  run_in "${shallow}" section v1.0.1
  assert_failure 1
  assert_stderr --partial 'shallow clone'
  # What does not read history still works there.
  run_in "${shallow}" validate v1.0.0
  assert_success
  run_in "${shallow}" top "${SKELETON}"
  assert_success
  run_in "${RELEASE_REPO}" next
  assert_output 'v1.0.1'
}

@test 'a shallow-clone check that cannot be answered is a failure for every subcommand that reads history' {
  release_repo v1.0.0
  write_git_stub "${BATS_TEST_TMPDIR}/bin" '*--is-shallow-repository*' 2
  PATH="${BATS_TEST_TMPDIR}/bin:${PATH}" run_in "${RELEASE_REPO}" last-tag
  assert_failure 1
  assert_stderr --partial 'cannot tell whether the repository is a shallow clone'
  PATH="${BATS_TEST_TMPDIR}/bin:${PATH}" run_in "${RELEASE_REPO}" next
  assert_failure 1
  assert_stderr --partial 'cannot tell whether the repository is a shallow clone'
  PATH="${BATS_TEST_TMPDIR}/bin:${PATH}" run_in "${RELEASE_REPO}" section v1.0.1
  assert_failure 1
  assert_stderr --partial 'cannot tell whether the repository is a shallow clone'
}

@test 'greater compares by number, strictly' {
  run_in "${BATS_TEST_TMPDIR}" greater v1.0.0 v0.1.0
  assert_success
  run_in "${BATS_TEST_TMPDIR}" greater v1.0.0 v1.0.0
  assert_failure 1
  run_in "${BATS_TEST_TMPDIR}" greater v1.9.0 v1.10.0
  assert_failure 1
  run_in "${BATS_TEST_TMPDIR}" greater v1.10.0 v1.9.0
  assert_success
  run_in "${BATS_TEST_TMPDIR}" greater v1.0.10 v1.0.9
  assert_success
  run_in "${BATS_TEST_TMPDIR}" greater v10.0.0 v9.99.99
  assert_success
  run_in "${BATS_TEST_TMPDIR}" greater v0.9.9 v1.0.0
  assert_failure 1
  # A verdict, not a crash: the ERR trap stays quiet.
  refute_stderr --partial 'ERROR: line'
  run_in "${BATS_TEST_TMPDIR}" greater v1.0.0 1.0.0
  assert_failure 2
}

@test 'next bumps patch, minor and major as the commits call for' {
  release_repo v1.0.0
  run_in "${RELEASE_REPO}" next
  assert_success
  assert_output ''
  commit_empty "${RELEASE_REPO}" 'fix: a'
  run_in "${RELEASE_REPO}" next
  assert_output 'v1.0.1'
  commit_empty "${RELEASE_REPO}" 'feat: b'
  run_in "${RELEASE_REPO}" next
  assert_output 'v1.1.0'
  commit_empty "${RELEASE_REPO}" 'feat!: c'
  run_in "${RELEASE_REPO}" next
  assert_output 'v2.0.0'

  release_repo v1.0.0
  commit_empty "${RELEASE_REPO}" 'fix: a' 'BREAKING CHANGE: x'
  run_in "${RELEASE_REPO}" next
  assert_output 'v2.0.0'

  release_repo v1.0.0
  commit_empty "${RELEASE_REPO}" 'perf: a'
  run_in "${RELEASE_REPO}" next
  assert_output 'v1.0.1'
}

@test 'next releases nothing for types that release nothing, until one is breaking' {
  release_repo v1.0.0
  local subject
  for subject in 'docs: a' 'chore: b' 'ci: c' 'test: d' 'style: e' 'build: f' 'refactor: g' \
    'chore(release): v1.0.1' 'not a conventional subject'; do
    commit_empty "${RELEASE_REPO}" "${subject}"
  done
  run_in "${RELEASE_REPO}" next
  assert_success
  assert_output ''
  commit_empty "${RELEASE_REPO}" 'docs!: h'
  run_in "${RELEASE_REPO}" next
  assert_output 'v2.0.0'
}

@test 'next makes no exception below 1.0.0' {
  release_repo v0.1.0
  commit_empty "${RELEASE_REPO}" 'fix: a'
  run_in "${RELEASE_REPO}" next
  assert_output 'v0.1.1'
  commit_empty "${RELEASE_REPO}" 'feat: b'
  run_in "${RELEASE_REPO}" next
  assert_output 'v0.2.0'
  commit_empty "${RELEASE_REPO}" 'feat!: c'
  run_in "${RELEASE_REPO}" next
  assert_output 'v1.0.0'
}

@test 'next counts from last-tag, never from a tag git-cliff picks itself' {
  release_repo v1.0.0
  git -C "${RELEASE_REPO}" switch --quiet --create side
  commit_empty "${RELEASE_REPO}" 'feat!: side'
  git -C "${RELEASE_REPO}" tag v5.0.0
  git -C "${RELEASE_REPO}" switch --quiet main
  commit_empty "${RELEASE_REPO}" 'fix: a'
  git -C "${RELEASE_REPO}" tag v1.0.1-rc.1
  run_in "${RELEASE_REPO}" next
  assert_output 'v1.0.1'
  mkdir "${RELEASE_REPO}/sub"
  run_in "${RELEASE_REPO}/sub" next
  assert_output 'v1.0.1'

  release_repo v2.0.0
  commit_empty "${RELEASE_REPO}" 'fix: a'
  git -C "${RELEASE_REPO}" tag v1.9.1
  commit_empty "${RELEASE_REPO}" 'fix: b'
  run_in "${RELEASE_REPO}" next
  assert_output 'v2.0.1'
}

@test 'next with no tag starts at the initial version' {
  release_repo
  run_in "${RELEASE_REPO}" next
  assert_success
  assert_output ''
  commit_empty "${RELEASE_REPO}" 'feat: a'
  run_in "${RELEASE_REPO}" next
  assert_output 'v0.1.0'
}

@test 'next without a cliff.toml fails' {
  release_repo v1.0.0
  commit_empty "${RELEASE_REPO}" 'fix: a'
  rm -- "${RELEASE_REPO}/cliff.toml"
  run_in "${RELEASE_REPO}" next
  assert_failure 1
  assert_stderr --partial 'no cliff.toml at the top of'
}

@test 'a git-cliff failure is reported with its own message' {
  release_repo v1.0.0
  commit_empty "${RELEASE_REPO}" 'fix: a'
  printf 'not = [valid toml\n' > "${RELEASE_REPO}/cliff.toml"
  run_in "${RELEASE_REPO}" next
  assert_failure 1
  assert_stderr --partial 'git-cliff exited'
}

# @description Put a git-cliff stand-in first on PATH that lists one entry for
#              any section and answers --bumped-version with a fixed text.
# @arg $1 bumped what --bumped-version prints
function fake_cliff() {
  local -r bumped="$1"
  mkdir --parents "${BATS_TEST_TMPDIR}/bin"
  cat > "${BATS_TEST_TMPDIR}/bin/git-cliff" << STUB
#!/usr/bin/env bash
for arg in "\$@"; do
  if [[ "\${arg}" == '--bumped-version' ]]; then
    printf '%s\\n' '${bumped}'
    exit 0
  fi
done
printf '## v0.0.0\\n\\n- a\\n'
STUB
  chmod +x "${BATS_TEST_TMPDIR}/bin/git-cliff"
  # Shadows the real git-cliff; each test runs in its own process, so it goes no further.
  PATH="${BATS_TEST_TMPDIR}/bin:${PATH}"
}

@test 'next refuses a version git-cliff computes that is not a release version' {
  release_repo v1.0.0
  commit_empty "${RELEASE_REPO}" 'fix: a'
  fake_cliff '1.0.1'
  run_in "${RELEASE_REPO}" next
  assert_failure 1
  assert_stderr --partial 'which is not vMAJOR.MINOR.PATCH'
}

@test 'next refuses a version git-cliff computes that is not above the last tag' {
  release_repo v1.0.0
  commit_empty "${RELEASE_REPO}" 'fix: a'
  fake_cliff 'v1.0.0'
  run_in "${RELEASE_REPO}" next
  assert_failure 1
  assert_stderr --partial 'which is not above v1.0.0'
}

@test 'section groups and orders the commits since the tag' {
  release_repo v1.0.0
  commit_empty "${RELEASE_REPO}" 'feat: add widget'
  commit_empty "${RELEASE_REPO}" 'fix(render): pad columns'
  commit_empty "${RELEASE_REPO}" 'docs: tidy'
  commit_empty "${RELEASE_REPO}" 'chore(release): v1.0.0'
  commit_empty "${RELEASE_REPO}" 'feat!: drop gadget' 'BREAKING CHANGE: remove gadget from repos.toml'
  commit_empty "${RELEASE_REPO}" 'perf: read once'
  commit_empty "${RELEASE_REPO}" 'feat(cli)!: rename flag'
  commit_empty "${RELEASE_REPO}" 'fix: quote names' $'BREAKING CHANGE: names print quoted\nand the old form is gone'
  commit_empty "${RELEASE_REPO}" 'feat: add sprocket'
  run_in "${RELEASE_REPO}" section v2.0.0
  assert_success
  # The whole section, so the blank lines, the bold scope, the indented footer
  # and the once-only listing of a breaking commit are all pinned.
  assert_output "$(
    cat << 'EOF'
## v2.0.0

### Breaking changes

- drop gadget

  remove gadget from repos.toml

- **cli:** rename flag
- quote names

  names print quoted
  and the old form is gone

### Features

- add widget
- add sprocket

### Fixes

- **render:** pad columns
- read once
EOF
  )"
}

@test 'section rejects a version that is not a release version' {
  release_repo v1.0.0
  run_in "${RELEASE_REPO}" section 2.0.0
  assert_failure 2
}

@test 'section omits a group with no commits, and is the heading alone with nothing to release' {
  release_repo v1.0.0
  commit_empty "${RELEASE_REPO}" 'fix: a'
  run_in "${RELEASE_REPO}" section v1.0.1
  assert_output $'## v1.0.1\n\n### Fixes\n\n- a'

  release_repo v1.0.0
  commit_empty "${RELEASE_REPO}" 'docs: a'
  run_in "${RELEASE_REPO}" section v1.0.1
  assert_success
  assert_output '## v1.0.1'
}

@test 'a type is matched whole' {
  release_repo v1.0.0
  commit_empty "${RELEASE_REPO}" 'fixture: not a fix'
  commit_empty "${RELEASE_REPO}" 'feature: not a feat'
  commit_empty "${RELEASE_REPO}" 'performance: not a perf'
  run_in "${RELEASE_REPO}" next
  assert_output ''
  run_in "${RELEASE_REPO}" section v1.0.1
  assert_output '## v1.0.1'
  commit_empty "${RELEASE_REPO}" 'fix: real'
  run_in "${RELEASE_REPO}" next
  assert_output 'v1.0.1'
  run_in "${RELEASE_REPO}" section v1.0.1
  assert_output $'## v1.0.1\n\n### Fixes\n\n- real'

  release_repo v1.0.0
  commit_empty "${RELEASE_REPO}" 'feat(cli): a'
  commit_empty "${RELEASE_REPO}" 'perf(render): b'
  commit_empty "${RELEASE_REPO}" 'fix: c'
  run_in "${RELEASE_REPO}" section v1.1.0
  assert_output $'## v1.1.0\n\n### Features\n\n- **cli:** a\n\n### Fixes\n\n- **render:** b\n- c'
}

@test 'a revert is listed last and releases a patch; a breaking one is listed once as breaking' {
  release_repo v1.0.0
  commit_empty "${RELEASE_REPO}" 'revert: feat: add widget'
  run_in "${RELEASE_REPO}" next
  assert_output 'v1.0.1'
  run_in "${RELEASE_REPO}" section v1.0.1
  assert_output $'## v1.0.1\n\n### Reverts\n\n- feat: add widget'

  commit_empty "${RELEASE_REPO}" 'fix: a'
  commit_empty "${RELEASE_REPO}" 'revert(cli): fix: b'
  commit_empty "${RELEASE_REPO}" 'feat: c'
  commit_empty "${RELEASE_REPO}" 'reverting: not a revert'
  run_in "${RELEASE_REPO}" next
  assert_output 'v1.1.0'
  run_in "${RELEASE_REPO}" section v1.1.0
  assert_output $'## v1.1.0\n\n### Features\n\n- c\n\n### Fixes\n\n- a\n\n### Reverts\n\n- feat: add widget\n- **cli:** fix: b'

  release_repo v1.0.0
  commit_empty "${RELEASE_REPO}" 'revert!: feat: add widget' 'BREAKING CHANGE: the widget is gone again'
  run_in "${RELEASE_REPO}" next
  assert_output 'v2.0.0'
  run_in "${RELEASE_REPO}" section v2.0.0
  assert_output $'## v2.0.0\n\n### Breaking changes\n\n- feat: add widget\n\n  the widget is gone again'
}

@test 'angle brackets outside code spans are escaped, and the changelog passes markdownlint' {
  release_repo v1.0.0
  commit_empty "${RELEASE_REPO}" 'feat: accept <owner>/<repo> arguments'
  # shellcheck disable=SC2016 # Markdown code spans, meant literally
  commit_empty "${RELEASE_REPO}" 'fix: keep `<name>` and `a < b` in code spans, escape <tag>'
  commit_empty "${RELEASE_REPO}" 'fix: one stray ` leaves <nothing> raw'
  commit_empty "${RELEASE_REPO}" 'feat!: take <path>' $'BREAKING CHANGE: pass <dir> instead of <file>\nand quote `<dir>` if it has spaces'
  run_in "${RELEASE_REPO}" section v2.0.0
  assert_success
  assert_output "$(
    cat << 'EOF'
## v2.0.0

### Breaking changes

- take \<path>

  pass \<dir> instead of \<file>
  and quote `<dir>` if it has spaces

### Features

- accept \<owner>/\<repo> arguments

### Fixes

- keep `<name>` and `a < b` in code spans, escape \<tag>
- one stray ` leaves \<nothing> raw
EOF
  )"
  printf '%s\n' "${output}" > "${BATS_TEST_TMPDIR}/section.md"
  run_in "${BATS_TEST_TMPDIR}" prepend "${SKELETON}" "${BATS_TEST_TMPDIR}/section.md"
  printf '%s\n' "${output}" > "${BATS_TEST_TMPDIR}/CHANGELOG.md"
  run markdownlint-cli2 --config "${REPO_DIR}/.markdownlint-cli2.yaml" "${BATS_TEST_TMPDIR}/CHANGELOG.md"
  assert_success
}

@test 'a merge commit is dropped, so a change is listed once and its subject decides nothing' {
  release_repo v1.0.0
  git -C "${RELEASE_REPO}" switch --quiet --create topic
  commit_empty "${RELEASE_REPO}" 'fix: one thing'
  git -C "${RELEASE_REPO}" switch --quiet main
  git -C "${RELEASE_REPO}" merge --quiet --no-ff --message 'fix: one thing (#7)' topic
  run_in "${RELEASE_REPO}" section v1.0.1
  assert_output $'## v1.0.1\n\n### Fixes\n\n- one thing'
  run_in "${RELEASE_REPO}" next
  assert_output 'v1.0.1'

  release_repo v1.0.0
  git -C "${RELEASE_REPO}" switch --quiet --create topic
  commit_empty "${RELEASE_REPO}" 'feat!: drop gadget'
  git -C "${RELEASE_REPO}" switch --quiet main
  git -C "${RELEASE_REPO}" merge --quiet --no-ff --message 'feat!: drop gadget (#9)' topic
  run_in "${RELEASE_REPO}" section v2.0.0
  assert_output $'## v2.0.0\n\n### Breaking changes\n\n- drop gadget'

  release_repo v1.0.0
  git -C "${RELEASE_REPO}" switch --quiet --create topic
  commit_empty "${RELEASE_REPO}" 'docs: reword'
  git -C "${RELEASE_REPO}" switch --quiet main
  git -C "${RELEASE_REPO}" merge --quiet --no-ff --message 'feat!: reword (#8)' topic
  run_in "${RELEASE_REPO}" next
  assert_success
  assert_output ''
}

@test 'top prints the newest version a changelog names' {
  local -r file="${BATS_TEST_TMPDIR}/top.md"
  printf '# Changelog\n\nPreamble.\n\n## v1.2.0\n\n- a\n\n## v1.1.0\n\n- b\n' > "${file}"
  run_in "${BATS_TEST_TMPDIR}" top "${file}"
  assert_output 'v1.2.0'
  printf '# Changelog\n\n## v1.2.0 - 2026-01-01\n\n- a\n' > "${file}"
  run_in "${BATS_TEST_TMPDIR}" top "${file}"
  assert_output 'v1.2.0'
  run_in "${BATS_TEST_TMPDIR}" top "${SKELETON}"
  assert_success
  assert_output ''
  # A deeper heading is not a section heading.
  printf '# Changelog\n\n### Features\n\n## v1.2.0\n' > "${file}"
  run_in "${BATS_TEST_TMPDIR}" top "${file}"
  assert_output 'v1.2.0'
}

@test 'top fails on a first heading that is not a release version, or a missing file' {
  local -r file="${BATS_TEST_TMPDIR}/top.md"
  local text
  for text in '# Changelog\n\n## Unreleased\n\n- a\n\n## v1.2.0\n' '# Changelog\n\n## v1.2.0-rc.1\n\n## v1.1.0\n' \
    '# Changelog\n\n## v1.2\n'; do
    # shellcheck disable=SC2059 # the format is the whole fixture
    printf "${text}" > "${file}"
    run_in "${BATS_TEST_TMPDIR}" top "${file}"
    assert_failure 1
    assert_stderr --partial 'is not a version'
  done
  run_in "${BATS_TEST_TMPDIR}" top "${BATS_TEST_TMPDIR}/absent.md"
  assert_failure 1
  assert_stderr --partial 'cannot read'
}

@test 'notes prints one version body without its heading' {
  local -r file="${BATS_TEST_TMPDIR}/notes.md"
  printf '# Changelog\n\nPreamble.\n\n## v1.2.0\n\n### Features\n\n- a\n\n\n## v1.1.0 - 2026-01-01\n\n### Fixes\n\n- b\n- c\n\n' > "${file}"
  run_in "${BATS_TEST_TMPDIR}" notes "${file}" v1.2.0
  assert_output $'### Features\n\n- a'
  run_in "${BATS_TEST_TMPDIR}" notes "${file}" v1.1.0
  assert_output $'### Fixes\n\n- b\n- c'
  run_in "${BATS_TEST_TMPDIR}" notes "${file}" v9.9.9
  assert_failure 1
  assert_stderr --partial 'has no section for v9.9.9'
  run_in "${BATS_TEST_TMPDIR}" notes "${file}" v1.1
  assert_failure 2
  run_in "${BATS_TEST_TMPDIR}" notes "${BATS_TEST_TMPDIR}/absent.md" v1.2.0
  assert_failure 1
  printf '## v1.10.0\n\n- ten\n\n## v1.1.0\n\n- one\n' > "${file}"
  run_in "${BATS_TEST_TMPDIR}" notes "${file}" v1.1.0
  assert_success
  assert_output '- one'
}

@test 'prepend into the skeleton puts the section after the preamble' {
  local -r section="${BATS_TEST_TMPDIR}/section.md"
  printf '## v1.3.0\n\n### Features\n\n- new\n' > "${section}"
  run_in "${BATS_TEST_TMPDIR}" prepend "${SKELETON}" "${section}"
  assert_success
  assert_output "${SKELETON_TEXT}"$'\n\n## v1.3.0\n\n### Features\n\n- new'
  printf '%s\n' "${output}" > "${BATS_TEST_TMPDIR}/CHANGELOG.md"
  run_in "${BATS_TEST_TMPDIR}" top "${BATS_TEST_TMPDIR}/CHANGELOG.md"
  assert_output 'v1.3.0'
  run markdownlint-cli2 --config "${REPO_DIR}/.markdownlint-cli2.yaml" "${BATS_TEST_TMPDIR}/CHANGELOG.md"
  assert_success
}

@test 'prepend above older sections leaves them byte for byte' {
  local -r section="${BATS_TEST_TMPDIR}/section.md" file="${BATS_TEST_TMPDIR}/prepend.md"
  local -r old=$'## v1.2.0 - 2026-01-01\n\n### Fixes\n\n- old  \n\n\n## v1.1.0\n\n- older\n'
  printf '# Changelog\n\nPreamble.\n\n\n%s' "${old}" > "${file}"
  # Blank lines around the section's own text are not carried over.
  printf '\n\n## v1.3.0\n\n### Features\n\n- new\n\n\n' > "${section}"
  cd "${BATS_TEST_TMPDIR}" || return 1
  "${SCRIPT}" prepend "${file}" "${section}" > "${BATS_TEST_TMPDIR}/got.md"
  printf '# Changelog\n\nPreamble.\n\n## v1.3.0\n\n### Features\n\n- new\n\n%s' "${old}" > "${BATS_TEST_TMPDIR}/want.md"
  run cmp -- "${BATS_TEST_TMPDIR}/want.md" "${BATS_TEST_TMPDIR}/got.md"
  assert_success
  run_in "${BATS_TEST_TMPDIR}" notes "${BATS_TEST_TMPDIR}/got.md" v1.3.0
  assert_output $'### Features\n\n- new'
}

@test 'prepend into a file with no preamble leads with the section' {
  local -r section="${BATS_TEST_TMPDIR}/section.md" file="${BATS_TEST_TMPDIR}/prepend.md"
  printf '## v1.3.0\n\n### Features\n\n- new\n' > "${section}"
  printf '## v1.2.0\n\n- old\n' > "${file}"
  run_in "${BATS_TEST_TMPDIR}" prepend "${file}" "${section}"
  assert_output $'## v1.3.0\n\n### Features\n\n- new\n\n## v1.2.0\n\n- old'
}

@test 'prepend refuses an empty section and missing files' {
  local -r section="${BATS_TEST_TMPDIR}/section.md" file="${BATS_TEST_TMPDIR}/prepend.md"
  printf '## v1.2.0\n\n- old\n' > "${file}"
  printf '\n\n' > "${section}"
  run_in "${BATS_TEST_TMPDIR}" prepend "${file}" "${section}"
  assert_failure 1
  assert_stderr --partial 'holds no section'
  run_in "${BATS_TEST_TMPDIR}" prepend "${file}" "${BATS_TEST_TMPDIR}/absent.md"
  assert_failure 1
  run_in "${BATS_TEST_TMPDIR}" prepend "${BATS_TEST_TMPDIR}/absent.md" "${file}"
  assert_failure 1
}

@test 'two releases round-trip through prepend and notes' {
  release_repo v1.0.0
  commit_empty "${RELEASE_REPO}" 'feat: add widget'
  commit_empty "${RELEASE_REPO}" 'fix(render): pad columns'
  commit_empty "${RELEASE_REPO}" 'feat!: drop gadget' 'BREAKING CHANGE: remove gadget from repos.toml'
  local -r dir="${BATS_TEST_TMPDIR}/round-trip"
  mkdir "${dir}"
  cd "${RELEASE_REPO}" || return 1
  "${SCRIPT}" section v2.0.0 > "${dir}/section.md"
  "${SCRIPT}" prepend "${SKELETON}" "${dir}/section.md" > "${dir}/first.md"
  "${SCRIPT}" section v2.1.0 > "${dir}/section.md"
  "${SCRIPT}" prepend "${dir}/first.md" "${dir}/section.md" > "${dir}/CHANGELOG.md"
  run_in "${dir}" top "${dir}/CHANGELOG.md"
  assert_output 'v2.1.0'
  local want
  want="$(
    cat << 'EOF'
### Breaking changes

- drop gadget

  remove gadget from repos.toml

### Features

- add widget

### Fixes

- **render:** pad columns
EOF
  )"
  run_in "${dir}" notes "${dir}/CHANGELOG.md" v2.0.0
  assert_output "${want}"
  run_in "${dir}" notes "${dir}/CHANGELOG.md" v2.1.0
  assert_output "${want}"
  run markdownlint-cli2 --config "${REPO_DIR}/.markdownlint-cli2.yaml" "${dir}/CHANGELOG.md"
  assert_success
}

@test 'a call that cannot be meant is a usage error' {
  local -a call
  local calls=(
    ''
    'bogus'
    'validate'
    $'validate\nv1.0.0\nv1.0.1'
    $'last-tag\nv1.0.0'
    $'greater\nv1.0.0'
    $'greater\nv1.0.0\nv1.0.1\nv1.0.2'
    $'next\nv1.0.0'
    'section'
    'top'
    $'notes\nx.md'
    'notes'
    $'prepend\nx.md'
  )
  local line
  for line in "${calls[@]}"; do
    call=()
    if [[ -n "${line}" ]]; then
      mapfile -t call <<< "${line}"
    fi
    run_in "${BATS_TEST_TMPDIR}" "${call[@]}"
    assert_failure 2
    refute_stderr --partial 'ERROR: line'
  done
}
