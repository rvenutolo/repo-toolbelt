#!/usr/bin/env bats

# release-publish inside throwaway repositories, with the real git and
# stand-ins for gh, skopeo and sleep. Each stand-in records every argument list
# it is handed; gh answers the reads from files the test writes, skopeo reads
# the image stream it is piped and writes the digest it is told to, and sleep
# returns at once. Nothing reaches GitHub or a registry.

bats_require_minimum_version 1.5.0

readonly VERSION='v1.1.0'
# The commit the release is cut from. Nothing resolves it.
readonly SHA='aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa0001'
readonly DIGEST='sha256:1111111111111111111111111111111111111111111111111111111111111111'
readonly IMAGE='ghcr.io/rvenutolo/repo-toolbelt'

# Every gh call the script may make, as the stub logs it. NOTES stands for the
# notes file's path, the script's own temporary file.
readonly READ_STATE="release view ${VERSION} --repo gh-owner/tool --json isDraft --jq .isDraft"
readonly WRITE_DELETE="release delete ${VERSION} --repo gh-owner/tool --yes"
readonly WRITE_CREATE="release create ${VERSION} --repo gh-owner/tool --draft --target ${SHA} --title ${VERSION} --notes-file NOTES"
readonly WRITE_EDIT="release edit ${VERSION} --repo gh-owner/tool --draft=false"
readonly READ_TAG="api repos/gh-owner/tool/git/ref/tags/${VERSION} --jq .object.sha"
readonly READ_HEAD='api repos/gh-owner/tool/commits/HEAD --jq .sha'

function setup() {
  load test_helper/common
  load test_helper/release
  SCRIPT="${REPO_DIR}/.ci/release-publish"
  REPOSITORY='gh-owner/tool'
}

# @description Create a repository the script can run in: a changelog whose
#              newest section is VERSION, one commit holding it, optionally a
#              tag on it, and `.stub/` beside it; see write_stubs.
# @arg $1 tag the tag the commit carries, or empty for none
# @set RELEASE_REPO string the repository's absolute path
# @set STUB string its stub directory
function publish_repo() {
  local -r tag="${1:-}"
  release_repo
  cat > "${RELEASE_REPO}/CHANGELOG.md" << EOF
# Changelog

Every release of this tool, newest first.

## ${VERSION}

### Features

- add widget

## v1.0.0

The first stable release.
EOF
  git -C "${RELEASE_REPO}" add -- CHANGELOG.md
  git -C "${RELEASE_REPO}" commit --quiet --amend --no-edit
  if [[ -n "${tag}" ]]; then
    git -C "${RELEASE_REPO}" tag "${tag}"
  fi
  STUB="${RELEASE_REPO}/.stub"
  write_stubs "${STUB}"
  printf '.stub/\n' >> "${RELEASE_REPO}/.git/info/exclude"
}

