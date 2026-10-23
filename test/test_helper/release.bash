#!/usr/bin/env bash

# Shared setup for the release scripts' suites, loaded after common:
#
#   load test_helper/common
#   load test_helper/release
#
# Every repository is a throwaway under the per-test temp dir. Signing is turned
# off in each, for tags as well as commits: the host may sign both by default
# and no key exists here.

# The changelog every release fixture starts from: a preamble and no release.
# Written here, never copied from this repo's CHANGELOG.md, which gains a
# section with every release.
readonly SKELETON_TEXT='# Changelog

Every release of this tool, newest first. Each section is generated from the
Conventional Commit subjects merged since the release before it.'

# @description Write the skeleton changelog to the per-test temp dir and export
#              its path as SKELETON.
# @noargs
function write_skeleton() {
  SKELETON="${BATS_TEST_TMPDIR}/skeleton.md"
  printf '%s\n' "${SKELETON_TEXT}" > "${SKELETON}"
  export SKELETON
}

# @description Create a throwaway repository with one commit, `chore: start`,
#              optionally tagged, and set RELEASE_REPO to its path. cliff.toml
#              is copied in untracked: the scripts read it from the working
#              tree.
# @arg $1 tag the tag the first commit carries, or empty for none
# @set RELEASE_REPO string the repository's absolute path
function release_repo() {
  local -r tag="${1:-}"
  RELEASE_REPO="$(mktemp --directory --tmpdir="${BATS_TEST_TMPDIR}" repo.XXXXXX)"
  git -C "${RELEASE_REPO}" init --quiet --initial-branch=main
  git -C "${RELEASE_REPO}" config user.name 'Test Author'
  git -C "${RELEASE_REPO}" config user.email 'test@example.invalid'
  git -C "${RELEASE_REPO}" config commit.gpgsign false
  git -C "${RELEASE_REPO}" config tag.gpgsign false
  cp -- "${REPO_DIR}/cliff.toml" "${RELEASE_REPO}/cliff.toml"
  commit_empty "${RELEASE_REPO}" 'chore: start'
  if [[ -n "${tag}" ]]; then
    git -C "${RELEASE_REPO}" tag "${tag}"
  fi
}

# @description Add one empty commit.
# @arg $1 repo
# @arg $2 subject
# @arg $3 footer paragraph, optional
function commit_empty() {
  local -r repo="$1" subject="$2" footer="${3:-}"
  local -a args=(--message "${subject}")
  if [[ -n "${footer}" ]]; then
    args+=(--message "${footer}")
  fi
  git -C "${repo}" commit --quiet --allow-empty "${args[@]}"
}

# @description Write a stand-in git to a directory that answers one call with a
#              set status and otherwise runs the real git. Put the directory
#              ahead of PATH to use it. The real git is resolved here, before
#              PATH changes.
# @arg $1 dir directory to put the stand-in in
# @arg $2 pattern shell glob the stand-in's whole argument list is matched against
# @arg $3 status exit status for a matching call
# @arg $4 output what a matching call prints to stdout, optional
function write_git_stub() {
  local -r dir="$1" pattern="$2" status="$3" output="${4:-}"
  local real_git
  real_git="$(command -v git)"
  mkdir --parents "${dir}"
  # shellcheck disable=SC2016 # the stand-in's own expansions must stay literal in the file it writes
  printf '#!/usr/bin/env bash\nif [[ "$*" == %s ]]; then\n  printf "%%s" %q\n  exit %q\nfi\nexec %q "$@"\n' \
    "${pattern}" "${output}" "${status}" "${real_git}" > "${dir}/git"
  chmod +x "${dir}/git"
}
