#!/usr/bin/env bats

bats_require_minimum_version 1.5.0

# Every test runs in a fixture repo holding a two-lint manifest (one quick,
# one slow), a one-formatter manifest (fx, whose derived lint is
# check-fx-formatted) and example files that agree with them; each test breaks
# one thing.

readonly IMAGE_REPO='ghcr.io/rvenutolo/repo-toolbelt'
readonly USES_WORKFLOW='rvenutolo/repo-toolbelt/.github/workflows/lint.yml'
readonly PLACEHOLDER_DIGEST='0000000000000000000000000000000000000000000000000000000000000000'
readonly REAL_DIGEST='0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef'
readonly PLACEHOLDER_IMAGE="${IMAGE_REPO}:v0.0.0@sha256:${PLACEHOLDER_DIGEST}"
readonly PLACEHOLDER_USES="${USES_WORKFLOW}@0000000000000000000000000000000000000000 # v0.0.0"
readonly REAL_IMAGE="${IMAGE_REPO}:v1.2.3@sha256:${REAL_DIGEST}"
readonly REAL_USES="${USES_WORKFLOW}@0123456789abcdef0123456789abcdef01234567 # v1.2.3"
# The value of write_hook's formatters argument that writes no FORMATTERS block.
readonly NO_BLOCK='-'

function setup() {
  load test_helper/common
  make_repo
  cat > lints.toml << 'TOML'
[[lint]]
name = "lint-quick"
language = "bash"
speed = "quick"
description = "A quick lint"

[[lint]]
name = "lint-slow"
language = "go"
speed = "slow"
description = "A slow lint"
TOML
  cat > formatters.toml << 'TOML'
[[formatter]]
name = "fx"
language = "bash"
speed = "quick"
description = "Fx files are formatted"
include = ["*.fx"]
TOML
  mkdir --parents examples/consumer/.github/workflows examples/consumer/.githooks
  write_workflow "${PLACEHOLDER_USES}" "${PLACEHOLDER_IMAGE}" 'lint-quick' 'lint-slow' 'check-fx-formatted'
  write_hook pre-commit "${PLACEHOLDER_IMAGE}" '# fx' 'lint-quick' 'lint-slow # slow' 'check-fx-formatted'
  write_hook pre-push "${PLACEHOLDER_IMAGE}" "${NO_BLOCK}" 'lint-quick' 'lint-slow' 'check-fx-formatted'
  cat > examples/consumer/.repo-toolbelt.yml << 'YAML'
# Delete what you do not need.
# lint-quick:
#   exempt:
#     - path: a
#       reason: b
YAML
  git add .
  git -c user.name=t -c user.email=t@example.invalid commit --quiet --message 'test: fixture'
}

# @description Write the example CI workflow.
# @arg $1 uses the shared-workflow reference
# @arg $2 image the image reference
# @arg $@ lints the lines of the lints input, from $3 on
function write_workflow() {
  local -r uses="$1"
  local -r image="$2"
  shift 2
  {
    printf 'name: lint\non: [push]\npermissions: {}\njobs:\n  lint:\n'
    printf '    uses: %s\n' "${uses}"
    printf '    with:\n      image: %s\n      lints: |\n' "${image}"
    local lint
    for lint in "$@"; do
      printf '        %s\n' "${lint}"
    done
  } > examples/consumer/.github/workflows/lint.yml
}

# @description Write an example hook: a FORMATTERS=( block, unless
#              formatters is NO_BLOCK, then a LINTS=( array holding the
#              given lines.
# @arg $1 hook the hook name
# @arg $2 image the image reference
# @arg $3 formatters the block's lines, newline-separated ('' for an empty
#         block), or NO_BLOCK for none
# @arg $@ lines the LINTS array's lines, from $4 on
function write_hook() {
  local -r hook="$1"
  local -r image="$2"
  local -r formatters="$3"
  shift 3
  {
    printf '#!/usr/bin/env bash\n'
    printf '# The LINTS=( list below is what runs.\n'
    if [[ "${formatters}" != "${NO_BLOCK}" ]]; then
      printf 'readonly FORMATTERS=(\n'
      if [[ -n "${formatters}" ]]; then
        printf '%s\n' "${formatters}"
      fi
      printf ')\n'
    fi
    printf 'readonly LINTS=(\n'
    local line
    for line in "$@"; do
      printf '  %s\n' "${line}"
    done
    printf ')\n'
    # shellcheck disable=SC2016 # the hook's own array expansion, written literally
    printf 'exec docker run %s run --hook %s "${LINTS[@]}"\n' "${image}" "${hook}"
  } > "examples/consumer/.githooks/${hook}"
}