# @description Write the stand-ins. gh logs each call and answers from files:
#
#              - `release-state`: `draft` or `published`; absent for none,
#                anything else for a GitHub that cannot be asked
#              - `remote-tag`: what the tag points at before any publish;
#                absent for no tag, `unreachable` for a failed read
#              - `default-head`: the default branch's commit; absent, the
#                repository's own main; `unreachable` for a failed read
#              - `tag-sha`: where the tag points once published
#              - `tag-misses`: reads after the publish before the tag shows
#              - `tag-miss-status`: makes those misses failed reads, not 404s
#              - `draft-sticks`, `delete-fails`, `create-fails`, `edit-fails`
#
#              skopeo logs each call; `copy` keeps the stream it is piped in
#              `stream` and writes `copy-digest` (default DIGEST) to its
#              --digestfile; `inspect` prints `inspect-digest` (default
#              DIGEST). `copy-fails` and `inspect-fails` make them fail. The
#              image is a stand-in stream script that prints an inert marker.
# @arg $1 dir directory to put the stand-ins and their files in
function write_stubs() {
  local -r dir="$1"
  local q_dir
  q_dir="$(printf '%q' "${dir}")"
  mkdir --parents "${dir}/bin"
  : > "${dir}/calls.log"
  : > "${dir}/skopeo.log"
  : > "${dir}/sleep.log"
  cat > "${dir}/bin/gh" << STUB
#!/usr/bin/env bash
printf '%s\n' "\$*" >> ${q_dir}/calls.log
case "\$*" in
  '${READ_STATE}')
    if [[ ! -e ${q_dir}/release-state || (-e ${q_dir}/deleted && ! -e ${q_dir}/draft-sticks) ]]; then
      echo 'release not found' >&2
      exit 1
    fi
    case "\$(< ${q_dir}/release-state)" in
      draft) echo true ;;
      published) echo false ;;
      *)
        echo 'HTTP 502: Bad Gateway' >&2
        exit 1
        ;;
    esac
    ;;
  '${READ_HEAD}')
    if [[ ! -e ${q_dir}/default-head ]]; then
      git -C ${q_dir}/.. rev-parse --verify --quiet refs/heads/main
    elif [[ "\$(< ${q_dir}/default-head)" == 'unreachable' ]]; then
      echo 'gh: Bad Gateway (HTTP 502)' >&2
      exit 1
    else
      cat ${q_dir}/default-head
    fi
    ;;
  '${WRITE_DELETE}')
    if [[ -e ${q_dir}/delete-fails ]]; then
      echo 'gh: Bad Gateway (HTTP 502)' >&2
      exit 1
    fi
    : > ${q_dir}/deleted
    ;;
  'release create ${VERSION} --repo gh-owner/tool --draft '*)
    if [[ -e ${q_dir}/create-fails ]]; then
      echo 'gh: Bad Gateway (HTTP 502)' >&2
      exit 1
    fi
    while ((\$# > 1)); do
      if [[ "\$1" == '--notes-file' ]]; then
        cat -- "\$2" > ${q_dir}/notes.md
      fi
      shift
    done
    ;;
  '${WRITE_EDIT}')
    if [[ -e ${q_dir}/edit-fails ]]; then
      echo 'gh: Bad Gateway (HTTP 502)' >&2
      exit 1
    fi
    : > ${q_dir}/published
    ;;
  '${READ_TAG}')
    if [[ -e ${q_dir}/remote-tag ]]; then
      if [[ "\$(< ${q_dir}/remote-tag)" == 'unreachable' ]]; then
        echo 'gh: Bad Gateway (HTTP 502)' >&2
        exit 1
      fi
      cat ${q_dir}/remote-tag
      exit 0
    fi
    misses=0
    if [[ -e ${q_dir}/published ]]; then
      echo read >> ${q_dir}/tag-reads
      if [[ -e ${q_dir}/tag-misses ]]; then
        misses="\$(< ${q_dir}/tag-misses)"
      fi
    fi
    if [[ -e ${q_dir}/published && -e ${q_dir}/tag-sha ]] \\
      && ((\$(wc --lines < ${q_dir}/tag-reads) > misses)); then
      cat ${q_dir}/tag-sha
    elif [[ -e ${q_dir}/published && -e ${q_dir}/tag-miss-status ]]; then
      echo 'gh: Bad Gateway (HTTP 502)' >&2
      exit 1
    else
      echo '{"message":"Not Found","status":"404"}'
      echo 'gh: Not Found (HTTP 404)' >&2
      exit 1
    fi
    ;;
  *)
    printf 'stub gh: unexpected call: %s\n' "\$*" >&2
    exit 1
    ;;
esac
STUB
  cat > "${dir}/bin/skopeo" << STUB
#!/usr/bin/env bash
printf '%s\n' "\$*" >> ${q_dir}/skopeo.log
case "\$1" in
  copy)
    cat > ${q_dir}/stream
    if [[ -e ${q_dir}/copy-fails ]]; then
      echo 'stub skopeo: unauthorized' >&2
      exit 1
    fi
    while ((\$# > 1)); do
      if [[ "\$1" == '--digestfile' ]]; then
        if [[ -e ${q_dir}/copy-digest ]]; then
          cat ${q_dir}/copy-digest > "\$2"
        else
          printf '%s' '${DIGEST}' > "\$2"
        fi
      fi
      shift
    done
    ;;
  inspect)
    if [[ -e ${q_dir}/inspect-fails ]]; then
      echo 'stub skopeo: manifest unknown' >&2
      exit 1
    fi
    if [[ -e ${q_dir}/inspect-digest ]]; then
      cat ${q_dir}/inspect-digest
    else
      echo '${DIGEST}'
    fi
    ;;
  *)
    printf 'stub skopeo: unexpected call: %s\n' "\$*" >&2
    exit 1
    ;;
esac
STUB
  cat > "${dir}/bin/sleep" << STUB
#!/usr/bin/env bash
printf '%s\n' "\$*" >> ${q_dir}/sleep.log
STUB
  cat > "${dir}/image-stream" << 'STUB'
#!/usr/bin/env bash
printf 'INERT_IMAGE_STREAM\n'
STUB
  chmod +x "${dir}/bin/gh" "${dir}/bin/skopeo" "${dir}/bin/sleep" "${dir}/image-stream"
}

# @description Run the script in RELEASE_REPO with the stand-ins first on PATH.
#              Leaves bats' `status`, `output` and `stderr`, and CALLS (gh,
#              with the notes path as NOTES), SKOPEO and SLEPT.
# @arg $@ arguments passed to the script
function run_script() {
  : > "${STUB}/calls.log"
  : > "${STUB}/skopeo.log"
  : > "${STUB}/sleep.log"
  : > "${STUB}/notes.md"
  cd "${RELEASE_REPO}" || return 1
  # The stand-ins shadow the real tools; each test runs in its own process.
  PATH="${STUB}/bin:${PATH}" GITHUB_REPOSITORY="${REPOSITORY}" run --separate-stderr "${SCRIPT}" "$@"
  CALLS="$(< "${STUB}/calls.log")"
  SKOPEO="$(< "${STUB}/skopeo.log")"
  SLEPT="$(< "${STUB}/sleep.log")"
  if [[ "${CALLS}" =~ --notes-file\ ([^[:space:]]+) ]]; then
    CALLS="${CALLS//"${BASH_REMATCH[1]}"/NOTES}"
  fi
}

