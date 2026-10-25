{
  description = "repo-toolbelt - shared lints and formatters for rvenutolo's repos, shipped as a Docker image";

  inputs = {
    # Keep this URL in github:NixOS/nixpkgs/... form. Renovate's
    # lockFileMaintenance refreshes flake.lock only when that exact string
    # appears here; any other spelling leaves the lock frozen with no error.
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    systems.url = "github:nix-systems/default";
    treefmt-nix.url = "github:numtide/treefmt-nix";
    treefmt-nix.inputs.nixpkgs.follows = "nixpkgs";
    bats-support = {
      url = "github:bats-core/bats-support";
      flake = false;
    };
    bats-assert = {
      url = "github:bats-core/bats-assert";
      flake = false;
    };
  };

  outputs =
    {
      self,
      nixpkgs,
      systems,
      treefmt-nix,
      bats-support,
      bats-assert,
    }:
    let
      eachSystem = f: nixpkgs.lib.genAttrs (import systems) (system: f nixpkgs.legacyPackages.${system});
      treefmtEval = eachSystem (pkgs: treefmt-nix.lib.evalModule pkgs ./.treefmt.nix);
      # nixpkgs' unstable-version convention, derived from the input's own
      # lastModifiedDate so a flake.lock bump relabels the store path too.
      unstableVersion =
        input:
        let
          d = input.lastModifiedDate;
        in
        "0-unstable-${builtins.substring 0 4 d}-${builtins.substring 4 2 d}-${builtins.substring 6 2 d}";
      # The version a development build reports and its image is tagged
      # with: the commit it was built from, so two builds of different trees
      # never share a tag.
      devVersion = "0.0.0-${self.shortRev or self.dirtyShortRev or "unknown"}";
      # The version of a release build: the one CHANGELOG.md's first `## `
      # heading names, without its `v`, read the way .ci/release-version's
      # `top` reads it. Pure evaluation sees no tags, so whether this commit
      # is a release is not decided here: the release workflow builds
      # release-image only on the commit `.ci/release-publish pending` names,
      # where that heading is the version being published. A changelog with
      # no release heading falls back to devVersion, so evaluating the flake
      # never fails on it.
      releaseVersion = import ./nix/release-version.nix {
        changelog = builtins.readFile ./CHANGELOG.md;
        script = builtins.readFile ./.ci/release-version;
        fallback = devVersion;
      };
      # The runner, the dist layout and the image, all reporting one version.
      imageFor =
        pkgs: version:
        let
          toolbelt = pkgs.callPackage ./nix/toolbelt.nix { inherit version; };
          dist = pkgs.callPackage ./nix/dist.nix { inherit toolbelt; };
        in
        {
          inherit toolbelt dist;
          image = pkgs.callPackage ./nix/image.nix { inherit dist version; };
        };
    in
    {
      # x86_64-linux only: the image is linux/amd64, and every CI runner and
      # supported local run is x86_64 Linux.
      packages.x86_64-linux =
        let
          pkgs = nixpkgs.legacyPackages.x86_64-linux;
        in
        imageFor pkgs devVersion
        // {
          # What the release workflow pushes to GHCR; see releaseVersion. Its
          # version is the release's only on a commit `.ci/release-publish
          # pending` approved; on any other commit the first changelog heading
          # names a release that is already out, or none.
          release-image = (imageFor pkgs releaseVersion).image;
        };

      formatter = eachSystem (pkgs: treefmtEval.${pkgs.stdenv.hostPlatform.system}.config.build.wrapper);

      checks = eachSystem (pkgs: {
        formatting = treefmtEval.${pkgs.stdenv.hostPlatform.system}.config.build.check self;
      });

      devShells = eachSystem (
        pkgs:
        let
          # kcov's xtrace parser tracks single-quote state across lines, but only
          # honours backslash escapes outside a quote. Bash renders any value
          # containing a newline with ANSI-C quoting, where \' is an escaped
          # quote, so one such trace line leaves the parser latched in
          # INPUT_SINGLE_QUOTE and every later line is silently discarded: an
          # empty report, no error, exit 0. bats' own `run` traces its saved
          # traps that way, so without the patch every hit after the suite's first
          # `run` is lost and every measured file scores 0%.
          #
          # kcov's bash engine also sets PS4='kcov@${BASH_SOURCE}@${LINENO}@'
          # with no default expansion, so a `bash -c '… set -u …'` string, which
          # has no BASH_SOURCE, dies inside PS4 expansion and the test around it
          # fails under kcov alone. The `:-` guard is the whole fix;
          # --replace-fail makes a kcov bump that moves the line fail the build
          # loudly instead of silently un-patching. The escaped `\$` is for
          # bash, so substituteInPlace receives the literal `${BASH_SOURCE}`.
          kcovPatched = pkgs.kcov.overrideAttrs (prev: {
            patches = (prev.patches or [ ]) ++ [ ./.nix/kcov-ansi-c-quoting.patch ];
            postPatch = (prev.postPatch or "") + ''
              substituteInPlace src/engines/bash-helper.sh src/engines/bash-helper-debug-trap.sh \
                --replace-fail "\''${BASH_SOURCE}" "\''${BASH_SOURCE:-}"
            '';
          });
        in
        {
          default = pkgs.mkShellNoCC {
            packages =
              with pkgs;
              [
                # linters
                shellcheck
                shfmt
                actionlint
                # zizmor audits workflows for security where actionlint audits
                # them for syntax; they overlap nowhere.
                zizmor
                yamllint
                markdownlint-cli2
                typos
                # nix linters: statix for antipatterns, deadnix for unused bindings.
                statix
                deadnix
                # runtime
                yq-go
                # Reads govulncheck's JSON report in .ci/vuln.
                jq
                git
                just
                commitlint
                # nix.out, not bare `nix`: the default output is `dev`, which has
                # no bin/, so a bare `nix` puts nothing on PATH.
                nix.out
                coreutils
                findutils
                gnugrep
                # Go toolchain and its lint gate.
                go
                # `go test -race` needs cgo, and so a C compiler.
                gcc
                golangci-lint
                # Bash coverage for the BATS suite, patched as described above.
                kcovPatched
                # Image build and release tooling. The docker daemon is the
                # host's; only the client lives here.
                docker-client
                # sysctl, for .ci/test-image's IP-forwarding check before it
                # starts any container.
                procps
                skopeo
                git-cliff
                gh
                # withLibraries, not bare bats: the wrapper exports BATS_LIB_PATH
                # at its own share/bats, so test_helper/common.bash loads the
                # libraries with `bats_load_library`, and a bats that is not this
                # one fails loudly instead of silently missing them. Both libraries
                # come from flake inputs pinned in flake.lock: the nixpkgs
                # bats-assert release lacks assert_stderr.
                (bats.withLibraries (l: [
                  (l.bats-support.overrideAttrs (_: {
                    src = bats-support;
                    version = unstableVersion bats-support;
                  }))
                  (l.bats-assert.overrideAttrs (_: {
                    src = bats-assert;
                    version = unstableVersion bats-assert;
                  }))
                ]))
              ]
              # Every formatter's tools, from the one list the image reads too,
              # so the devShell cannot drift from it: .ci/build-dist finds the
              # tools here for local runs.
              ++ pkgs.lib.concatLists (pkgs.lib.attrValues (import ./nix/formatter-tools.nix pkgs));

            # Activate the tracked git hooks for this clone. Tolerant of failure
            # on purpose: a devShell that aborts on hook setup is worse than
            # inert hooks. Resolved via git rather than the flake's store path so
            # it configures the working clone, not a read-only copy.
            shellHook = ''
              if hooks_repo_root="$(git rev-parse --show-toplevel 2>/dev/null)"; then
                "$hooks_repo_root/.ci/activate-githooks" \
                  || echo 'warning: could not activate tracked git hooks' >&2
              fi
            '';
          };
        }
      );
    };
}
