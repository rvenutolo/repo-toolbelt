#!/usr/bin/env bats

bats_require_minimum_version 1.5.0

# Fixture text. Nothing in a fixture is ever executed: the formatter only
# parses and prints it, so each payload is an inert marker.
readonly UNFORMATTED_SH=$'if true;then\necho PAYLOAD_RAN\nfi\n'
readonly FORMATTED_SH=$'if true; then\n  echo PAYLOAD_RAN\nfi\n'
readonly FORMATTED_SH_INDENT_4=$'if true; then\n    echo PAYLOAD_RAN\nfi\n'
readonly UNFORMATTED_SCRIPT=$'#!/usr/bin/env bash\nif true;then\necho PAYLOAD_RAN\nfi\n'
readonly FORMATTED_SCRIPT=$'#!/usr/bin/env bash\nif true; then\n  echo PAYLOAD_RAN\nfi\n'
readonly UNFORMATTED_BATS=$'@test "x" {\necho PAYLOAD_RAN\n}\n'
readonly FORMATTED_BATS=$'@test "x" {\n  echo PAYLOAD_RAN\n}\n'
readonly UNPARSABLE=$'if then\n'
# shellcheck disable=SC2016 # the backticks are shfmt's message text, not a command substitution
readonly PARSE_TEXT='`if` must be followed by a statement list'
readonly PARSE_MESSAGE="1:1: ${PARSE_TEXT}"

function setup() {
  load test_helper/common
  FORMATTER_PATH="${REPO_DIR}/formatters/shell/shell"
  WORK_DIR="${BATS_TEST_TMPDIR}/work"
  mkdir --parents "${WORK_DIR}"
  cd "${WORK_DIR}" || return 1
  # Temp files belong under TMPDIR, never in the checkout; a directory of the
  # test's own shows whether the formatter cleans up after itself.
  export TMPDIR="${BATS_TEST_TMPDIR}/tmp"
  mkdir --parents "${TMPDIR}"
  set_mode check
  # The runner always hands a formatter a section file; `null` is what it
  # writes when the config file has no section.
  write_section <<< 'null'
  write_list
  REAL_SHFMT="$(command -v shfmt)"
  export REAL_SHFMT
  export SHFMT_CALLS="${BATS_TEST_TMPDIR}/shfmt-calls"
}

# A test that makes a file unreadable restores it here, so bats can delete the
# test's temp dir.
function teardown() {
  if [[ -n "${LOCKED_FILE:-}" ]]; then
    chmod u+rw -- "${LOCKED_FILE}"
  fi
}

# @description Set the formatter's mode for the runs that follow, as
#              TOOLBELT_MODE.
# @arg $1 mode check or write, or any other value to test its rejection
function set_mode() {
  local -r mode="$1"
  export TOOLBELT_MODE="${mode}"
}

# @description Write the given paths, each terminated by NUL, to a file
#              outside the work dir, and export its path as TOOLBELT_FILES, as
#              the runner does.
# @arg $@ paths the repo-relative paths to list
function write_list() {
  local -r list_file="${BATS_TEST_TMPDIR}/files"
  : > "${list_file}"
  local path
  for path in "$@"; do
    printf '%s\0' "${path}" >> "${list_file}"
  done
  export TOOLBELT_FILES="${list_file}"
}

# @description Write text to a file in the work dir, exactly, with no trailing
#              newline added.
# @arg $1 path the file's path relative to the work dir
# @arg $2 text the content
function write_file() {
  local -r path="$1"
  local -r text="$2"
  printf '%s' "${text}" > "${WORK_DIR}/${path}"
}

# @description Assert a file in the work dir holds exactly the given text.
# @arg $1 path the file's path relative to the work dir
# @arg $2 text the expected content
function assert_content() {
  local -r path="$1"
  local -r text="$2"
  local actual
  # The x keeps any trailing newlines through the command substitution.
  actual="$(cat -- "${WORK_DIR}/${path}" && printf 'x')"
  assert_equal "${actual%x}" "${text}"
}

