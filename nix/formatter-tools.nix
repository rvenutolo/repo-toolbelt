# The tools each bash formatter runs, by formatter name: the Nix packages
# whose bin/ the formatter's wrapper puts on its PATH, after the library's
# tools. This is the one place a formatter's tool is chosen; formatters.toml
# names no package. Every bash formatter in formatters.toml needs an entry,
# an empty list when it calls nothing beyond the library's tools; nix/dist.nix
# fails evaluation on a missing one.
pkgs:
let
  # goimports alone out of gotools. nixpkgs ships it as a wrapper that puts
  # the Go toolchain on PATH, and the binary embeds the toolchain's path as
  # its default GOROOT; with -format-only it never runs go nor reads GOROOT.
  # Copying the unwrapped binary and stripping that path keeps the rest of
  # gotools and the whole toolchain out of the image, and keeps every other
  # gotools command off the formatter's PATH.
  goimports =
    pkgs.runCommand "goimports-${pkgs.gotools.version}"
      {
        nativeBuildInputs = [ pkgs.removeReferencesTo ];
        disallowedReferences = [
          pkgs.go
          pkgs.gotools
        ];
        meta.mainProgram = "goimports";
      }
      ''
        install -D --mode=0755 ${pkgs.gotools}/bin/.goimports-wrapped "$out/bin/goimports"
        remove-references-to -t ${pkgs.go} "$out/bin/goimports"
      '';
in
{
  go = [
    pkgs.gofumpt
    goimports
  ];
  shell = [ pkgs.shfmt ];
}