# @description Set the repository the script is told to publish to.
# @arg $1 repository owner/name, or empty to run the script without one
function set_repository() {
  REPOSITORY="$1"
}

# @description Run the next calls in another repository and its stand-ins.
# @arg $1 repo the repository's absolute path; its stand-ins are written
function use_repo() {
  local -r repo="$1"
  RELEASE_REPO="${repo}"
  STUB="${repo}/.stub"
  write_stubs "${STUB}"
}

# @description Fail unless the run made exactly these gh calls, in order.
# @arg $@ the expected calls; none when gh must not be called at all
function assert_only_calls() {
  local want='' call
  for call in "$@"; do
    want+="${want:+$'\n'}${call}"
  done
  if [[ "${CALLS}" != "${want}" ]]; then
    fail "gh calls were: ${CALLS:-(none)}; want exactly: ${want:-(none)}"
  fi
}

# @description Add one commit that changes a file other than the changelog.
# @arg $1 file the file to write, relative to the repository
# @arg $2 subject
function commit_file() {
  local -r file="$1" subject="$2"
  printf '%s\n' "${subject}" > "${RELEASE_REPO}/${file}"
  git -C "${RELEASE_REPO}" add -- "${file}"
  git -C "${RELEASE_REPO}" commit --quiet --message "${subject}"
}

# @description Commit a new changelog to main and leave the checkout detached
#              at a commit: an old checkout and a default branch that moved on.
# @arg $1 commit the commit to leave checked out
# @arg $2 subject
# @arg $3 changelog the new changelog, as a printf format with no arguments
function commit_default_changelog() {
  local -r commit="$1" subject="$2" changelog="$3"
  git -C "${RELEASE_REPO}" switch --quiet main
  # shellcheck disable=SC2059 # the format is the caller's whole changelog
  printf "${changelog}" > "${RELEASE_REPO}/CHANGELOG.md"
  git -C "${RELEASE_REPO}" commit --quiet --message "${subject}" -- CHANGELOG.md
  git -C "${RELEASE_REPO}" checkout --quiet --detach "${commit}"
}

@test 'pending: an untagged version with no release, or a draft, is to be published from HEAD' {
  publish_repo
  local head
  head="$(git -C "${RELEASE_REPO}" rev-parse HEAD)"
  run_script pending
  assert_success
  assert_output $'publish=true\nversion='"${VERSION}"$'\nsha='"${head}"
  assert_only_calls "${READ_TAG}" "${READ_STATE}" "${READ_HEAD}"
  printf 'draft\n' > "${STUB}/release-state"
  run_script pending
  assert_success
  assert_output $'publish=true\nversion='"${VERSION}"$'\nsha='"${head}"
}

@test 'pending: a published release is nothing to publish, even without a local tag' {
  publish_repo
  printf 'published\n' > "${STUB}/release-state"
  run_script pending
  assert_success
  assert_output 'publish=false'
  assert_stderr --partial "a release ${VERSION} is published on GitHub"
  assert_only_calls "${READ_TAG}" "${READ_STATE}"
}

@test 'pending: GitHub that cannot be asked about the release is a failure' {
  publish_repo
  printf 'unreachable\n' > "${STUB}/release-state"
  run_script pending
  assert_failure 1
  assert_output ''
  assert_stderr --partial "cannot tell whether a release ${VERSION} exists"
  assert_only_calls "${READ_TAG}" "${READ_STATE}"
}

@test 'pending: a release state that is neither draft nor published is a failure' {
  publish_repo
  mkdir --parents "${BATS_TEST_TMPDIR}/odd"
  cat > "${BATS_TEST_TMPDIR}/odd/gh" << 'STUB'
#!/usr/bin/env bash
case "$*" in
  *'git/ref/tags/'*)
    echo 'gh: Not Found (HTTP 404)' >&2
    exit 1
    ;;
  *) echo maybe ;;
esac
STUB
  chmod +x "${BATS_TEST_TMPDIR}/odd/gh"
  cd "${RELEASE_REPO}" || return 1
  PATH="${BATS_TEST_TMPDIR}/odd:${PATH}" GITHUB_REPOSITORY="${REPOSITORY}" run --separate-stderr "${SCRIPT}" pending
  assert_failure 1
  assert_stderr --partial "cannot tell whether the release ${VERSION} is a draft: maybe"
}

@test 'pending: a tag on GitHub alone settles it, and a failed tag read is a failure' {
  publish_repo
  printf 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb0002\n' > "${STUB}/remote-tag"
  run_script pending
  assert_success
  assert_output 'publish=false'
  assert_stderr --partial "the tag ${VERSION} exists on GitHub"
  assert_only_calls "${READ_TAG}"
  printf 'unreachable\n' > "${STUB}/remote-tag"
  run_script pending
  assert_failure 1
  assert_output ''
  assert_stderr --partial "cannot tell whether the tag ${VERSION} exists"
  assert_only_calls "${READ_TAG}"
}