@test 'passes when every example lists every lint and formatter and no release tag exists' {
  run --separate-stderr "${REPO_DIR}/.ci/check-examples"
  assert_success
  assert_output ''
}

@test 'passes the real repo examples' {
  cd "${REPO_DIR}"
  run --separate-stderr "${REPO_DIR}/.ci/check-examples"
  assert_success
  assert_output ''
}

@test 'passes with blank lines and comments in the lists' {
  write_workflow "${PLACEHOLDER_USES}" "${PLACEHOLDER_IMAGE}" '# quick ones' 'lint-quick' '' 'lint-slow' \
    'check-fx-formatted'
  write_hook pre-push "${PLACEHOLDER_IMAGE}" "${NO_BLOCK}" '# all of them' 'lint-quick' '' 'lint-slow # slow' \
    'check-fx-formatted'
  run --separate-stderr "${REPO_DIR}/.ci/check-examples"
  assert_success
  assert_output ''
}

@test 'passes a FORMATTERS block with an uncommented entry and blank lines' {
  write_hook pre-commit "${PLACEHOLDER_IMAGE}" $'\n  fx  \n' 'lint-quick' 'lint-slow # slow' 'check-fx-formatted'
  run --separate-stderr "${REPO_DIR}/.ci/check-examples"
  assert_success
  assert_output ''
}

@test 'passes an empty FORMATTERS block when no formatter exists' {
  printf '# No formatters.\n' > formatters.toml
  write_workflow "${PLACEHOLDER_USES}" "${PLACEHOLDER_IMAGE}" 'lint-quick' 'lint-slow'
  write_hook pre-commit "${PLACEHOLDER_IMAGE}" '' 'lint-quick' 'lint-slow # slow'
  write_hook pre-push "${PLACEHOLDER_IMAGE}" "${NO_BLOCK}" 'lint-quick' 'lint-slow'
  run --separate-stderr "${REPO_DIR}/.ci/check-examples"
  assert_success
  assert_output ''
}

@test 'reports a lint the workflow example omits' {
  write_workflow "${PLACEHOLDER_USES}" "${PLACEHOLDER_IMAGE}" 'lint-quick' 'check-fx-formatted'
  run --separate-stderr "${REPO_DIR}/.ci/check-examples"
  assert_failure 1
  assert_line --index 0 'examples/consumer/.github/workflows/lint.yml: omits lint-slow'
  assert_line --index 1 '1 example problem'
}

@test 'reports a derived formatter lint the workflow example omits' {
  write_workflow "${PLACEHOLDER_USES}" "${PLACEHOLDER_IMAGE}" 'lint-quick' 'lint-slow'
  run --separate-stderr "${REPO_DIR}/.ci/check-examples"
  assert_failure 1
  assert_output 'examples/consumer/.github/workflows/lint.yml: omits check-fx-formatted
1 example problem'
}

@test 'reports an unknown lint the workflow example names' {
  write_workflow "${PLACEHOLDER_USES}" "${PLACEHOLDER_IMAGE}" 'lint-quick' 'lint-slow' 'check-fx-formatted' \
    'lint-gone'
  run --separate-stderr "${REPO_DIR}/.ci/check-examples"
  assert_failure 1
  assert_output "examples/consumer/.github/workflows/lint.yml: names unknown lint lint-gone
1 example problem"
}

@test 'reports a lint a hook example omits' {
  write_hook pre-push "${PLACEHOLDER_IMAGE}" "${NO_BLOCK}" 'lint-slow' 'check-fx-formatted'
  run --separate-stderr "${REPO_DIR}/.ci/check-examples"
  assert_failure 1
  assert_output 'examples/consumer/.githooks/pre-push: omits lint-quick
1 example problem'
}

@test 'reports a derived formatter lint the pre-commit LINTS list omits' {
  write_hook pre-commit "${PLACEHOLDER_IMAGE}" '# fx' 'lint-quick' 'lint-slow # slow'
  run --separate-stderr "${REPO_DIR}/.ci/check-examples"
  assert_failure 1
  assert_output 'examples/consumer/.githooks/pre-commit: omits check-fx-formatted
1 example problem'
}

