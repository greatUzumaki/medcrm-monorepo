#!/usr/bin/env bash
# PostToolUse: format the edited file. Silently skips tools that are not installed yet.
path=$(jq -r '.tool_input.file_path // empty')
{ [ -z "$path" ] || [ ! -f "$path" ]; } && exit 0

case "$path" in
  */internal/api/*|*/internal/store/*|*/packages/contracts/gen/*) exit 0 ;;  # generated
  *.go)
    if command -v goimports >/dev/null; then goimports -w "$path"
    elif command -v gofmt >/dev/null; then gofmt -w "$path"; fi ;;
  *.ts|*.tsx|*.js|*.jsx|*.json|*.css|*.astro)
    biome="$CLAUDE_PROJECT_DIR/node_modules/.bin/biome"
    [ -x "$biome" ] && "$biome" check --write --no-errors-on-unmatched "$path" ;;
esac >/dev/null 2>&1
exit 0
