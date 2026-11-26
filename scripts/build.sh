#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

VERSION="${VERSION:-1.0.0}"
LDFLAGS="-s -w"

echo "==> 构建服务端"
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
  go build -trimpath -ldflags="$LDFLAGS" -o "modernfs-linux-amd64" .

echo "==> 构建配置向导 (Windows)"
cd cmd/config-writer
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 \
  go build -trimpath -ldflags="$LDFLAGS" -o "config-writer.exe" .
cd ../..

echo "==> 完成"
ls -lh modernfs-linux-amd64 cmd/config-writer/config-writer.exe

echo "提示: 发布时可重命名为 modernfs-v${VERSION}-linux-amd64"