@test 'reports a derived formatter lint the pre-push LINTS list omits' {
  write_hook pre-push "${PLACEHOLDER_IMAGE}" "${NO_BLOCK}" 'lint-quick' 'lint-slow'
  run --separate-stderr "${REPO_DIR}/.ci/check-examples"
  assert_failure 1
  assert_output 'examples/consumer/.githooks/pre-push: omits check-fx-formatted
1 example problem'
}

@test 'reports an unknown lint a hook example names, with its line' {
  write_hook pre-commit "${PLACEHOLDER_IMAGE}" '# fx' 'lint-quick' 'lint-slow # slow' 'check-fx-formatted' \
    'lint-gone'
  run --separate-stderr "${REPO_DIR}/.ci/check-examples"
  assert_failure 1
  assert_output 'examples/consumer/.githooks/pre-commit:10: names unknown lint lint-gone
1 example problem'
}

@test 'reports a slow lint not marked # slow in the pre-commit example' {
  write_hook pre-commit "${PLACEHOLDER_IMAGE}" '# fx' 'lint-quick' 'lint-slow' 'check-fx-formatted'
  run --separate-stderr "${REPO_DIR}/.ci/check-examples"
  assert_failure 1
  assert_output 'examples/consumer/.githooks/pre-commit:8: lint-slow is slow but not marked # slow
1 example problem'
}

@test 'reports a quick lint marked # slow in the pre-commit example' {
  write_hook pre-commit "${PLACEHOLDER_IMAGE}" '# fx' 'lint-quick # slow' 'lint-slow # slow' 'check-fx-formatted'
  run --separate-stderr "${REPO_DIR}/.ci/check-examples"
  assert_failure 1
  assert_output 'examples/consumer/.githooks/pre-commit:7: lint-quick is marked # slow but is quick
1 example problem'
}

@test 'reports a hook example with no lints list' {
  printf '#!/usr/bin/env bash\nexec docker run %s run --hook pre-push\n' "${PLACEHOLDER_IMAGE}" \
    > examples/consumer/.githooks/pre-push
  run --separate-stderr "${REPO_DIR}/.ci/check-examples"
  assert_failure 1
  assert_output 'examples/consumer/.githooks/pre-push: no LINTS=( list
1 example problem'
}

@test 'reports a pre-commit example with no FORMATTERS block' {
  write_hook pre-commit "${PLACEHOLDER_IMAGE}" "${NO_BLOCK}" 'lint-quick' 'lint-slow # slow' 'check-fx-formatted'
  run --separate-stderr "${REPO_DIR}/.ci/check-examples"
  assert_failure 1
  assert_output 'examples/consumer/.githooks/pre-commit: missing FORMATTERS
1 example problem'
}

@test 'reports a formatter the FORMATTERS block omits' {
  write_hook pre-commit "${PLACEHOLDER_IMAGE}" '' 'lint-quick' 'lint-slow # slow' 'check-fx-formatted'
  run --separate-stderr "${REPO_DIR}/.ci/check-examples"
  assert_failure 1
  assert_output 'examples/consumer/.githooks/pre-commit: omits fx
1 example problem'
}

@test 'reports an unknown formatter the FORMATTERS block names, with its line' {
  write_hook pre-commit "${PLACEHOLDER_IMAGE}" $'  # fx\n  gone' 'lint-quick' 'lint-slow # slow' \
    'check-fx-formatted'
  run --separate-stderr "${REPO_DIR}/.ci/check-examples"
  assert_failure 1
  assert_output 'examples/consumer/.githooks/pre-commit:5: names unknown formatter gone
1 example problem'
}

@test 'reports a line in the FORMATTERS block that is not a formatter entry' {
  write_hook pre-commit "${PLACEHOLDER_IMAGE}" $'  # Uncomment to rewrite\n  # fx' 'lint-quick' \
    'lint-slow # slow' 'check-fx-formatted'
  run --separate-stderr "${REPO_DIR}/.ci/check-examples"
  assert_failure 1
  assert_output 'examples/consumer/.githooks/pre-commit:4: not a formatter entry
1 example problem'
}

