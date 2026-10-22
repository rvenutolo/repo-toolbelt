{ pkgs, ... }:
{
  projectRootFile = "flake.nix";

  programs = {
    nixfmt.enable = true;
    gofumpt.enable = true;
    goimports.enable = true;
    yamlfmt.enable = true;
    prettier = {
      enable = true;
      includes = [ "*.json" ];
    };
  };

  # shfmt via explicit entries rather than programs.shfmt: that module
  # hardcodes `-s` (simplify), which strips quotes inside `[[ ]]` and
  # contradicts the quote-every-expansion rule. Two entries because the bats
  # dialect is a parser flag that must not apply to the other files. The
  # extensionless scripts are listed by directory because treefmt matches on
  # globs, not shebangs.
  settings.formatter = {
    shfmt = {
      command = "${pkgs.shfmt}/bin/shfmt";
      options = [
        "--write"
        "--indent"
        "2"
        "--case-indent"
        "--binary-next-line"
        "--space-redirects"
      ];
      includes = [
        "*.bash"
        ".ci/*"
        ".githooks/*"
        "lib/*"
        "lints/*"
        "run-all-checks"
        "run-tests"
      ];
      # `lints/*` and `lib/*` can also match Go sources and Nix files, which
      # belong to other formatters.
      excludes = [
        "*.nix"
        "*.go"
        "*.toml"
        "*.md"
      ];
    };
    shfmt-bats = {
      command = "${pkgs.shfmt}/bin/shfmt";
      options = [
        "--write"
        "--indent"
        "2"
        "--case-indent"
        "--binary-next-line"
        "--space-redirects"
        "--language-dialect"
        "bats"
      ];
      includes = [ "*.bats" ];
    };
  };

  # The config loader's fixtures are deliberately invalid or oddly written YAML
  # that must reach the loader byte for byte, so no formatter may touch them.
  settings.global.excludes = [
    "flake.lock"
    ".coverage/**"
    "internal/config/testdata/**"
  ];
}
