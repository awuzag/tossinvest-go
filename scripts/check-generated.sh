#!/usr/bin/env sh
set -eu

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

go run ./internal/codegen/tossopenapi -out "$tmpdir"

status=0
for name in catalog_gen.go types_gen.go client_gen.go; do
  expected="$tmpdir/$name"
  actual="internal/generated/tossapi/$name"
  if [ ! -f "$actual" ] || ! cmp -s "$expected" "$actual"; then
    echo "generated file is stale: $actual"
    status=1
  fi
done

exit "$status"
