#!/usr/bin/env sh
set -eu

bump="${1:-patch}"

case "$bump" in
major | minor | patch) ;;
*)
	echo "usage: task deploy:tag [BUMP=major|minor|patch]" >&2
	exit 2
	;;
esac

if ! git diff --quiet || ! git diff --cached --quiet; then
	echo "working tree is not clean; commit or stash changes before tagging" >&2
	exit 1
fi

git fetch --tags origin

latest=""
for tag in $(git tag --list 'v*.*.*' --sort=-v:refname); do
	candidate="${tag#v}"
	old_ifs="$IFS"
	IFS=.
	set -- $candidate
	IFS="$old_ifs"
	if [ "$#" -ne 3 ]; then
		continue
	fi
	case "$1" in "" | *[!0-9]*)
		continue
		;;
	esac
	case "$2" in "" | *[!0-9]*)
		continue
		;;
	esac
	case "$3" in "" | *[!0-9]*)
		continue
		;;
	esac
	latest="$tag"
	break
done
if [ -z "$latest" ]; then
	latest="v0.0.0"
fi

version="${latest#v}"
old_ifs="$IFS"
IFS=.
set -- $version
IFS="$old_ifs"

major="${1:-0}"
minor="${2:-0}"
patch="${3:-0}"

case "$major$minor$patch" in
*[!0-9]*)
	echo "latest tag is not a numeric semver tag: $latest" >&2
	exit 1
	;;
esac

case "$bump" in
major)
	major=$((major + 1))
	minor=0
	patch=0
	;;
minor)
	minor=$((minor + 1))
	patch=0
	;;
patch)
	patch=$((patch + 1))
	;;
esac

next="v${major}.${minor}.${patch}"

if git rev-parse -q --verify "refs/tags/${next}" >/dev/null; then
	echo "tag already exists: $next" >&2
	exit 1
fi

git tag -a "$next" -m "Release $next"
git push origin "$next"

echo "$next"