@test 'pending: no repository to ask about is a failure' {
  publish_repo
  set_repository ''
  run_script pending
  assert_failure 1
  assert_stderr --partial 'GITHUB_REPOSITORY is not set'
  assert_only_calls
}

@test 'pending: a local tag, a changelog with no release, or a first heading that is no version publish nothing' {
  publish_repo "${VERSION}"
  run_script pending
  assert_success
  assert_output 'publish=false'
  assert_stderr --partial "${VERSION} is tagged"
  assert_only_calls

  publish_repo
  printf '# Changelog\n\nEvery release of this tool, newest first.\n' > "${RELEASE_REPO}/CHANGELOG.md"
  run_script pending
  assert_success
  assert_output 'publish=false'
  assert_stderr --partial 'the changelog holds no release'
  assert_only_calls

  printf '# Changelog\n\n## Unreleased\n\n- add widget\n' > "${RELEASE_REPO}/CHANGELOG.md"
  run_script pending
  assert_success
  assert_output 'publish=false'
  assert_stderr --partial 'Unreleased'
  assert_stderr --partial 'WARN'
  assert_only_calls
}

@test 'pending: an argument is a usage error' {
  publish_repo
  run_script pending --bogus
  assert_failure 2
  assert_stderr --partial 'pending takes no argument'
  assert_only_calls
}

@test 'pending: outside a repository, or with no changelog, is a failure' {
  publish_repo
  rm -- "${RELEASE_REPO}/CHANGELOG.md"
  run_script pending
  assert_failure 1
  assert_stderr --partial 'no CHANGELOG.md at the top of'
  mkdir "${BATS_TEST_TMPDIR}/not-a-repo"
  cd "${BATS_TEST_TMPDIR}/not-a-repo" || return 1
  PATH="${STUB}/bin:${PATH}" GITHUB_REPOSITORY="${REPOSITORY}" run --separate-stderr "${SCRIPT}" pending
  assert_failure 1
  assert_stderr --partial 'not inside a git repository'
}

@test 'pending: only the merge that brought the version in publishes it' {
  publish_repo
  printf '# Changelog\n\n## v1.0.0\n\nThe first stable release.\n' > "${RELEASE_REPO}/CHANGELOG.md"
  git -C "${RELEASE_REPO}" commit --quiet --message 'chore: start over' -- CHANGELOG.md
  git -C "${RELEASE_REPO}" switch --quiet --create chore/release
  printf '# Changelog\n\n## %s\n\n- add widget\n\n## v1.0.0\n\nThe first stable release.\n' "${VERSION}" \
    > "${RELEASE_REPO}/CHANGELOG.md"
  git -C "${RELEASE_REPO}" commit --quiet --message "chore(release): ${VERSION}" -- CHANGELOG.md
  local branch_commit merge later
  branch_commit="$(git -C "${RELEASE_REPO}" rev-parse HEAD)"
  git -C "${RELEASE_REPO}" switch --quiet main
  git -C "${RELEASE_REPO}" merge --quiet --no-ff --message 'chore(release): merge' chore/release
  merge="$(git -C "${RELEASE_REPO}" rev-parse HEAD)"
  run_script pending
  assert_success
  assert_output $'publish=true\nversion='"${VERSION}"$'\nsha='"${merge}"
  refute_output --partial "${branch_commit}"

  # A later push finds the release unpublished: a failure naming the run to
  # run again, with nothing for the workflow to act on.
  commit_file notes.txt 'feat: add gadget'
  run_script pending
  assert_failure 1
  assert_output ''
  assert_stderr --partial "the release ${VERSION} is not published"
  assert_stderr --partial "${merge}"
  assert_stderr --partial 're-run the workflow run of that commit'

  # A later edit that keeps the version on top does not move the release.
  printf '# Changelog\n\nNewest first.\n\n## %s\n\n- add widget\n\n## v1.0.0\n\nThe first stable release.\n' \
    "${VERSION}" > "${RELEASE_REPO}/CHANGELOG.md"
  git -C "${RELEASE_REPO}" commit --quiet --message 'docs: say the order' -- CHANGELOG.md
  later="$(git -C "${RELEASE_REPO}" rev-parse HEAD)"
  run_script pending
  assert_failure 1
  assert_stderr --partial "the release commit is ${merge}"
  refute_stderr --partial "the release commit is ${later}"

  # The merge's own run, started again after those pushes.
  git -C "${RELEASE_REPO}" checkout --quiet --detach "${merge}"
  run_script pending
  assert_success
  assert_output $'publish=true\nversion='"${VERSION}"$'\nsha='"${merge}"
}

