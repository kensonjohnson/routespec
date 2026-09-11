#!/usr/bin/env bash
set -euo pipefail

tracked_go_files=$(git ls-files '*.go')
if [[ -n "$tracked_go_files" ]] && [[ -n "$(gofmt -l $tracked_go_files)" ]]; then
	echo "Go files need gofmt" >&2
	exit 1
fi

git diff --check
go test ./...
go test -race ./...
go vet ./...

UPDATE_GOLDEN=1 go test ./...
git diff --exit-code -- testdata/public-contract-openapi.json

release_version=${RELEASE_VERSION:-v0.1.0}
./scripts/verify-consumer.sh "$release_version" "$PWD"
