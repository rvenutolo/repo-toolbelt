# The lint and formatter manifests, read from lints.toml and formatters.toml
# so the packages and the image never keep a second list. Every entry's
# language must be one the packages know how to build; anything else fails
# evaluation rather than shipping an executable the image cannot run.
# formatters.toml may hold no [[formatter]] table at all, and then has no
# `formatter` attribute.
let
  known = [
    "bash"
    "go"
  ];
  checked =
    file: kind: entries:
    map (
      entry:
      if builtins.elem entry.language known then
        entry
      else
        throw "${file}: ${kind} ${entry.name} has unsupported language ${entry.language}"
    ) entries;
  lints = checked "lints.toml" "lint" (builtins.fromTOML (builtins.readFile ../lints.toml)).lint;
  formatters = checked "formatters.toml" "formatter" (
    (builtins.fromTOML (builtins.readFile ../formatters.toml)).formatter or [ ]
  );
  byLanguage = language: builtins.filter (entry: entry.language == language);
in
{
  goLints = byLanguage "go" lints;
  bashLints = byLanguage "bash" lints;
  goFormatters = byLanguage "go" formatters;
  bashFormatters = byLanguage "bash" formatters;
}
