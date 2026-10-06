#!/usr/bin/env bash
#
# Regenerate api/** with the generator versions pinned in codegen.versions and
# verify that
#   1. the generated packages compile and their tests pass,
#   2. generation is idempotent (a second run produces identical files), and
#   3. the output matches the tracked digest baseline codegen.sha256.
#
# SPHERE_CODEGEN_SOURCE selects where the go-sphere protoc plugins come from:
#   release (default)  the versions pinned in codegen.versions
#   local              build ../protoc-gen-* checkouts (plugin development)
#   auto               local checkouts when all are present, else release
# The baseline describes the pinned releases, so a mismatch in local mode shows
# how unreleased plugin changes would alter this layout's output.

set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
ECOSYSTEM_DIR=$(cd "$ROOT_DIR/.." && pwd)
BIN_DIR=$(mktemp -d)
SOURCE_MODE=${SPHERE_CODEGEN_SOURCE:-release}
trap 'rm -rf "$BIN_DIR"' EXIT

# shellcheck source=../codegen.versions
source "$ROOT_DIR/codegen.versions"

export GOBIN="$BIN_DIR"
export PATH="$BIN_DIR:$PATH"

PLUGINS=(
	protoc-gen-sphere
	protoc-gen-sphere-binding
	protoc-gen-sphere-errors
	protoc-gen-route
)

build_local_plugins() {
	local plugin
	for plugin in "${PLUGINS[@]}"; do
		if [[ ! -f "$ECOSYSTEM_DIR/$plugin/go.mod" ]]; then
			return 1
		fi
	done

	for plugin in "${PLUGINS[@]}"; do
		(
			cd "$ECOSYSTEM_DIR/$plugin"
			go build -o "$BIN_DIR/$plugin" .
		)
	done
}

install_released_plugins() {
	go install "github.com/go-sphere/protoc-gen-sphere@$PROTOC_GEN_SPHERE_VERSION"
	go install "github.com/go-sphere/protoc-gen-sphere-binding@$PROTOC_GEN_SPHERE_BINDING_VERSION"
	go install "github.com/go-sphere/protoc-gen-sphere-errors@$PROTOC_GEN_SPHERE_ERRORS_VERSION"
	go install "github.com/go-sphere/protoc-gen-route@$PROTOC_GEN_ROUTE_VERSION"
}

case "$SOURCE_MODE" in
local)
	build_local_plugins
	;;
release)
	install_released_plugins
	;;
auto)
	if ! build_local_plugins; then
		SOURCE_MODE=release
		install_released_plugins
	fi
	;;
*)
	echo "unknown SPHERE_CODEGEN_SOURCE value: $SOURCE_MODE" >&2
	exit 1
	;;
esac

go install "google.golang.org/protobuf/cmd/protoc-gen-go@$PROTOC_GEN_GO_VERSION"
go install "github.com/bufbuild/buf/cmd/buf@$BUF_VERSION"

cd "$ROOT_DIR"

generate() {
	buf generate
	buf generate --template buf.binding.yaml
}

# Start from an empty api/ so files of deleted Proto definitions cannot linger.
rm -rf api
generate
go test ./api/...
first_digest=$(scripts/codegen-digest.sh print)

generate
second_digest=$(scripts/codegen-digest.sh print)

if [[ "$first_digest" != "$second_digest" ]]; then
	echo "code generation is not idempotent" >&2
	diff <(printf '%s\n' "$first_digest") <(printf '%s\n' "$second_digest") >&2 || true
	exit 1
fi

if ! scripts/codegen-digest.sh check; then
	if [[ "$SOURCE_MODE" != release ]]; then
		echo "note: plugins were built from local checkouts (SPHERE_CODEGEN_SOURCE=$SOURCE_MODE)," >&2
		echo "while $ROOT_DIR/codegen.sha256 records the releases pinned in codegen.versions." >&2
	fi
	exit 1
fi

echo "code generation verification passed (plugins: $SOURCE_MODE)"
