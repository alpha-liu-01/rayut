#!/bin/bash
# Compile rayutd and mihomo into the click install directory.
# Clickable runs this inside its build container. It does not download either program.
set -euo pipefail

install_dir=$1
root=$2

export PATH="/usr/local/go/bin:${PATH}"
export CGO_ENABLED=0
export GOOS=linux
export GOARCH=arm64
export GOTOOLCHAIN=local
export GOFLAGS=-mod=readonly
export GOMODCACHE="${root}/build/gomodcache"
export GOCACHE="${root}/build/gocache"
mkdir -p "${install_dir}/bin" "${GOMODCACHE}" "${GOCACHE}"

cd "${root}/daemon"
go build -trimpath -ldflags '-w -s -buildid=' -o "${install_dir}/bin/rayutd" ./cmd/rayutd

cd "${root}/third_party/mihomo"
go build -tags with_gvisor -trimpath \
    -ldflags '-X github.com/metacubex/mihomo/constant.Version=v1.19.32 -w -s -buildid=' \
    -o "${install_dir}/bin/mihomo" .

cd "${root}/third_party/geodata"
sha256sum -c SHA256SUMS