# @description Print a checksum of every file in the work dir, with its name,
#              so a run that writes, adds or removes a file changes the output.
# @noargs
function snapshot() {
  find "${WORK_DIR}" -type f -printf '%P\0' | sort --zero-terminated \
    | (cd "${WORK_DIR}" && xargs --null --no-run-if-empty --max-args=100 sha256sum --)
}

# @description Put a stub shfmt first on PATH. It records the count of path
#              arguments after `--` of each call, one line per call, in
#              SHFMT_CALLS, then runs the real shfmt.
# @noargs
function stub_recording_shfmt() {
  mkdir --parents "${BATS_TEST_TMPDIR}/bin"
  cat > "${BATS_TEST_TMPDIR}/bin/shfmt" << 'EOF'
#!/usr/bin/env bash
paths=0
after=0
for arg in "$@"; do
  if ((after == 1)); then
    paths=$((paths + 1))
  elif [[ "${arg}" == '--' ]]; then
    after=1
  fi
done
printf '%s\n' "${paths}" >> "${SHFMT_CALLS}"
exec "${REAL_SHFMT}" "$@"
EOF
  chmod +x "${BATS_TEST_TMPDIR}/bin/shfmt"
  PATH="${BATS_TEST_TMPDIR}/bin:${PATH}"
}

# @description Put a stub shfmt first on PATH whose body is the given text.
# @arg $1 body the stub's bash body, after the shebang
function stub_shfmt() {
  local -r body="$1"
  mkdir --parents "${BATS_TEST_TMPDIR}/bin"
  printf '#!/usr/bin/env bash\n%s\n' "${body}" > "${BATS_TEST_TMPDIR}/bin/shfmt"
  chmod +x "${BATS_TEST_TMPDIR}/bin/shfmt"
  PATH="${BATS_TEST_TMPDIR}/bin:${PATH}"
}

# @description Assert TMPDIR holds nothing after a run.
# @noargs
function assert_tmpdir_empty() {
  run find "${TMPDIR}" -mindepth 1
  assert_output ''
}

@test '--help prints the usage and exits 0' {
  run --separate-stderr "${FORMATTER_PATH}" --help
  assert_success
  assert_output 'usage: shell [--help]'
  assert_stderr ''
}

@test 'an unexpected argument is a setup failure' {
  run --separate-stderr "${FORMATTER_PATH}" --bogus
  assert_failure 2
  assert_output ''
  assert_stderr --partial "unexpected argument '--bogus'"
}

@test 'more than one argument is a setup failure' {
  run --separate-stderr "${FORMATTER_PATH}" --help --help
  assert_failure 2
  assert_stderr --partial 'expected at most one argument, got 2'
}

@test 'check mode reports an unformatted .sh, .bats and extensionless bash script and writes nothing' {
  write_file a.sh "${UNFORMATTED_SH}"
  write_file b.bats "${UNFORMATTED_BATS}"
  write_file run-it "${UNFORMATTED_SCRIPT}"
  write_list a.sh b.bats run-it
  local before
  before="$(snapshot)"
  run --separate-stderr "${FORMATTER_PATH}"
  assert_failure 1
  assert_output "$(printf '%s\n' 'a.sh: not formatted' 'b.bats: not formatted' 'run-it: not formatted' '3 file(s)')"
  assert_stderr ''
  assert_equal "$(snapshot)" "${before}"
  assert_tmpdir_empty
}

@test 'check mode passes formatted files' {
  write_file a.sh "${FORMATTED_SH}"
  write_file b.bats "${FORMATTED_BATS}"
  write_file run-it "${FORMATTED_SCRIPT}"
  write_file .envrc "${FORMATTED_SH}"
  write_list a.sh b.bats run-it .envrc
  run --separate-stderr "${FORMATTER_PATH}"
  assert_success
  assert_output ''
  assert_stderr ''
}

