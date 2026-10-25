#!/usr/bin/env bash

# Shared setup for the e2e suites, which drive the built image with docker.
# `.ci/test-image` builds and loads the image and exports:
#
#   TOOLBELT_TEST_IMAGE    the loaded image's reference
#   TOOLBELT_TEST_VERSION  the version the image was built as
#
# run-tests skips this directory by its `e2e` file tag: these suites need a
# Docker daemon and an image, and kcov cannot see inside a container anyway.

load ../test_helper/common

FIXTURES_DIR="${BATS_TEST_DIRNAME}/fixtures"

# The user a hook file runs the container as.
HOST_USER="$(id -u):$(id -g)"

# Extra `docker run` options a test adds in front of the image; empty for the
# plain hook shape.
DOCKER_EXTRA=()

# @description Fail the suite before any test runs when the image is not named.
# @noargs
function require_image() {
  if [[ -z "${TOOLBELT_TEST_IMAGE:-}" ]]; then
    printf 'TOOLBELT_TEST_IMAGE is not set; run ./.ci/in-devshell ./.ci/test-image\n' >&2
    return 1
  fi
}

# @description Run the image with the hook files' `docker run` shape, for an
#              explicit checkout root, git common dir and user.
# @arg $1 root the checkout root, mounted read-only at its own path
# @arg $2 common the git common dir, mounted read-only at its own path
# @arg $3 user the `uid:gid` the container runs as
# @arg $@ args the runner's arguments
function shape_run() {
  local -r root="$1"
  local -r common="$2"
  local -r user="$3"
  shift 3
  docker run --rm --interactive --network=none \
    --user "${user}" \
    --volume "${root}:${root}:ro" --volume "${common}:${common}:ro" \
    --workdir "${root}" \
    "${DOCKER_EXTRA[@]}" \
    "${TOOLBELT_TEST_IMAGE}" "$@"
}

# @description Run the image exactly as a hook file does from the current
#              directory: root and common dir from git, the caller's IDs.
# @arg $@ args the runner's arguments
function hook_run() {
  local root common
  root="$(git rev-parse --show-toplevel)"
  common="$(git rev-parse --path-format=absolute --git-common-dir)"
  shape_run "${root}" "${common}" "${HOST_USER}" "$@"
}

# @description Run the image as README "Running `fmt`" does from the current
#              directory: the hook shape with the checkout root mounted
#              read-write, so `fmt` can rewrite files.
# @arg $@ args the runner's arguments
function rw_hook_run() {
  local root common
  root="$(git rev-parse --show-toplevel)"
  common="$(git rev-parse --path-format=absolute --git-common-dir)"
  docker run --rm --network=none --user "${HOST_USER}" \
    --volume "${root}:${root}" --volume "${common}:${common}:ro" \
    --workdir "${root}" \
    "${TOOLBELT_TEST_IMAGE}" "$@"
}

# @description Write text to a file in the current repo, exactly, creating its
#              directory.
# @arg $1 path the file's repo-relative path
# @arg $2 text the content
function write_file() {
  local -r path="$1"
  local -r text="$2"
  mkdir --parents -- "$(dirname -- "${path}")"
  printf '%s' "${text}" > "${path}"
}

# @description Assert a file in the current repo holds exactly the given text.
# @arg $1 path the file's repo-relative path
# @arg $2 text the expected content
function assert_content() {
  local -r path="$1"
  local -r text="$2"
  local actual
  # The x keeps any trailing newlines through the command substitution.
  actual="$(cat -- "${path}" && printf 'x')"
  assert_equal "${actual%x}" "${text}"
}

# @description Commit everything in the current repo.
# @arg $1 message the commit message
function commit_all() {
  git add --all
  git -c user.name=e2e -c user.email=e2e@example.invalid -c commit.gpgsign=false \
    commit --quiet --allow-empty --message "$1"
}

# @description Create a consumer fixture repo with one commit at the given
#              path and cd into it.
# @arg $1 dir where to create the repo; must not exist
function make_consumer() {
  local -r dir="$1"
  git init --quiet --initial-branch=main "${dir}"
  cp --recursive -- "${FIXTURES_DIR}/consumer/." "${dir}/"
  cd "${dir}" || return 1
  commit_all 'initial'
}

# @description Print every lint name in the manifest, one per line.
# @arg $1 language only lints in this language; empty for every lint
function manifest_lints() {
  local -r language="$1"
  if [[ -z "${language}" ]]; then
    yq --input-format toml --output-format tsv '.lint[].name' "${REPO_DIR}/lints.toml"
  else
    LANGUAGE="${language}" yq --input-format toml --output-format tsv \
      '.lint[] | select(.language == strenv(LANGUAGE)) | .name' "${REPO_DIR}/lints.toml"
  fi
}

# @description Run a command in the image as root with the given directory
#              mounted read-write, bypassing the runner; for fixture set-up
#              and clean-up the caller cannot do as itself.
# @arg $1 dir the directory to mount at its own path
# @arg $2 entrypoint the absolute path of the command in the image
# @arg $@ args the command's arguments
function root_run() {
  local -r dir="$1"
  local -r entrypoint="$2"
  shift 2
  docker run --rm --network=none --user 0:0 --volume "${dir}:${dir}" \
    --entrypoint "${entrypoint}" "${TOOLBELT_TEST_IMAGE}" "$@"
}
