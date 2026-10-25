#!/usr/bin/env bats

# nix/release-version.nix, which the flake uses to version release-image, held
# to `.ci/release-version top` over the same changelogs: the version top names,
# without its `v`, or the fallback where top names none or refuses the heading.

bats_require_minimum_version 1.5.0

function setup() {
  load test_helper/common
  CHANGELOG="${BATS_TEST_TMPDIR}/CHANGELOG.md"
}

# @description Print the version nix/release-version.nix derives from a
#              changelog, with `dev` as the fallback.
# @arg $1 changelog the changelog's path
function nix_version() {
  local -r changelog="$1"
  nix eval --extra-experimental-features nix-command --impure --raw --expr "import \"${REPO_DIR}/nix/release-version.nix\" {
    changelog = builtins.readFile \"${changelog}\";
    script = builtins.readFile \"${REPO_DIR}/.ci/release-version\";
    fallback = \"dev\";
  }"
}

# @description Print what the flake should derive from a changelog, by asking
#              release-version: top's version without its `v`, or `dev`.
# @arg $1 changelog the changelog's path
function expected_version() {
  local -r changelog="$1"
  local top
  if top="$("${REPO_DIR}/.ci/release-version" top "${changelog}" 2> '/dev/null')" && [[ -n "${top}" ]]; then
    printf '%s\n' "${top#v}"
  else
    printf 'dev\n'
  fi
}

# @description Write a changelog, then require nix to agree with
#              release-version and with the stated version.
# @arg $1 text the changelog, as a printf format with no arguments
# @arg $2 want the version both must name
function assert_versions() {
  local -r text="$1" want="$2"
  # shellcheck disable=SC2059 # the format is the whole fixture
  printf "${text}" > "${CHANGELOG}"
  run expected_version "${CHANGELOG}"
  assert_output "${want}"
  run --separate-stderr nix_version "${CHANGELOG}"
  assert_success
  assert_output "${want}"
}

@test 'the top release heading is the version, without its v' {
  assert_versions '# Changelog\n\nPreamble.\n\n## v1.2.3\n\n- a\n\n## v1.2.2\n\n- b\n' '1.2.3'
}

@test 'text after the version, set off by a blank, is ignored' {
  assert_versions '# Changelog\n\n## v1.2.3 - 2026-01-01\n\n- a\n' '1.2.3'
}

@test 'a deeper heading above the first section is not a section' {
  assert_versions '# Changelog\n\n### Notes\n\n## v10.0.1\n' '10.0.1'
}

@test 'a preamble-only changelog falls back' {
  assert_versions '# Changelog\n\nEvery release, newest first.\n' 'dev'
}

@test 'a first heading that is not a release version falls back' {
  local text
  for text in '# Changelog\n\n## Unreleased\n\n## v1.2.3\n' '# Changelog\n\n## v1.2.3-rc.1\n' \
    '# Changelog\n\n## v01.2.3\n' '# Changelog\n\n## v1.2\n' '# Changelog\n\n## v1.2.3x\n'; do
    assert_versions "${text}" 'dev'
  done
}

@test 'the flake reads the heading pattern from release-version, so there is one copy' {
  run grep --count --fixed-strings 'HEADING_PATTERN' "${REPO_DIR}/nix/release-version.nix"
  assert_success
  run grep --count --fixed-strings '[[:blank:]]' "${REPO_DIR}/nix/release-version.nix" "${REPO_DIR}/flake.nix"
  assert_failure
}
