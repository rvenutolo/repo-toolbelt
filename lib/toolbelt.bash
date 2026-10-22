#!/usr/bin/env bash

# Shared helpers for the bash lints. Each lint sources this file by a path
# relative to its own resolved location (`../../lib/toolbelt.bash`), never
# through an environment variable, so the source tree and the image share one
# layout.
#
# Callers run under `set -Eeuo pipefail` and `shopt -s inherit_errexit`, but a
# function whose status is tested (`fn || n=$?`) runs with errexit off, so every
# command here that can fail checks its own status and dies rather than relying
# on errexit.
#
# Exit statuses follow the lint calling convention: 0 no findings, 1 findings
# (config errors in the lint's own section included), 2 a setup failure.

# The consumer config file's name, relative to the repo root. The runner checks
# the file level and hands each lint its section; lints name the file only in
# messages.
readonly TOOLBELT_CONFIG='.repo-toolbelt.yml'

# The exit status of a setup failure: anything that is not a finding.
readonly TOOLBELT_SETUP_FAILURE=2

# Exit statuses are 8 bits wide, so a larger error count would wrap, and 256
# errors would read as success.
readonly TOOLBELT_MAX_STATUS=255

# The yq expressions below share two traps:
# - yq-go evaluates a string literal even when its input stream is empty, so
#   `[] | .[] | "x"` prints `x`. Every message therefore concatenates a value
#   drawn from its input, and `top level` is substituted for an empty path with
#   `sub` rather than written as a fallback literal.
# - yq-go's string escapes are unreliable, so a single quote cannot be spelled
#   inside a yq string. Each `'"'"'` ends the shell quote, adds a literal single
#   quote and reopens it.

# yq fragment printing a value, a key in particular, on one line: JSON string
# escaping without the surrounding quotes, so `a<newline>b` prints as `a\nb`.
# Keys are user data, and a raw newline in one would split a one-per-line
# listing into names that look allowed; a raw carriage return would garble the
# terminal line.
readonly TOOLBELT_ESCAPE_EXPR='(tostring | @json | sub("^.", "") | sub(".$", ""))'

# yq fragment turning a path array into a `where`: `section.key[index]`, with
# keys escaped, or `top level` for the root.
readonly TOOLBELT_WHERE_EXPR='(map(select(tag == "!!str") |= "." + '"${TOOLBELT_ESCAPE_EXPR}"')
  | map(select(tag == "!!int") |= "[" + tostring + "]") | join("") | sub("^\.", "") | sub("^$", "top level"))'

# yq fragment, applied to a tag string, that is true for a YAML core tag. Any
# other tag changes what a node means to each parser that reads it.
readonly TOOLBELT_CORE_TAG_EXPR='(. == "!!map" or . == "!!seq" or . == "!!str" or . == "!!null" or . == "!!int"
  or . == "!!bool" or . == "!!float")'

# yq expression listing every tag other than the core ones, on keys or on
# values, anywhere in the file, one message each. A read that tests a node's
# `type` sees its tag, so a structural map or list carrying a custom tag would
# read as neither.
# shellcheck disable=SC2016,SC2034 # $p is a yq variable; the lints that source this file read it
readonly TOOLBELT_UNSUPPORTED_TAGS_EXPR='
([.. | select(kind == "map") | {"path": path, "tags": [keys | .[] | tag
    | select(. != "!!merge") | select('"${TOOLBELT_CORE_TAG_EXPR}"' | not)]}]
  | map(.path as $p | .tags | map({"path": $p, "tag": .})) | flatten(1) | .[]
  | "YAML tag '"'"'" + (.tag | '"${TOOLBELT_ESCAPE_EXPR}"') + "'"'"' is not supported (in "
    + (.path | '"${TOOLBELT_WHERE_EXPR}"') + "); quote the value or drop the tag"),
(.. | select(kind != "alias") | select(tag | '"${TOOLBELT_CORE_TAG_EXPR}"' | not)
  | "YAML tag '"'"'" + (tag | '"${TOOLBELT_ESCAPE_EXPR}"') + "'"'"' is not supported (in "
    + (path | '"${TOOLBELT_WHERE_EXPR}"') + "); quote the value or drop the tag")
'

# yq expression listing, escaped and one per line, every key of the map it is
# given that is not in the comma-separated TOOLBELT_ALLOWED environment
# variable. The comparison happens inside yq, on the keys themselves.
# shellcheck disable=SC2034 # read by the lints that source this file
readonly TOOLBELT_UNKNOWN_KEYS_EXPR='(keys | unique) - (strenv(TOOLBELT_ALLOWED) | split(","))
  | .[] | '"${TOOLBELT_ESCAPE_EXPR}"

