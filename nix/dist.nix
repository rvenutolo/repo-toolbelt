# The /opt/toolbelt layout the runner expects, the same one .ci/build-dist
# builds for local runs: the runner at opt/toolbelt/bin/toolbelt (and
# bin/toolbelt), each lint at opt/toolbelt/lints/<name>/<name>, the bash
# library at opt/toolbelt/lib. build-dist cannot run here: it finds the repo
# through git, and the build sandbox has no .git.
{
  lib,
  runCommand,
  makeWrapper,
  toolbelt,
  bashNonInteractive,
  coreutils,
  findutils,
  gitMinimal,
  gnugrep,
  yq-go,
}:
let
  inherit (import ./manifest.nix) goLints bashLints;

  # What a bash lint and the library call. Prefixed onto the lint's PATH so it
  # runs the same in the image and anywhere else this package is used.
  bashLintPath = lib.makeBinPath [
    bashNonInteractive
    coreutils
    findutils
    gitMinimal
    gnugrep
    yq-go
  ];

  copyGoLint = lint: ''
    install -D --mode=0755 ${toolbelt}/bin/${lint.name} "$opt/lints/${lint.name}/${lint.name}"
  '';

  # The shebang names the non-interactive bash that makeWrapper's wrapper also
  # runs under; nixpkgs' `bash` is bash-interactive, and naming it would put a
  # second bash, with readline and ncurses, in the image.
  # The wrapper takes the lint's own name, and the script moves beside it, so
  # the lint still finds ../../lib/toolbelt.bash from its resolved path.
  copyBashLint = lint: ''
    cp --recursive ${../lints + "/${lint.name}"} "$opt/lints/${lint.name}"
    chmod --recursive u+w "$opt/lints/${lint.name}"
    substituteInPlace "$opt/lints/${lint.name}/${lint.name}" \
      --replace-fail '#!/usr/bin/env bash' '#!${lib.getExe bashNonInteractive}'
    wrapProgram "$opt/lints/${lint.name}/${lint.name}" --prefix PATH : ${bashLintPath}
  '';
in
runCommand "repo-toolbelt-dist-${toolbelt.version}"
  {
    nativeBuildInputs = [ makeWrapper ];
  }
  ''
    opt="$out/opt/toolbelt"
    install -D --mode=0755 ${toolbelt}/bin/toolbelt "$opt/bin/toolbelt"
    mkdir --parents "$out/bin" "$opt/lints"
    ln --symbolic ../opt/toolbelt/bin/toolbelt "$out/bin/toolbelt"
    cp --recursive ${../lib} "$opt/lib"
    ${lib.concatMapStrings copyGoLint goLints}
    ${lib.concatMapStrings copyBashLint bashLints}
  ''
