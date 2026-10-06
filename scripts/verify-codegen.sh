#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
ECOSYSTEM_DIR=$(cd "$ROOT_DIR/.." && pwd)
BIN_DIR=$(mktemp -d)
SOURCE_MODE=${SPHERE_CODEGEN_SOURCE:-auto}
trap 'rm -rf "$BIN_DIR"' EXIT

# shellcheck source=../codegen.versions
source "$ROOT_DIR/codegen.versions"

export GOBIN="$BIN_DIR"
export PATH="$BIN_DIR:$PATH"

build_local_plugins() {
	local plugin
	for plugin in \
		protoc-gen-sphere \
		protoc-gen-sphere-binding \
		protoc-gen-sphere-errors \
		protoc-gen-route; do
		if [[ ! -f "$ECOSYSTEM_DIR/$plugin/go.mod" ]]; then
			return 1
		fi
	done

	for plugin in \
		protoc-gen-sphere \
		protoc-gen-sphere-binding \
		protoc-gen-sphere-errors \
		protoc-gen-route; do
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

buf generate
buf generate --template buf.binding.yaml
git diff --exit-code -- api
go test ./api/...

first_digest=$(
	find api -type f -name '*.go' |
		LC_ALL=C sort |
		while IFS= read -r file; do
			printf '%s %s\n' "$(git hash-object "$file")" "$file"
		done
)

buf generate
buf generate --template buf.binding.yaml

second_digest=$(
	find api -type f -name '*.go' |
		LC_ALL=C sort |
		while IFS= read -r file; do
			printf '%s %s\n' "$(git hash-object "$file")" "$file"
		done
)

if [[ "$first_digest" != "$second_digest" ]]; then
	echo "code generation is not idempotent" >&2
	diff <(printf '%s\n' "$first_digest") <(printf '%s\n' "$second_digest") >&2 || true
	exit 1
fi

git diff --exit-code -- api
echo "cross-plugin code generation verification passed"
