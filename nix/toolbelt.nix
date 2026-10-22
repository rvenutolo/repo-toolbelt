# The runner and every Go lint, built from the one Go module.
{
  lib,
  buildGoModule,
  version,
}:
let
  inherit (import ./manifest.nix) goLints;
in
buildGoModule {
  pname = "repo-toolbelt";
  inherit version;

  # Only what the Go build reads, so an edit to docs, tests or bash leaves
  # this derivation alone. lints.toml is embedded in the runner.
  src = lib.fileset.toSource {
    root = ../.;
    fileset = lib.fileset.unions [
      ../go.mod
      ../go.sum
      ../lints.go
      ../lints.toml
      ../cmd
      ../internal
    ];
  };

  vendorHash = "sha256-OfVFt+FPrV18g7Hs/EfU5PZnutRoHMSRlnR5YmbJX+Y=";

  subPackages = [ "cmd/toolbelt" ] ++ map (lint: "cmd/${lint.name}") goLints;

  # Static binaries: the image carries no libc for them to load.
  env.CGO_ENABLED = 0;

  ldflags = [
    "-s"
    "-w"
    "-X main.version=${version}"
  ];

  # run-tests runs the Go tests with -race, which needs cgo; repeating a
  # cgo-less subset here would only slow the image build.
  doCheck = false;

  meta = {
    description = "repo-toolbelt runner and Go lints";
    mainProgram = "toolbelt";
  };
}
