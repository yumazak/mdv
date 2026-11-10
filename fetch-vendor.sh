#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")"

MARKED_VERSION=18.2.0
MERMAID_VERSION=12.3.0
MARKED_SHA=392f4df89533b50f1dadff3f6dc0952350bd39c9f17def50a0374ef47d6147ef
MERMAID_SHA=336df803d87a38b70a01e736e666c8f7601d2ce42f5e47c8fafe5d721074b9a3

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

mkdir -p vendor

curl -fsSL "https://registry.npmjs.org/marked/-/marked-${MARKED_VERSION}.tgz" -o "$tmp/marked.tgz"
tar xzOf "$tmp/marked.tgz" package/lib/marked.umd.js > vendor/marked.umd.js

curl -fsSL "https://registry.npmjs.org/mermaid/-/mermaid-${MERMAID_VERSION}.tgz" -o "$tmp/mermaid.tgz"
tar xzOf "$tmp/mermaid.tgz" package/dist/mermaid.min.js > vendor/mermaid.min.js

check() {
  local expected="$1" file="$2"
  local actual
  if command -v sha256sum >/dev/null 2>&1; then
    actual="$(sha256sum "$file" | cut -d' ' -f1)"
  else
    actual="$(shasum -a 256 "$file" | cut -d' ' -f1)"
  fi
  if [ "$actual" != "$expected" ]; then
    echo "checksum mismatch: $file" >&2
    echo "  expected $expected" >&2
    echo "  actual   $actual" >&2
    exit 1
  fi
}

check "$MARKED_SHA" vendor/marked.umd.js
check "$MERMAID_SHA" vendor/mermaid.min.js

echo "vendor updated: marked ${MARKED_VERSION}, mermaid ${MERMAID_VERSION}"
