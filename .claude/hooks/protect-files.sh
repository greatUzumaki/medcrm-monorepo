#!/usr/bin/env bash
# PreToolUse guard for Edit|Write|MultiEdit:
#  - block edits to secret files and already-applied migrations
#  - block writing content that looks like secrets / private keys
set -euo pipefail

input=$(cat)
path=$(jq -r '.tool_input.file_path // empty' <<<"$input")
[ -z "$path" ] && exit 0

case "$path" in
  *.env.example) ;;
  *.env|*.env.*|*.pem|*.key|*.p12|*.pfx)
    echo "Blocked: $path holds secrets. Edit .env.example and ask the user to update real secrets." >&2
    exit 2 ;;
  */apps/api/db/migrations/*.sql)
    if [ -f "$path" ]; then
      echo "Blocked: $path is an existing migration. Create a new one: make -C apps/api migration name=..." >&2
      exit 2
    fi ;;
esac

# Skip secret scan for docs/skills that describe patterns.
case "$path" in */.claude/*|*.md) exit 0 ;; esac

content=$(jq -r '[.tool_input.content, .tool_input.new_string, (.tool_input.edits // [] | .[].new_string)] | map(select(. != null)) | join("\n")' <<<"$input")
if grep -Eq -- '-----BEGIN [A-Z ]*PRIVATE KEY-----|AKIA[0-9A-Z]{16}|sk-[A-Za-z0-9_-]{32,}|ghp_[A-Za-z0-9]{36}|xox[baprs]-[A-Za-z0-9-]{10,}' <<<"$content"; then
  echo "Blocked: content looks like a secret/private key. Load it from env/secret store instead." >&2
  exit 2
fi
exit 0
