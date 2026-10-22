# The lint manifest, read from lints.toml so the packages and the image never
# keep a second list. Every lint's language must be one the packages know how
# to build; anything else fails evaluation rather than shipping a lint the
# image cannot run.
let
  lints = (builtins.fromTOML (builtins.readFile ../lints.toml)).lint;
  known = [
    "bash"
    "go"
  ];
  checked = map (
    lint:
    if builtins.elem lint.language known then
      lint
    else
      throw "lints.toml: lint ${lint.name} has unsupported language ${lint.language}"
  ) lints;
  byLanguage = language: builtins.filter (lint: lint.language == language) checked;
in
{
  goLints = byLanguage "go";
  bashLints = byLanguage "bash";
}
