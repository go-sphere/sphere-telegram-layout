#!/usr/bin/env bash
#
# Digest baseline for generated code.
#
# Generated output (api/**) is gitignored, so `git diff` cannot detect when a
# generator change alters it. Instead the repository tracks codegen.sha256: one
# "<sha256>  <path>" line per generated file, paths relative to the repository
# root, sorted bytewise (LC_ALL=C). Digests are taken over the raw file bytes.
#
# Usage:
#   scripts/codegen-digest.sh print    write the current digest to stdout
#   scripts/codegen-digest.sh check    compare the current digest with the baseline
#   scripts/codegen-digest.sh update   overwrite the baseline with the current digest
#
# Generate first (`make gen/all`) with the tools pinned in codegen.versions.

set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
BASELINE=codegen.sha256
GENERATED_DIRS=(api)

cd "$ROOT_DIR"

sha256() {
	if command -v sha256sum >/dev/null 2>&1; then
		sha256sum "$@"
	else
		shasum -a 256 "$@"
	fi
}

print_digest() {
	local dir
	for dir in "${GENERATED_DIRS[@]}"; do
		if [[ ! -d "$dir" ]] || [[ -z "$(find "$dir" -type f -print -quit)" ]]; then
			echo "no generated files under $dir/; run \`make gen/all\` first" >&2
			exit 1
		fi
	done
	find "${GENERATED_DIRS[@]}" -type f | LC_ALL=C sort | while IFS= read -r file; do
		sha256 "$file"
	done
}

case "${1:-}" in
print)
	print_digest
	;;
update)
	current=$(print_digest)
	printf '%s\n' "$current" >"$BASELINE"
	echo "wrote $BASELINE ($(wc -l <"$BASELINE" | tr -d ' ') files)"
	;;
check)
	current=$(print_digest)
	if [[ ! -f "$BASELINE" ]]; then
		echo "missing $BASELINE; run \`make codegen-baseline\` and commit it" >&2
		exit 1
	fi
	if [[ "$current" == "$(cat "$BASELINE")" ]]; then
		echo "generated code matches $BASELINE"
		exit 0
	fi
	{
		echo "generated code differs from the tracked baseline $BASELINE:"
		diff -u --label "$BASELINE (tracked)" --label "generated (current)" \
			"$BASELINE" <(printf '%s\n' "$current") || true
		echo
		echo "If the change is intended (Proto edits, or a generator bump in"
		echo "codegen.versions), regenerate with the pinned tools and refresh the"
		echo "baseline, then commit $BASELINE together with the change:"
		echo
		echo "    make install && make gen/all && make codegen-baseline"
		echo
		echo "Otherwise a generator produced unexpected output: check that the"
		echo "installed tools match codegen.versions (\`go version -m \$(which <tool>)\`)."
	} >&2
	exit 1
	;;
*)
	echo "usage: $0 print|check|update" >&2
	exit 2
	;;
esac