@test 'check mode reports only the files that would change, in list order, with the count' {
  write_file z.sh "${UNFORMATTED_SH}"
  write_file m.sh "${FORMATTED_SH}"
  write_file a.bats "${UNFORMATTED_BATS}"
  write_file .envrc "${UNFORMATTED_SH}"
  write_list z.sh m.sh a.bats .envrc
  run --separate-stderr "${FORMATTER_PATH}"
  assert_failure 1
  assert_output "$(printf '%s\n' 'z.sh: not formatted' 'a.bats: not formatted' '.envrc: not formatted' '3 file(s)')"
}

@test 'a file shfmt cannot parse is a finding, in check mode' {
  write_file bad.sh "${UNPARSABLE}"
  write_file ugly.sh "${UNFORMATTED_SH}"
  write_list bad.sh ugly.sh
  local before
  before="$(snapshot)"
  run --separate-stderr "${FORMATTER_PATH}"
  assert_failure 1
  assert_output "$(printf '%s\n' "bad.sh: cannot be parsed: ${PARSE_MESSAGE}" 'ugly.sh: not formatted' '2 file(s)')"
  assert_stderr ''
  assert_equal "$(snapshot)" "${before}"
}

@test 'a .bats file parses under the bats dialect' {
  write_file t.bats "${FORMATTED_BATS}"
  write_list t.bats
  run --separate-stderr "${FORMATTER_PATH}"
  assert_success
  assert_output ''
  assert_stderr ''
}

@test 'the same @test syntax in a .sh file is a parse finding' {
  write_file t.sh "${FORMATTED_BATS}"
  write_file u.sh "#!/usr/bin/env bash"$'\n'"${FORMATTED_BATS}"
  write_list t.sh u.sh
  run --separate-stderr "${FORMATTER_PATH}"
  assert_failure 1
  assert_line --index 0 --regexp '^t\.sh: cannot be parsed: 3:1: '
  assert_line --index 1 --regexp '^u\.sh: cannot be parsed: 4:1: '
  assert_line --index 2 '2 file(s)'
  assert_stderr ''
}

@test 'a .sh file with a #!/bin/sh shebang is held to POSIX; with no shebang it parses as bash' {
  local -r body=$'function f() {\n  echo PAYLOAD_RAN\n}\n'
  write_file posix.sh "#!/bin/sh"$'\n'"${body}"
  write_file plain.sh "${body}"
  write_list posix.sh plain.sh
  run --separate-stderr "${FORMATTER_PATH}"
  assert_failure 1
  assert_line --index 0 --regexp '^posix\.sh: cannot be parsed: 2:[0-9]+: .*a bash feature.*posix'
  assert_line --index 1 '1 file(s)'
  assert_equal "${#lines[@]}" 2
  assert_stderr ''
}

@test 'settings.indent replaces the default indent' {
  write_section <<< '{"settings": {"indent": 4}}'
  write_file four.sh "${FORMATTED_SH_INDENT_4}"
  write_file two.sh "${FORMATTED_SH}"
  write_list four.sh two.sh
  run --separate-stderr "${FORMATTER_PATH}"
  assert_failure 1
  assert_output "$(printf '%s\n' 'two.sh: not formatted' '1 file(s)')"
}

@test 'a section without settings.indent uses the default indent of 2' {
  write_section <<< '{"exclude": ["vendored/**"], "settings": {}}'
  write_file four.sh "${FORMATTED_SH_INDENT_4}"
  write_file two.sh "${FORMATTED_SH}"
  write_list four.sh two.sh
  run --separate-stderr "${FORMATTER_PATH}"
  assert_failure 1
  assert_output "$(printf '%s\n' 'four.sh: not formatted' '1 file(s)')"
}

