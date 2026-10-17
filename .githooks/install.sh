#!/bin/sh
set -eu
cd "$(git rev-parse --show-toplevel)"
configured=$(git config --get core.hooksPath || true)
if [ -n "$configured" ] && [ "$configured" != .githooks ]; then
    echo "Existing core.hooksPath=$configured; preserve it and integrate .githooks/pre-commit manually." >&2
    exit 1
fi
if [ -z "$configured" ] && [ -f "$(git rev-parse --git-path hooks)/pre-commit" ]; then
    echo 'An existing pre-commit hook is installed; preserve it and integrate .githooks/pre-commit manually.' >&2
    exit 1
fi
# Keep the hook choice local to this worktree, including when other worktrees exist.
git config extensions.worktreeConfig true
git config --worktree core.hooksPath .githooks
.tools/pre-commit/bin/pre-commit validate-config
echo 'Pre-commit installed for this worktree.'