@test 'reports a FORMATTERS block in the pre-push example' {
  write_hook pre-push "${PLACEHOLDER_IMAGE}" '# fx' 'lint-quick' 'lint-slow' 'check-fx-formatted'
  run --separate-stderr "${REPO_DIR}/.ci/check-examples"
  assert_failure 1
  assert_output 'examples/consumer/.githooks/pre-push:3: pre-push takes no FORMATTERS block
1 example problem'
}

@test 'reports every missing example file' {
  rm examples/consumer/.github/workflows/lint.yml examples/consumer/.githooks/pre-commit \
    examples/consumer/.githooks/pre-push examples/consumer/.repo-toolbelt.yml
  run --separate-stderr "${REPO_DIR}/.ci/check-examples"
  assert_failure 1
  assert_output 'examples/consumer/.github/workflows/lint.yml: missing
examples/consumer/.githooks/pre-commit: missing
examples/consumer/.githooks/pre-push: missing
examples/consumer/.repo-toolbelt.yml: missing
4 example problems'
}

@test 'reports an unknown lint the config example names in a commented section' {
  printf '# lint-gone:\n#   exempt: []\n' >> examples/consumer/.repo-toolbelt.yml
  run --separate-stderr "${REPO_DIR}/.ci/check-examples"
  assert_failure 1
  assert_output 'examples/consumer/.repo-toolbelt.yml:6: names unknown lint lint-gone
1 example problem'
}

@test 'reports every placeholder pin once a v* release tag exists' {
  git tag v0.1.0
  run --separate-stderr "${REPO_DIR}/.ci/check-examples"
  assert_failure 1
  assert_output 'examples/consumer/.github/workflows/lint.yml:6: placeholder pin, but a v* release tag exists
examples/consumer/.github/workflows/lint.yml:8: placeholder pin, but a v* release tag exists
examples/consumer/.githooks/pre-commit:11: placeholder pin, but a v* release tag exists
examples/consumer/.githooks/pre-push:8: placeholder pin, but a v* release tag exists
4 example problems'
}

@test 'a tag that is not a v* release leaves the placeholder accepted' {
  git tag nightly
  run --separate-stderr "${REPO_DIR}/.ci/check-examples"
  assert_success
  assert_output ''
}

@test 'passes a real pin once a v* release tag exists' {
  git tag v1.2.3
  write_workflow "${REAL_USES}" "${REAL_IMAGE}" 'lint-quick' 'lint-slow' 'check-fx-formatted'
  write_hook pre-commit "${REAL_IMAGE}" '# fx' 'lint-quick' 'lint-slow # slow' 'check-fx-formatted'
  write_hook pre-push "${REAL_IMAGE}" "${NO_BLOCK}" 'lint-quick' 'lint-slow' 'check-fx-formatted'
  run --separate-stderr "${REPO_DIR}/.ci/check-examples"
  assert_success
  assert_output ''
}

@test 'reports every problem in one run' {
  write_workflow "${PLACEHOLDER_USES}" "${PLACEHOLDER_IMAGE}" 'lint-quick' 'check-fx-formatted'
  write_hook pre-commit "${PLACEHOLDER_IMAGE}" '' 'lint-quick' 'lint-slow' 'check-fx-formatted'
  run --separate-stderr "${REPO_DIR}/.ci/check-examples"
  assert_failure 1
  assert_line --index 3 '3 example problems'
}

@test 'exits 2 when the workflow example is not YAML' {
  printf 'jobs: [\n' > examples/consumer/.github/workflows/lint.yml
  run --separate-stderr "${REPO_DIR}/.ci/check-examples"
  assert_failure 2
  assert_stderr --partial 'check-examples: cannot read examples/consumer/.github/workflows/lint.yml'
}

@test 'exits 2 when the manifest cannot be read' {
  printf 'not = [toml\n' > lints.toml
  run --separate-stderr "${REPO_DIR}/.ci/check-examples"
  assert_failure 2
  assert_stderr --partial 'check-examples: cannot read lints.toml'
}

@test 'exits 2 when the formatter manifest cannot be read' {
  printf 'not = [toml\n' > formatters.toml
  run --separate-stderr "${REPO_DIR}/.ci/check-examples"
  assert_failure 2
  assert_stderr --partial 'check-examples: cannot read formatters.toml'
}

@test 'exits 2 with usage when given an argument' {
  run --separate-stderr "${REPO_DIR}/.ci/check-examples" extra
  assert_failure 2
  assert_stderr 'usage: check-examples'
}