@test 'write mode applies settings.indent' {
  set_mode write
  write_section <<< '{"settings": {"indent": 4}}'
  write_file a.sh "${UNFORMATTED_SH}"
  write_list a.sh
  run --separate-stderr "${FORMATTER_PATH}"
  assert_success
  assert_content a.sh "${FORMATTED_SH_INDENT_4}"
}

@test 'an empty list passes in check mode without running shfmt' {
  stub_recording_shfmt
  run --separate-stderr "${FORMATTER_PATH}"
  assert_success
  assert_output ''
  assert_stderr ''
  refute [ -e "${SHFMT_CALLS}" ]
}

@test 'an empty list does nothing in write mode without running shfmt' {
  set_mode write
  stub_recording_shfmt
  run --separate-stderr "${FORMATTER_PATH}"
  assert_success
  assert_output ''
  assert_stderr ''
  refute [ -e "${SHFMT_CALLS}" ]
}

@test 'a list of only bats files never runs shfmt without paths' {
  stub_recording_shfmt
  write_file t.bats "${UNFORMATTED_BATS}"
  write_list t.bats
  run --separate-stderr "${FORMATTER_PATH}"
  assert_failure 1
  assert_output "$(printf '%s\n' 't.bats: not formatted' '1 file(s)')"
  run cat -- "${SHFMT_CALLS}"
  assert_output '1'
}

@test 'a list of only non-bats files never runs shfmt without paths' {
  stub_recording_shfmt
  write_file a.sh "${UNFORMATTED_SH}"
  write_list a.sh
  run --separate-stderr "${FORMATTER_PATH}"
  assert_failure 1
  assert_output "$(printf '%s\n' 'a.sh: not formatted' '1 file(s)')"
  run cat -- "${SHFMT_CALLS}"
  assert_output '1'
}

@test 'write mode with only bats or only non-bats files never runs shfmt without paths' {
  set_mode write
  stub_recording_shfmt
  write_file t.bats "${UNFORMATTED_BATS}"
  write_list t.bats
  run --separate-stderr "${FORMATTER_PATH}"
  assert_success
  write_file a.sh "${UNFORMATTED_SH}"
  write_list a.sh
  run --separate-stderr "${FORMATTER_PATH}"
  assert_success
  # Each run: one listing pass, then one format of the one changed file.
  run cat -- "${SHFMT_CALLS}"
  assert_output "$(printf '%s\n' 1 1 1 1)"
  assert_content t.bats "${FORMATTED_BATS}"
  assert_content a.sh "${FORMATTED_SH}"
}

@test 'paths with a space, a newline or a leading dash, or a bare dash, each reach shfmt as one path' {
  write_file 'sp ace.sh' "${UNFORMATTED_SH}"
  write_file $'new\nline.sh' "${UNFORMATTED_SH}"
  write_file '-lead.sh' "${UNFORMATTED_SH}"
  write_file '-' "${UNFORMATTED_SCRIPT}"
  write_file '--' "${FORMATTED_SCRIPT}"
  write_list 'sp ace.sh' $'new\nline.sh' '-lead.sh' '-' '--'
  run --separate-stderr "${FORMATTER_PATH}" < '/dev/null'
  assert_failure 1
  assert_output "$(printf '%s\n' 'sp ace.sh: not formatted' 'new\nline.sh: not formatted' \
    '-lead.sh: not formatted' '-: not formatted' '4 file(s)')"
  assert_stderr ''
}

@test 'write mode rewrites paths with a space, a newline or a leading dash, and a bare dash' {
  set_mode write
  write_file 'sp ace.sh' "${UNFORMATTED_SH}"
  write_file $'new\nline.sh' "${UNFORMATTED_SH}"
  write_file '-lead.sh' "${UNFORMATTED_SH}"
  write_file '-' "${UNFORMATTED_SCRIPT}"
  write_list 'sp ace.sh' $'new\nline.sh' '-lead.sh' '-'
  run --separate-stderr "${FORMATTER_PATH}" < '/dev/null'
  assert_success
  assert_output ''
  assert_stderr ''
  assert_content 'sp ace.sh' "${FORMATTED_SH}"
  assert_content $'new\nline.sh' "${FORMATTED_SH}"
  assert_content '-lead.sh' "${FORMATTED_SH}"
  assert_content '-' "${FORMATTED_SCRIPT}"
}

