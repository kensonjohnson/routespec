#!/usr/bin/env bash
set -euo pipefail

version=${1:?usage: scripts/verify-consumer.sh vX.Y.Z [local-source]}
source_dir=${2:-}
case "$version" in
	v*) ;;
	*)
		echo "version must begin with v" >&2
		exit 2
		;;
esac

workdir=$(mktemp -d)
trap 'rm -rf "$workdir"' EXIT
cd "$workdir"

go mod init example.com/routespec-consumer
go mod edit -go=1.27
if [[ -n "$source_dir" ]]; then
	source_dir=$(cd "$source_dir" && pwd)
	go mod edit -replace "github.com/kensonjohnson/routespec=${source_dir}"
	GOPROXY=off go get "github.com/kensonjohnson/routespec@${version}"
else
	GOPROXY=direct go get "github.com/kensonjohnson/routespec@${version}"
fi

cat > main.go <<'EOF'
package main

import (
	"net/http"

	"github.com/kensonjohnson/routespec"
)

func main() {
	routespec.New(http.NewServeMux(), routespec.Info{Title: "Consumer", Version: "1.0.0"})
}
EOF

go build .