@test 'pending: a version the default branch has given up is published from no commit' {
  publish_repo
  local merge
  merge="$(git -C "${RELEASE_REPO}" rev-parse HEAD)"
  local changelog
  for changelog in '# Changelog\n\n## v1.1.1\n\n- add widget\n' '# Changelog\n\nNo release.\n' \
    '# Changelog\n\n## Unreleased\n\n- add widget\n' '# Changelog\n\n## v1.0.0\n\nThe first stable release.\n'; do
    commit_default_changelog "${merge}" 'chore: change the changelog' "${changelog}"
    run_script pending
    assert_failure 1
    assert_output ''
    assert_stderr --partial "the default branch no longer proposes ${VERSION}"
    assert_only_calls "${READ_TAG}" "${READ_STATE}" "${READ_HEAD}"
  done

  # Proposed again: the new merge is the release commit, never the old one.
  commit_default_changelog "${merge}" "chore(release): ${VERSION} again" \
    "# Changelog\n\n## ${VERSION}\n\n- add widget\n"
  local again
  again="$(git -C "${RELEASE_REPO}" rev-parse refs/heads/main)"
  run_script pending
  assert_failure 1
  assert_stderr --partial "the release commit is ${again}"
  git -C "${RELEASE_REPO}" switch --quiet main
  run_script pending
  assert_success
  assert_output $'publish=true\nversion='"${VERSION}"$'\nsha='"${again}"

  # Once tagged, a later run has nothing to say.
  git -C "${RELEASE_REPO}" tag "${VERSION}" "${again}"
  run_script pending
  assert_success
  assert_output 'publish=false'
  assert_only_calls
}

@test 'pending: a default branch head that cannot be read or walked is a failure' {
  publish_repo
  printf 'unreachable\n' > "${STUB}/default-head"
  run_script pending
  assert_failure 1
  assert_stderr --partial 'cannot read the head of the default branch'
  printf 'not-a-sha\n' > "${STUB}/default-head"
  run_script pending
  assert_failure 1
  assert_stderr --partial 'GitHub answered not-a-sha'
  printf '0123456789abcdef0123456789abcdef01234567\n' > "${STUB}/default-head"
  run_script pending
  assert_failure 1
  assert_stderr --partial '0123456789abcdef0123456789abcdef01234567 is not in this checkout'
}

@test 'pending: a changelog edited and not committed, or held by no commit, is a failure' {
  publish_repo
  printf '\nA line nobody committed.\n' >> "${RELEASE_REPO}/CHANGELOG.md"
  run_script pending
  assert_failure 1
  assert_stderr --partial 'CHANGELOG.md is not as it was committed'

  publish_repo
  git -C "${RELEASE_REPO}" switch --quiet --orphan untracked
  git -C "${RELEASE_REPO}" show main:CHANGELOG.md > "${RELEASE_REPO}/CHANGELOG.md"
  git -C "${RELEASE_REPO}" commit --quiet --allow-empty --message 'chore: start'
  run_script pending
  assert_failure 1
  assert_stderr --partial 'no commit'
}

@test 'pending: a shallow clone is refused' {
  publish_repo
  commit_file notes.txt 'feat: add gadget'
  local -r shallow="${BATS_TEST_TMPDIR}/shallow"
  git clone --quiet --depth 1 "file://${RELEASE_REPO}" "${shallow}"
  use_repo "${shallow}"
  run_script pending
  assert_failure 1
  assert_stderr --partial 'shallow clone'
  assert_only_calls
}

@test 'pending: a shallow-clone check that cannot be answered is a failure' {
  publish_repo
  write_git_stub "${STUB}/bin" '*--is-shallow-repository*' 2
  run_script pending
  assert_failure 1
  assert_stderr --partial 'cannot tell whether the repository is a shallow clone'
  assert_only_calls
}

@test 'pending: a changelog history that cannot be read is a failure' {
  publish_repo
  write_git_stub "${STUB}/bin" '*log*--first-parent*' 2
  run_script pending
  assert_failure 1
  assert_stderr --partial 'cannot read the history of CHANGELOG.md'
  assert_only_calls "${READ_TAG}" "${READ_STATE}" "${READ_HEAD}"
}

@test 'pending: a changelog history with no commit is a failure, not a release from nowhere' {
  publish_repo
  local head
  head="$(git -C "${RELEASE_REPO}" rev-parse HEAD)"
  write_git_stub "${STUB}/bin" '*log*--first-parent*' 0
  run_script pending
  assert_failure 1
  assert_stderr --partial \
    "no commit in the history of ${head} holds a CHANGELOG.md that names ${VERSION}"
  assert_only_calls "${READ_TAG}" "${READ_STATE}" "${READ_HEAD}"
}

@test 'push-image streams the image to GHCR under the version and prints the digest the registry holds' {
  publish_repo
  run_script push-image --version "${VERSION}" --image "${STUB}/image-stream"
  assert_success
  assert_output "digest=${DIGEST}"
  assert_equal "$(< "${STUB}/stream")" 'INERT_IMAGE_STREAM'
  local -a lines
  mapfile -t lines <<< "${SKOPEO}"
  assert_equal "${#lines[@]}" 2
  [[ "${lines[0]}" == 'copy --retry-times 3 --digestfile '*" docker-archive:/dev/stdin docker://${IMAGE}:${VERSION}" ]]
  assert_equal "${lines[1]}" "inspect --retry-times 3 --format {{.Digest}} docker://${IMAGE}:${VERSION}"
  # Nothing is asked of GitHub.
  assert_only_calls
}