@test 'a parse error in a path with a newline, a leading dash or a bare dash is a finding for that path' {
  write_file $'bad\nnl.sh' "${UNPARSABLE}"
  write_file '-bad.sh' "${UNPARSABLE}"
  write_file '-' "#!/usr/bin/env bash"$'\n'"${UNPARSABLE}"
  write_list $'bad\nnl.sh' '-bad.sh' '-'
  run --separate-stderr "${FORMATTER_PATH}" < '/dev/null'
  assert_failure 1
  assert_output "$(printf '%s\n' "bad\\nnl.sh: cannot be parsed: ${PARSE_MESSAGE}" \
    "-bad.sh: cannot be parsed: ${PARSE_MESSAGE}" "-: cannot be parsed: 2:1: ${PARSE_TEXT}" \
    '3 file(s)')"
  assert_stderr ''
}

@test 'an .editorconfig asking for tabs does not change the result' {
  printf 'root = true\n\n[*]\nindent_style = tab\nindent_size = 8\n' > "${WORK_DIR}/.editorconfig"
  write_file spaces.sh "${FORMATTED_SH}"
  write_file tabs.sh $'if true; then\n\techo PAYLOAD_RAN\nfi\n'
  write_list spaces.sh tabs.sh
  run --separate-stderr "${FORMATTER_PATH}"
  assert_failure 1
  assert_output "$(printf '%s\n' 'tabs.sh: not formatted' '1 file(s)')"
  set_mode write
  run --separate-stderr "${FORMATTER_PATH}"
  assert_success
  assert_content tabs.sh "${FORMATTED_SH}"
}

@test 'write mode rewrites each unformatted file and exits 0' {
  set_mode write
  write_file a.sh "${UNFORMATTED_SH}"
  write_file b.bats "${UNFORMATTED_BATS}"
  write_file run-it "${UNFORMATTED_SCRIPT}"
  write_file ok.sh "${FORMATTED_SH}"
  write_list a.sh b.bats run-it ok.sh
  run --separate-stderr "${FORMATTER_PATH}"
  assert_success
  assert_output ''
  assert_stderr ''
  assert_content a.sh "${FORMATTED_SH}"
  assert_content b.bats "${FORMATTED_BATS}"
  assert_content run-it "${FORMATTED_SCRIPT}"
  assert_content ok.sh "${FORMATTED_SH}"
  # Nothing but the listed files is left in the checkout, and TMPDIR is clean.
  run bash -c 'find "$1" -mindepth 1 -printf "%P\n" | LC_ALL=C sort' _ "${WORK_DIR}"
  assert_output "$(printf '%s\n' a.sh b.bats ok.sh run-it)"
  assert_tmpdir_empty
}

@test 'write mode rewrites in place, keeping mode bits, inode and hard links' {
  set_mode write
  write_file run-it "${UNFORMATTED_SCRIPT}"
  chmod 0755 -- "${WORK_DIR}/run-it"
  ln -- "${WORK_DIR}/run-it" "${BATS_TEST_TMPDIR}/hard-link"
  local inode_before
  inode_before="$(stat --format='%i' -- "${WORK_DIR}/run-it")"
  write_list run-it
  run --separate-stderr "${FORMATTER_PATH}"
  assert_success
  assert_content run-it "${FORMATTED_SCRIPT}"
  assert_equal "$(stat --format='%a' -- "${WORK_DIR}/run-it")" '755'
  assert_equal "$(stat --format='%i' -- "${WORK_DIR}/run-it")" "${inode_before}"
  assert_equal "$(stat --format='%h' -- "${WORK_DIR}/run-it")" '2'
  run cat -- "${BATS_TEST_TMPDIR}/hard-link"
  assert_output "${FORMATTED_SCRIPT%$'\n'}"
}

