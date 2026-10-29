#!/usr/bin/env bash
# Generate shell completion scripts into ./completions for inclusion in release
# archives. Run from the repo root (GoReleaser's before.hooks does this).
set -euo pipefail

outdir="completions"
rm -rf "$outdir"
mkdir -p "$outdir"

for shell in bash zsh fish; do
  go run ./cmd/flint completion "$shell" >"$outdir/flint.$shell"
done

echo "wrote completions to $outdir/"
