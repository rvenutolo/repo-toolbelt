# The tools each bash formatter runs, by formatter name: the Nix packages
# whose bin/ the formatter's wrapper puts on its PATH, after the library's
# tools. This is the one place a formatter's tool is chosen; formatters.toml
# names no package. Every bash formatter in formatters.toml needs an entry,
# an empty list when it calls nothing beyond the library's tools; nix/dist.nix
# fails evaluation on a missing one.
pkgs: {
  shell = [ pkgs.shfmt ];
}
