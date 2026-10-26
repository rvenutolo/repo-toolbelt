# The image: the runner as entrypoint, every lint and formatter, and the
# tools they call, with no base distro. A bash formatter's own tool arrives
# through its wrapper's closure in dist. Streamed rather than written to the
# store, so a build costs no image-sized store path; `./result | docker load`
# loads it.
{
  dockerTools,
  dist,
  version,
  bashNonInteractive,
  coreutils,
  findutils,
  gitMinimal,
  git-lfs,
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
    # A consumer's required LFS clean filter runs when a working-tree diff
    # or status meets a file whose stat data changed; git fails the command
    # when the filter's program is missing.
    git-lfs
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
      "TOOLBELT_FORMATTERS_DIR=/opt/toolbelt/formatters"
      # The LFS clean filter writes under lfs.storage, by default inside the
      # git dir, which the hooks mount read-only. Set in the environment
      # because that scope outranks a repo's own lfs.storage.
      "GIT_CONFIG_COUNT=1"
      "GIT_CONFIG_KEY_0=lfs.storage"
      "GIT_CONFIG_VALUE_0=/tmp/lfs"
    ];
  };
}
