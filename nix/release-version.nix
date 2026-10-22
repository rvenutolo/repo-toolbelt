# The version a changelog's newest release names, without its `v`, read the
# way `.ci/release-version top` reads it: the first line that starts with `## `
# is the section heading, and it names a version only when it matches that
# script's HEADING_PATTERN, which is taken from the script itself so there is
# one copy of it. Where `top` names no version, or refuses the heading, the
# answer is the fallback, so evaluating the flake never fails on a changelog.
# Builtins only, so a test can evaluate it without nixpkgs.
{
  # The changelog's text.
  changelog,
  # The text of .ci/release-version.
  script,
  # What to answer when the changelog names no release.
  fallback,
}:
let
  lines = text: builtins.filter builtins.isString (builtins.split "\n" text);
  firstWith =
    prefix: text:
    let
      found = builtins.filter (line: builtins.substring 0 (builtins.stringLength prefix) line == prefix) (
        lines text
      );
    in
    if found == [ ] then null else builtins.head found;

  patternLine = firstWith "readonly HEADING_PATTERN='" script;
  patternMatch =
    if patternLine == null then null else builtins.match "readonly HEADING_PATTERN='(.*)'" patternLine;
  pattern =
    if patternMatch == null then
      throw "nix/release-version.nix: .ci/release-version has no HEADING_PATTERN line"
    else
      builtins.head patternMatch;

  heading = firstWith "## " changelog;
  versionMatch = if heading == null then null else builtins.match pattern heading;
  version = builtins.head versionMatch;
in
if versionMatch == null then
  fallback
else
  builtins.substring 1 (builtins.stringLength version - 1) version