# yq fragment, applied to a stream of nodes, listing every duplicate mapping key
# among them, one message per key. yq-go keeps only the last of a duplicated
# key, so the duplicate is invisible to every later read.
# shellcheck disable=SC2016,SC2034 # $where is a yq variable; the lints that source this file read it
readonly TOOLBELT_DUPLICATE_KEYS_FRAGMENT='
select(kind == "map")
| select((keys | length) > (keys | unique | length))
| (path | '"${TOOLBELT_WHERE_EXPR}"') as $where
| keys | group_by(.) | map(select(length > 1) | .[0]) | .[]
| "duplicate key '"'"'" + '"${TOOLBELT_ESCAPE_EXPR}"' + "'"'"' in " + $where
'

# @description Print `<script-name>: <message>` to stderr and exit with the
#              setup-failure status. Inside a command substitution it exits
#              only that subshell, so callers test the substitution's status.
# @arg $1 message what went wrong
# @stderr the message, prefixed with the script's name
# @exitcode 2 always
function toolbelt::die() {
  local -r message="$1"
  printf '%s: %s\n' "${0##*/}" "${message}" >&2
  exit "${TOOLBELT_SETUP_FAILURE}"
}

# @description Print one config error as `.repo-toolbelt.yml: <problem>`. This
#              is a finding, part of the lint's report, not a log line.
# @arg $1 problem the problem with the config file
# @stdout the prefixed problem
function toolbelt::config_error() {
  local -r problem="$1"
  printf '%s: %s\n' "${TOOLBELT_CONFIG}" "${problem}"
}

# @description Print one error in a lint's own section of the config file.
# @arg $1 lint the lint whose section holds the error
# @arg $2 problem the problem, without the section prefix
# @stdout `.repo-toolbelt.yml: <lint>: <problem>`
function toolbelt::section_error() {
  local -r lint="$1"
  local -r problem="$2"
  toolbelt::config_error "${lint}: ${problem}"
}

# @description Store a value in the named variable in a form that prints on one
#              line: newline, carriage return and tab as `\n`, `\r` and `\t`,
#              and any other control character as `?`, so user data cannot
#              split or rewrite a message line.
# @arg $1 result_var name of the variable to set
# @arg $2 value the value to show
function toolbelt::printable() {
  local -n printable_ref="$1"
  local value="$2"
  value="${value//$'\n'/\\n}"
  value="${value//$'\r'/\\r}"
  value="${value//$'\t'/\\t}"
  # shellcheck disable=SC2034 # set through the nameref; the caller reads it
  printable_ref="${value//[[:cntrl:]]/?}"
}

# @description Print the absolute root of the enclosing git repository. Call it
#              as `if ! root="$(toolbelt::repo_root)"; then exit 2; fi`: the
#              die inside runs in the substitution's subshell.
# @noargs
# @stdout the absolute repo root
# @exitcode 2 not inside a git repository
function toolbelt::repo_root() {
  local root
  if ! root="$(git rev-parse --show-toplevel 2> '/dev/null')"; then
    toolbelt::die 'not inside a git repository'
  fi
  printf '%s\n' "${root}"
}

# @description Run yq with its stdout in out_file and its stderr in err_file,
#              so no yq warning reaches the lint's output. Dies with yq's
#              stderr when yq fails.
# @arg $1 out_file where to write yq's stdout
# @arg $2 err_file where to write yq's stderr
# @arg $@ args yq's arguments, after the first two
function toolbelt::yq() {
  local -r out_file="$1"
  local -r err_file="$2"
  shift 2
  if ! yq "$@" > "${out_file}" 2> "${err_file}"; then
    toolbelt::die "yq failed: $(< "${err_file}")"
  fi
}

# @description Fail loudly when a file a loop is about to read is missing.
#              Errexit is off in the validators, so a missing file would
#              otherwise read as zero errors.
# @arg $1 file the file that must exist
function toolbelt::require_file() {
  local -r file="$1"
  [[ -f "${file}" ]] || toolbelt::die "missing ${file}"
}

# @description Return an error count as an exit status, capped at 255 so that
#              256 errors cannot wrap to success.
# @arg $1 errors the error count
# @exitcode * the capped count
function toolbelt::return_count() {
  local -r errors="$1"
  if ((errors > TOOLBELT_MAX_STATUS)); then
    return "${TOOLBELT_MAX_STATUS}"
  fi
  return "${errors}"
}
