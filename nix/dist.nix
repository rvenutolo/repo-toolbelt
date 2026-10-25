# The /opt/toolbelt layout the runner expects, the same one .ci/build-dist
# builds for local runs: the runner at opt/toolbelt/bin/toolbelt (and
# bin/toolbelt), each lint at opt/toolbelt/lints/<name>/<name>, each
# formatter at opt/toolbelt/formatters/<name>/<name>, the bash library at
# opt/toolbelt/lib. lints/ and formatters/ exist even when empty.
# build-dist cannot run here: it finds the repo through git, and the build
# sandbox has no .git.
{
  lib,
  pkgs,
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
  inherit (import ./manifest.nix)
    goLints
    bashLints
    goFormatters
    bashFormatters
    ;

  formatterTools = import ./formatter-tools.nix pkgs;

  # What a bash lint, a bash formatter and the library call. Prefixed onto
  # the executable's PATH so it runs the same in the image and anywhere else
  # this package is used.
  bashLibraryTools = [
    bashNonInteractive
    coreutils
    findutils
    gitMinimal
    gnugrep
    yq-go
  ];

  # A bash formatter's PATH adds the tool it formats with, which reaches the
  # image through the wrapper's closure.
  formatterPath =
    name:
    lib.makeBinPath (
      bashLibraryTools
      ++ formatterTools.${name} or (throw "nix/formatter-tools.nix: no entry for formatter ${name}")
    );

  # A Go executable, built by the toolbelt package under its cmd/ name, at
  # <dir>/<name>/<name>.
  copyGo = dir: binary: name: ''
    install -D --mode=0755 ${toolbelt}/bin/${binary} "$opt/${dir}/${name}/${name}"
  '';

  # The shebang names the non-interactive bash that makeWrapper's wrapper also
  # runs under; nixpkgs' `bash` is bash-interactive, and naming it would put a
  # second bash, with readline and ncurses, in the image.
  # The wrapper takes the executable's own name, and the script moves beside
  # it, so the script still finds ../../lib/toolbelt.bash from its resolved
  # path.
  copyBash = dir: name: path: ''
    cp --recursive ${../. + "/${dir}/${name}"} "$opt/${dir}/${name}"
    chmod --recursive u+w "$opt/${dir}/${name}"
    substituteInPlace "$opt/${dir}/${name}/${name}" \
      --replace-fail '#!/usr/bin/env bash' '#!${lib.getExe bashNonInteractive}'
    wrapProgram "$opt/${dir}/${name}/${name}" --prefix PATH : ${path}
  '';

  copyGoLint = lint: copyGo "lints" lint.name lint.name;
  copyBashLint = lint: copyBash "lints" lint.name (lib.makeBinPath bashLibraryTools);
  copyGoFormatter = formatter: copyGo "formatters" "fmt-${formatter.name}" formatter.name;
  copyBashFormatter = formatter: copyBash "formatters" formatter.name (formatterPath formatter.name);
in
runCommand "repo-toolbelt-dist-${toolbelt.version}"
  {
    nativeBuildInputs = [ makeWrapper ];
  }
  ''
    opt="$out/opt/toolbelt"
    install -D --mode=0755 ${toolbelt}/bin/toolbelt "$opt/bin/toolbelt"
    mkdir --parents "$out/bin" "$opt/lints" "$opt/formatters"
    ln --symbolic ../opt/toolbelt/bin/toolbelt "$out/bin/toolbelt"
    cp --recursive ${../lib} "$opt/lib"
    ${lib.concatMapStrings copyGoLint goLints}
    ${lib.concatMapStrings copyBashLint bashLints}
    ${lib.concatMapStrings copyGoFormatter goFormatters}
    ${lib.concatMapStrings copyBashFormatter bashFormatters}
  ''