@test 'push-image fails when the push fails, and reads nothing back' {
  publish_repo
  : > "${STUB}/copy-fails"
  run_script push-image --version "${VERSION}" --image "${STUB}/image-stream"
  assert_failure 1
  assert_output ''
  assert_stderr --partial "could not push the image to ${IMAGE}:${VERSION}"
  [[ "${SKOPEO}" != *inspect* ]]
}

@test 'push-image fails when the image stream fails' {
  publish_repo
  printf '#!/usr/bin/env bash\nexit 3\n' > "${STUB}/image-stream"
  run_script push-image --version "${VERSION}" --image "${STUB}/image-stream"
  assert_failure 1
  assert_output ''
  assert_stderr --partial "could not push the image to ${IMAGE}:${VERSION}"
}

@test 'push-image fails when the registry cannot be read back' {
  publish_repo
  : > "${STUB}/inspect-fails"
  run_script push-image --version "${VERSION}" --image "${STUB}/image-stream"
  assert_failure 1
  assert_output ''
  assert_stderr --partial "could not read ${IMAGE}:${VERSION} back from the registry"
}

@test 'push-image fails when the registry holds another digest than was pushed' {
  publish_repo
  printf 'sha256:%064d\n' 2 > "${STUB}/inspect-digest"
  run_script push-image --version "${VERSION}" --image "${STUB}/image-stream"
  assert_failure 1
  assert_output ''
  assert_stderr --partial "the registry holds ${IMAGE}:${VERSION} at sha256:$(printf '%064d' 2), not at ${DIGEST}"
}

@test 'push-image fails when a digest is not a sha256 digest' {
  publish_repo
  printf 'sha256:abc' > "${STUB}/copy-digest"
  run_script push-image --version "${VERSION}" --image "${STUB}/image-stream"
  assert_failure 1
  assert_stderr --partial 'skopeo reported the pushed digest as sha256:abc'
  rm -- "${STUB}/copy-digest"
  printf 'nonsense\n' > "${STUB}/inspect-digest"
  run_script push-image --version "${VERSION}" --image "${STUB}/image-stream"
  assert_failure 1
  assert_stderr --partial 'the registry holds'
}

@test 'push-image refuses an image that is not an executable stream script' {
  publish_repo
  run_script push-image --version "${VERSION}" --image "${STUB}/absent"
  assert_failure 1
  assert_stderr --partial 'is not an executable image stream'
  assert_equal "${SKOPEO}" ''
}

@test 'push-image: a malformed call is a usage error and pushes nothing' {
  publish_repo
  local -a call
  local line
  local calls=(
    $'push-image\n--image\nx'
    $'push-image\n--version\nv1.1.0'
    $'push-image\n--version\n1.1.0\n--image\nx'
    $'push-image\n--version'
    $'push-image\n--version\nv1.1.0\n--image\nx\n--bogus'
  )
  for line in "${calls[@]}"; do
    mapfile -t call <<< "${line}"
    run_script "${call[@]}"
    assert_failure 2
    assert_equal "${SKOPEO}" ''
  done
}

@test 'release: a draft with the notes, then the publish that creates the tag, then the tag read' {
  publish_repo
  printf '%s\n' "${SHA}" > "${STUB}/tag-sha"
  run_script release --version "${VERSION}" --digest "${DIGEST}" --sha "${SHA}"
  assert_success
  assert_only_calls "${READ_TAG}" "${READ_STATE}" "${WRITE_CREATE}" "${WRITE_EDIT}" "${READ_TAG}"
  assert_equal "${SLEPT}" ''
  assert_equal "${SKOPEO}" ''
  local notes
  notes="$(< "${STUB}/notes.md")"
  # The notes start with the full reference and the commit, ready to paste.
  local -r first=$'`'"${IMAGE}:${VERSION}@${DIGEST}"$'`\n\nCommit: `'"${SHA}"$'`\n\n### Features\n\n- add widget\n\n'
  assert_equal "${notes:0:${#first}}" "${first}"
  [[ "${notes}" == *$'uses: gh-owner/tool/.github/workflows/lint.yml@'"${SHA} # ${VERSION}"* ]]
  [[ "${notes}" == *"image: ${IMAGE}:${VERSION}@${DIGEST}"* ]]
  [[ "${notes}" == *"gh attestation verify oci://${IMAGE}@${DIGEST} --repo gh-owner/tool"* ]]
  # The section's own heading is the release's title; the next section is
  # another release's.
  [[ "${notes}" != *"## ${VERSION}"* && "${notes}" != *'The first stable release.'* ]]
}

@test 'release: a draft of that name is deleted first, and the state read again' {
  publish_repo
  printf 'draft\n' > "${STUB}/release-state"
  printf '%s\n' "${SHA}" > "${STUB}/tag-sha"
  run_script release --version "${VERSION}" --digest "${DIGEST}" --sha "${SHA}"
  assert_success
  assert_only_calls "${READ_TAG}" "${READ_STATE}" "${WRITE_DELETE}" "${READ_STATE}" \
    "${WRITE_CREATE}" "${WRITE_EDIT}" "${READ_TAG}"
}

