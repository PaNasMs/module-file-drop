#!/bin/sh
set -eu
npm ci
npm run build
mkdir -p dist/bin
go test -tags pam ./...
go test -race ./...
go vet -tags pam ./...
(cd third_party/qshare && go test ./... && go vet ./...)
go build -trimpath -tags pam -o dist/bin/server ./cmd/server
python3 scripts/check.py
python3 scripts/package.py
