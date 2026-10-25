#!/usr/bin/env bash

# The formatters this repo runs on itself, sourced by run-all-checks, the
# pre-commit hook and .ci/fmt so the three cannot drift apart. Each caller
# sources it by a path relative to its own resolved location. A new formatter
# that fits this repo is one more word in DOGFOOD_FORMATTERS.

# The formatter names, as `toolbelt fmt` takes them.
readonly DOGFOOD_FORMATTERS=(shell)

# Each formatter's check-mode lint, as `toolbelt run` takes it.
DOGFOOD_FORMATTER_CHECKS=()
for _dogfood_formatter in "${DOGFOOD_FORMATTERS[@]}"; do
  DOGFOOD_FORMATTER_CHECKS+=("check-${_dogfood_formatter}-formatted")
done
unset _dogfood_formatter
readonly DOGFOOD_FORMATTER_CHECKS
