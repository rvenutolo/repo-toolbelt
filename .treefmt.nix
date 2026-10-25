_: {
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

  # The config loader's fixtures are deliberately invalid or oddly written YAML
  # that must reach the loader byte for byte, so no formatter may touch them.
  settings.global.excludes = [
    "flake.lock"
    ".coverage/**"
    "internal/config/testdata/**"
  ];
}
