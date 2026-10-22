# The image: the runner as entrypoint, every lint, and the tools the lints
# call, with no base distro. Streamed rather than written to the store, so a
# build costs no image-sized store path; `./result | docker load` loads it.
{
  dockerTools,
  dist,
  version,
  bashNonInteractive,
  coreutils,
  findutils,
  gitMinimal,
  gnugrep,
  yq-go,
}:
dockerTools.streamLayeredImage {
  name = "ghcr.io/rvenutolo/repo-toolbelt";
  tag = version;

  contents = [
    dist
    # The bash the wrappers already run under, not bash-interactive.
    bashNonInteractive
    coreutils
    findutils
    # gitMinimal: no perl, python or docs, none of which a lint calls.
    gitMinimal
    gnugrep
    yq-go
    # /usr/bin/env, for `#!/usr/bin/env bash` shebangs.
    dockerTools.usrBinEnv
    # root and nobody only: the container runs as the caller's IDs, which
    # need no entry.
    dockerTools.fakeNss
  ];

  # Any --user may write here; the runner keeps its per-lint section files
  # in it, and HOME points at it.
  extraCommands = ''
    mkdir --mode=1777 tmp
  '';

  config = {
    Entrypoint = [ "/bin/toolbelt" ];
    Env = [
      "HOME=/tmp"
      "PATH=/bin"
      "TOOLBELT_LINTS_DIR=/opt/toolbelt/lints"
    ];
  };
}