@test 'write mode reports a file it cannot parse, leaves it untouched and exits 1' {
  set_mode write
  write_file bad.sh "${UNPARSABLE}"
  write_file ugly.sh "${UNFORMATTED_SH}"
  write_list bad.sh ugly.sh
  run --separate-stderr "${FORMATTER_PATH}"
  assert_failure 1
  assert_output "$(printf '%s\n' "bad.sh: cannot be parsed: ${PARSE_MESSAGE}" '1 file(s)')"
  assert_stderr ''
  assert_content bad.sh "${UNPARSABLE}"
  assert_content ugly.sh "${FORMATTED_SH}"
}

@test 'write mode: a file it cannot write is a setup failure, not a finding' {
  set_mode write
  write_file locked.sh "${UNFORMATTED_SH}"
  chmod 0444 -- "${WORK_DIR}/locked.sh"
  LOCKED_FILE="${WORK_DIR}/locked.sh"
  write_list locked.sh
  run --separate-stderr "${FORMATTER_PATH}"
  assert_failure 2
  assert_output ''
  assert_stderr --partial 'shell: cannot write locked.sh'
  assert_content locked.sh "${UNFORMATTED_SH}"
}

@test 'a file shfmt cannot read is a setup failure, not a finding' {
  write_file locked.sh "${FORMATTED_SH}"
  chmod 0000 -- "${WORK_DIR}/locked.sh"
  LOCKED_FILE="${WORK_DIR}/locked.sh"
  write_list locked.sh
  run --separate-stderr "${FORMATTER_PATH}"
  assert_failure 2
  assert_output ''
  assert_stderr --partial 'permission denied'
}

@test 'shfmt stderr that is not a parse error is a setup failure' {
  stub_shfmt "printf 'shfmt: something broke\n' >&2; exit 1"
  write_file a.sh "${FORMATTED_SH}"
  write_list a.sh
  run --separate-stderr "${FORMATTER_PATH}"
  assert_failure 2
  assert_output ''
  assert_stderr --partial 'shfmt: something broke'
}

@test 'a parse error naming a path that is not listed is a setup failure' {
  stub_shfmt "printf 'other.sh:1:1: \`if\` must be followed by a statement list\n' >&2; exit 1"
  write_file a.sh "${FORMATTED_SH}"
  write_list a.sh
  run --separate-stderr "${FORMATTER_PATH}"
  assert_failure 2
  assert_output ''
  assert_stderr --partial 'other.sh:1:1:'
}

@test 'shfmt listing a path it was not given is a setup failure' {
  stub_shfmt "printf 'other.sh\\0'; exit 1"
  write_file a.sh "${FORMATTED_SH}"
  write_list a.sh
  run --separate-stderr "${FORMATTER_PATH}"
  assert_failure 2
  assert_output ''
  assert_stderr --partial 'shfmt listed a file it was not given: other.sh'
}

@test 'a parse error line followed by other stderr is a setup failure' {
  stub_shfmt "printf 'a.sh:1:1: bad\nshfmt: something broke\n' >&2; exit 1"
  write_file a.sh "${FORMATTED_SH}"
  write_list a.sh
  run --separate-stderr "${FORMATTER_PATH}"
  assert_failure 2
  assert_output ''
  assert_stderr --partial 'shfmt: something broke'
}

@test 'shfmt failing with no output is a setup failure' {
  stub_shfmt 'exit 1'
  write_file a.sh "${FORMATTED_SH}"
  write_list a.sh
  run --separate-stderr "${FORMATTER_PATH}"
  assert_failure 2
  assert_output ''
  assert_stderr --partial 'shfmt exited 1'
}

