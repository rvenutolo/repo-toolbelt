#!/usr/bin/env bats

# The shared helper's isolation of fixture repos from the host's git setup.
# A host file is planted under a stand-in XDG_CONFIG_HOME before the helper
# loads, as a developer's ~/.config would be.

function setup() {
  local -r host_config="${BATS_TEST_TMPDIR}/host-config"
  mkdir --parents "${host_config}/git"
  printf 'vendor/\n' > "${host_config}/git/ignore"
  printf '* linguist-vendored\n' > "${host_config}/git/attributes"
  export XDG_CONFIG_HOME="${host_config}"
  load test_helper/common
  make_repo
}

@test "a fixture repo ignores the host's global excludes file" {
  mkdir vendor
  printf 'x\n' > vendor/lib.txt
  git add vendor/lib.txt

  run git ls-files
  assert_success
  assert_output 'vendor/lib.txt'
}

@test "a fixture repo ignores the host's global attributes file" {
  printf 'x\n' > lib.txt

  run git check-attr linguist-vendored -- lib.txt
  assert_success
  assert_output 'lib.txt: linguist-vendored: unspecified'
}
