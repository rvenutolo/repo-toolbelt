#!/usr/bin/env bats

# The repo's commitlint config against messages shaped like a merge commit,
# whose body is the PR body. Every line of each message is literal; the real
# commitlint reads it on stdin.

function setup() {
  load test_helper/common
  LONG_TEXT="$(printf 'x%.0s' {1..170})"
}

# @description Lint a message with the repo's config.
# @arg $1 message the full commit message
function lint() {
  run commitlint --config "${REPO_DIR}/.commitlintrc.yml" <<< "$1"
}

@test 'accepts a footer line over 100 characters below Closes' {
  lint "$(printf 'fix: repair a thing\n\nCloses #1\nRuling: %s\n' "${LONG_TEXT}")"
  assert_success
}

@test 'accepts a footer line over 100 characters above Closes' {
  lint "$(printf 'fix: repair a thing\n\nRuling: %s\nCloses #1\n' "${LONG_TEXT}")"
  assert_success
}

@test 'accepts a body line over 100 characters' {
  lint "$(printf 'fix: repair a thing\n\n%s\n' "${LONG_TEXT}")"
  assert_success
}

@test 'rejects a header over 72 characters' {
  lint "fix: $(printf 'y%.0s' {1..68})"
  assert_failure
  assert_output --partial 'header-max-length'
}