@test 'shfmt exiting with a status other than 0 or 1 is a setup failure' {
  stub_shfmt 'exit 3'
  write_file a.sh "${FORMATTED_SH}"
  write_list a.sh
  run --separate-stderr "${FORMATTER_PATH}"
  assert_failure 2
  assert_output ''
  assert_stderr --partial 'shfmt exited 3'
}

@test 'write mode: shfmt failing to format a listed file is a setup failure' {
  set_mode write
  # Lists the file as unformatted, then fails to format it.
  stub_shfmt "if [[ \" \$* \" == *' --list=0 '* ]]; then printf 'a.sh\0'; exit 1; fi
printf 'shfmt: crashed\n' >&2; exit 1"
  write_file a.sh "${UNFORMATTED_SH}"
  write_list a.sh
  run --separate-stderr "${FORMATTER_PATH}"
  assert_failure 2
  assert_output ''
  assert_stderr --partial 'shfmt: crashed'
  assert_content a.sh "${UNFORMATTED_SH}"
}

@test 'shfmt missing from PATH is a setup failure' {
  # The devShell puts each package's bin/ on PATH on its own, so dropping
  # shfmt's directory drops shfmt alone.
  local shfmt_dir
  shfmt_dir="$(dirname -- "${REAL_SHFMT}")"
  local kept='' dir
  local -a dirs
  IFS=':' read -r -a dirs <<< "${PATH}"
  for dir in "${dirs[@]}"; do
    if [[ "${dir}" != "${shfmt_dir}" ]]; then
      kept="${kept:+${kept}:}${dir}"
    fi
  done
  write_file a.sh "${FORMATTED_SH}"
  write_list a.sh
  PATH="${kept}"
  run --separate-stderr "${FORMATTER_PATH}"
  assert_failure 2
  assert_output ''
  assert_stderr --partial 'shfmt not found on PATH'
}

@test 'an unknown TOOLBELT_MODE is a setup failure' {
  set_mode fix
  run --separate-stderr "${FORMATTER_PATH}"
  assert_failure 2
  assert_stderr --partial "TOOLBELT_MODE must be 'check' or 'write', got 'fix'"
}

@test 'TOOLBELT_MODE unset is a setup failure' {
  unset TOOLBELT_MODE
  run --separate-stderr "${FORMATTER_PATH}"
  assert_failure 2
  assert_stderr --partial "TOOLBELT_MODE must be 'check' or 'write', got ''"
}

@test 'TOOLBELT_FILES unset is a setup failure' {
  unset TOOLBELT_FILES
  run --separate-stderr "${FORMATTER_PATH}"
  assert_failure 2
  assert_stderr --partial 'TOOLBELT_FILES is not set'
}

@test 'an unreadable file list is a setup failure' {
  chmod 0000 -- "${TOOLBELT_FILES}"
  LOCKED_FILE="${TOOLBELT_FILES}"
  run --separate-stderr "${FORMATTER_PATH}"
  assert_failure 2
  assert_stderr --partial "cannot read the file list ${TOOLBELT_FILES}"
}

@test 'a missing file list is a setup failure' {
  export TOOLBELT_FILES="${BATS_TEST_TMPDIR}/no-such-list"
  run --separate-stderr "${FORMATTER_PATH}"
  assert_failure 2
  assert_stderr --partial "cannot read the file list ${TOOLBELT_FILES}"
}

@test 'TOOLBELT_SECTION unset is a setup failure' {
  unset TOOLBELT_SECTION
  run --separate-stderr "${FORMATTER_PATH}"
  assert_failure 2
  assert_stderr --partial 'TOOLBELT_SECTION is not set'
}

@test 'TOOLBELT_SECTION naming no file is a setup failure' {
  export TOOLBELT_SECTION="${BATS_TEST_TMPDIR}/no-such-section"
  run --separate-stderr "${FORMATTER_PATH}"
  assert_failure 2
  assert_stderr --partial "TOOLBELT_SECTION names no file: ${TOOLBELT_SECTION}"
}
