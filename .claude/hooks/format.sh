#!/usr/bin/env bash
#
# Formats the file an edit just wrote, so nothing reaches `make lint` badly
# spaced: Go through gofmt, the web workspace's files (both apps and the
# shared styles, .scss included) through its Prettier.
#
# Claude Code runs this as a PostToolUse hook and hands it the tool call as
# JSON on stdin. It never fails a tool call: a file it does not know how to
# format, or a formatter that is missing, is simply left alone.

set -u

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

file="$(jq -r '.tool_response.filePath // .tool_input.file_path // empty')"
[[ -n $file && -f $file ]] || exit 0

# Only ever files of this repository.
case "$file" in
"$root"/*) ;;
*) exit 0 ;;
esac

case "$file" in
*.go)
	command -v gofmt >/dev/null && gofmt -w "$file"
	;;
"$root"/web/*.svelte | "$root"/web/*.ts | "$root"/web/*.js | "$root"/web/*.css | "$root"/web/*.scss | "$root"/web/*.json | "$root"/web/*.md)
	# Prettier is installed once for the workspace (web/node_modules), with
	# its one configuration in web/prettier.config.js.
	if [[ -d $root/web/node_modules ]]; then
		(cd "$root/web" && bunx prettier --write --ignore-unknown "$file" >/dev/null 2>&1)
	fi
	;;
esac

exit 0
