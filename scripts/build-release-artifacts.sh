#!/usr/bin/env bash
set -euo pipefail

version="${1:-${VERSION:-dev}}"
dist_dir="${2:-${DIST_DIR:-dist}}"

if [[ -z "${version}" ]]; then
	echo "version is required" >&2
	exit 2
fi

version="${version#v}"
rm -rf "${dist_dir}"
mkdir -p "${dist_dir}"

targets=(
	"linux amd64"
	"linux arm64"
	"darwin amd64"
	"darwin arm64"
	"windows amd64"
	"windows arm64"
)

for target in "${targets[@]}"; do
	read -r goos goarch <<< "${target}"
	name="tossinvest_${version}_${goos}_${goarch}"
	binary="tossinvest"
	if [[ "${goos}" == "windows" ]]; then
		binary="tossinvest.exe"
	fi

	mkdir -p "${dist_dir}/${name}"
	CGO_ENABLED=0 GOOS="${goos}" GOARCH="${goarch}" \
		go build -trimpath -ldflags="-s -w" -o "${dist_dir}/${name}/${binary}" ./cmd/tossinvest
	cp README.md "${dist_dir}/${name}/README.md"
	tar -czf "${dist_dir}/${name}.tar.gz" -C "${dist_dir}/${name}" .
	rm -rf "${dist_dir}/${name}"
done

(cd "${dist_dir}" && shasum -a 256 *.tar.gz > checksums.txt)
