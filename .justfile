# Developer shortcuts. Each recipe wraps a command that stands alone; nothing
# requires `just`. Run `just` on its own to list them.

# List the available recipes.
default:
    @just --list

# Run the full gate: ./run-all-checks
check:
    ./run-all-checks

# Run the lint suite only: ./.ci/run-lint-checks
lint:
    ./.ci/run-lint-checks

# Run the Go tests and the BATS suite: ./run-tests
test:
    ./run-tests

# Format what nix fmt and the dogfooded formatters cover: ./.ci/fmt
fmt:
    ./.ci/fmt
