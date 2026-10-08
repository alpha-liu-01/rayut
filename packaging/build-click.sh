#!/bin/bash
# Build the arm64 click from this repository.
# rayutd comes from daemon/. mihomo comes from the URL and SHA-256 in the core catalog.
set -euo pipefail

root=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
catalog="$root/daemon/internal/core/catalog.go"
core_dir="$root/app/packaging/core"

if ! command -v go >/dev/null 2>&1; then
    echo "go is required on PATH" >&2
    exit 1
fi
if ! command -v clickable >/dev/null 2>&1; then
    echo "clickable is required on PATH" >&2
    exit 1
fi

url=$(awk -F'"' '/^[[:space:]]*URL:/ { print $2; exit }' "$catalog")
sha=$(awk -F'"' '/^[[:space:]]*SHA256:/ { print $2; exit }' "$catalog")
case "$url" in
    https://github.com/MetaCubeX/mihomo/releases/download/*/mihomo-linux-arm64-*.gz) ;;
    *)
        echo "catalog URL is not the pinned linux-arm64 archive: $url" >&2
        exit 1
        ;;
esac
if ! printf '%s\n' "$sha" | grep -Eq '^[0-9a-f]{64}$'; then
    echo "catalog SHA-256 is missing" >&2
    exit 1
fi

mkdir -p "$core_dir"
(
    cd "$root/daemon"
    CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -o "$core_dir/rayutd" ./cmd/rayutd
)

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT
curl -fsSL -o "$work/mihomo.gz" "$url"
echo "$sha  $work/mihomo.gz" | sha256sum -c -
gunzip -c "$work/mihomo.gz" > "$core_dir/mihomo"
chmod 755 "$core_dir/mihomo"
python3 - "$core_dir/mihomo" <<'PY'
import struct, sys
data = open(sys.argv[1], "rb").read(20)
if data[:4] != b"\x7fELF" or data[4] != 2:
    sys.exit("mihomo is not a 64-bit ELF")
machine = struct.unpack_from("<H", data, 18)[0]
if machine != 183:
    sys.exit("mihomo is not aarch64")
PY

cd "$root/app"
clickable build --arch arm64 --accept-review-errors