@test 'release: a published release of that name is never touched' {
  publish_repo
  printf 'published\n' > "${STUB}/release-state"
  run_script release --version "${VERSION}" --digest "${DIGEST}" --sha "${SHA}"
  assert_failure 1
  assert_stderr --partial "the release ${VERSION} is published; a published release is never replaced"
  assert_only_calls "${READ_TAG}" "${READ_STATE}"
}

@test 'release: GitHub that cannot be asked about the release writes nothing' {
  publish_repo
  printf 'unreachable\n' > "${STUB}/release-state"
  run_script release --version "${VERSION}" --digest "${DIGEST}" --sha "${SHA}"
  assert_failure 1
  assert_stderr --partial "cannot tell whether a release ${VERSION} exists"
  assert_only_calls "${READ_TAG}" "${READ_STATE}"
}

@test 'release: a draft still there after every delete ends the run' {
  publish_repo
  printf 'draft\n' > "${STUB}/release-state"
  : > "${STUB}/draft-sticks"
  run_script release --version "${VERSION}" --digest "${DIGEST}" --sha "${SHA}"
  assert_failure 1
  assert_stderr --partial "a draft release ${VERSION} is still there after 5 deletes"
  assert_only_calls "${READ_TAG}" \
    "${READ_STATE}" "${WRITE_DELETE}" "${READ_STATE}" "${WRITE_DELETE}" "${READ_STATE}" "${WRITE_DELETE}" \
    "${READ_STATE}" "${WRITE_DELETE}" "${READ_STATE}" "${WRITE_DELETE}" "${READ_STATE}"
}

@test 'release: a draft that cannot be deleted, created or published ends the run with no tag' {
  publish_repo
  printf 'draft\n' > "${STUB}/release-state"
  : > "${STUB}/delete-fails"
  run_script release --version "${VERSION}" --digest "${DIGEST}" --sha "${SHA}"
  assert_failure 1
  assert_stderr --partial "could not delete the draft release ${VERSION}"
  assert_only_calls "${READ_TAG}" "${READ_STATE}" "${WRITE_DELETE}"

  publish_repo
  : > "${STUB}/create-fails"
  run_script release --version "${VERSION}" --digest "${DIGEST}" --sha "${SHA}"
  assert_failure 1
  assert_stderr --partial "could not create the draft release ${VERSION}; no tag was created"
  assert_only_calls "${READ_TAG}" "${READ_STATE}" "${WRITE_CREATE}"

  publish_repo
  : > "${STUB}/edit-fails"
  run_script release --version "${VERSION}" --digest "${DIGEST}" --sha "${SHA}"
  assert_failure 1
  assert_stderr --partial "could not publish the draft release ${VERSION}"
  assert_only_calls "${READ_TAG}" "${READ_STATE}" "${WRITE_CREATE}" "${WRITE_EDIT}"
}

@test 'release: a tag that exists on GitHub, or cannot be read, is found out before the first write' {
  publish_repo
  printf 'draft\n' > "${STUB}/release-state"
  printf 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb0002\n' > "${STUB}/remote-tag"
  run_script release --version "${VERSION}" --digest "${DIGEST}" --sha "${SHA}"
  assert_failure 1
  assert_stderr --partial "the tag ${VERSION} exists on GitHub, at bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb0002"
  assert_only_calls "${READ_TAG}"
  printf 'unreachable\n' > "${STUB}/remote-tag"
  run_script release --version "${VERSION}" --digest "${DIGEST}" --sha "${SHA}"
  assert_failure 1
  assert_stderr --partial "cannot tell whether the tag ${VERSION} exists"
  assert_only_calls "${READ_TAG}"
  printf 'null\n' > "${STUB}/remote-tag"
  run_script release --version "${VERSION}" --digest "${DIGEST}" --sha "${SHA}"
  assert_failure 1
  assert_stderr --partial "cannot tell what the tag ${VERSION} points at"
  assert_only_calls "${READ_TAG}"
}

@test 'release: no repository, or no section for the version, writes nothing' {
  publish_repo
  set_repository ''
  run_script release --version "${VERSION}" --digest "${DIGEST}" --sha "${SHA}"
  assert_failure 1
  assert_stderr --partial 'GITHUB_REPOSITORY is not set'
  assert_only_calls
  set_repository 'gh-owner/tool'
  printf 'draft\n' > "${STUB}/release-state"
  printf '# Changelog\n\n## v1.0.0\n\nThe first stable release.\n' > "${RELEASE_REPO}/CHANGELOG.md"
  run_script release --version "${VERSION}" --digest "${DIGEST}" --sha "${SHA}"
  assert_failure 1
  assert_stderr --partial "CHANGELOG.md has no section for ${VERSION}"
  assert_only_calls
}

