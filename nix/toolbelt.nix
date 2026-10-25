# The runner, every Go lint and every Go formatter, built from the one Go
# module. A Go formatter's package is cmd/fmt-<name>.
{
  lib,
  buildGoModule,
  version,
}:
let
  inherit (import ./manifest.nix) goLints goFormatters;
in
buildGoModule {
  pname = "repo-toolbelt";
  inherit version;

  # Only what the Go build reads, so the source hash ignores docs and bash.
  # The derivation still rebuilds on every commit of a development build,
  # whose version (0.0.0-<shortRev>) is linked in. lints.toml and formatters.toml
  # are embedded in the runner.
  src = lib.fileset.toSource {
    root = ../.;
    fileset = lib.fileset.unions [
      ../go.mod
      ../go.sum
      ../lints.go
      ../lints.toml
      ../formatters.toml
      ../cmd
      ../internal
    ];
  };

  vendorHash = "sha256-rToWHTNECWOQGODP/eZL0q+qFgxLeI2PKzUnaaBcA7o=";

  subPackages = [
    "cmd/toolbelt"
  ]
  ++ map (lint: "cmd/${lint.name}") goLints
  ++ map (formatter: "cmd/fmt-${formatter.name}") goFormatters;

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
    description = "repo-toolbelt runner, Go lints and Go formatters";
    mainProgram = "toolbelt";
  };
}