@test 'release: the tag at another commit after the publish is a failure, read once' {
  publish_repo
  printf 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb0002\n' > "${STUB}/tag-sha"
  run_script release --version "${VERSION}" --digest "${DIGEST}" --sha "${SHA}"
  assert_failure 1
  assert_stderr --partial 'its tag points at bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb0002'
  assert_only_calls "${READ_TAG}" "${READ_STATE}" "${WRITE_CREATE}" "${WRITE_EDIT}" "${READ_TAG}"
  assert_equal "${SLEPT}" ''
}

@test 'release: the tag read is retried a bounded number of times' {
  publish_repo
  printf '%s\n' "${SHA}" > "${STUB}/tag-sha"
  printf '2\n' > "${STUB}/tag-misses"
  run_script release --version "${VERSION}" --digest "${DIGEST}" --sha "${SHA}"
  assert_success
  assert_only_calls "${READ_TAG}" "${READ_STATE}" "${WRITE_CREATE}" "${WRITE_EDIT}" \
    "${READ_TAG}" "${READ_TAG}" "${READ_TAG}"
  assert_equal "${SLEPT}" $'2\n2'

  publish_repo
  printf '%s\n' "${SHA}" > "${STUB}/tag-sha"
  printf '2\n' > "${STUB}/tag-misses"
  : > "${STUB}/tag-miss-status"
  run_script release --version "${VERSION}" --digest "${DIGEST}" --sha "${SHA}"
  assert_success
  assert_equal "${SLEPT}" $'2\n2'

  publish_repo
  run_script release --version "${VERSION}" --digest "${DIGEST}" --sha "${SHA}"
  assert_failure 1
  assert_stderr --partial "GitHub shows no tag ${VERSION} after 5 reads"
  assert_only_calls "${READ_TAG}" "${READ_STATE}" "${WRITE_CREATE}" "${WRITE_EDIT}" \
    "${READ_TAG}" "${READ_TAG}" "${READ_TAG}" "${READ_TAG}" "${READ_TAG}"
  assert_equal "${SLEPT}" $'2\n2\n2\n2'
}

@test 'release: a malformed call is refused before anything is asked of GitHub' {
  publish_repo
  local version sha digest
  for version in '1.1.0' 'v1.1' 'v1.1.0-rc.1' 'v01.1.0'; do
    run_script release --version "${version}" --digest "${DIGEST}" --sha "${SHA}"
    assert_failure 2
    assert_stderr --partial "${version} is not vMAJOR.MINOR.PATCH"
    assert_only_calls
  done
  for sha in 'main' 'aaaaaaa' "${SHA^^}"; do
    run_script release --version "${VERSION}" --digest "${DIGEST}" --sha "${sha}"
    assert_failure 2
    assert_stderr --partial "--sha ${sha} is not a full commit id"
    assert_only_calls
  done
  for digest in 'sha256:abc' "${DIGEST^^}" "${DIGEST#sha256:}"; do
    run_script release --version "${VERSION}" --digest "${digest}" --sha "${SHA}"
    assert_failure 2
    assert_stderr --partial "--digest ${digest} is not a sha256 digest"
    assert_only_calls
  done
  local -a call
  local line
  local calls=(
    $'release\n--version\nv1.1.0\n--digest\nsha256:x'
    $'release\n--version\nv1.1.0\n--sha\nx'
    $'release\n--digest\nx\n--sha\nx'
    $'release\n--version\nv1.1.0\n--digest\nx\n--sha'
    $'release\n--bogus'
    'publish'
    ''
  )
  for line in "${calls[@]}"; do
    call=()
    if [[ -n "${line}" ]]; then
      mapfile -t call <<< "${line}"
    fi
    run_script "${call[@]}"
    assert_failure 2
    assert_only_calls
  done
}

@test 'pending: a changelog deleted and restored ends the walk at the deletion' {
  publish_repo
  git -C "${RELEASE_REPO}" rm --quiet -- CHANGELOG.md
  git -C "${RELEASE_REPO}" commit --quiet --message 'chore: drop the changelog'
  git -C "${RELEASE_REPO}" checkout --quiet HEAD~1 -- CHANGELOG.md
  git -C "${RELEASE_REPO}" commit --quiet --message 'chore: restore the changelog'
  local restored
  restored="$(git -C "${RELEASE_REPO}" rev-parse HEAD)"
  run_script pending
  assert_success
  assert_output $'publish=true\nversion='"${VERSION}"$'\nsha='"${restored}"
}

@test 'pending: a default branch that dropped the changelog proposes no version' {
  publish_repo
  local first
  first="$(git -C "${RELEASE_REPO}" rev-parse HEAD)"
  git -C "${RELEASE_REPO}" rm --quiet -- CHANGELOG.md
  git -C "${RELEASE_REPO}" commit --quiet --message 'chore: drop the changelog'
  git -C "${RELEASE_REPO}" checkout --quiet --detach "${first}"
  run_script pending
  assert_failure 1
  assert_output ''
  assert_stderr --partial "the default branch no longer proposes ${VERSION}"
  assert_stderr --partial 'names no version at its top'
}
