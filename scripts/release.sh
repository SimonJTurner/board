#!/usr/bin/env bash
set -euo pipefail

if [ -n "$(git status --porcelain)" ]; then
  echo "working tree not clean, commit changes first" >&2
  exit 1
fi

git fetch --tags
latest=$(git tag --list 'v[0-9]*.[0-9]*.[0-9]*' | sort -V | tail -n1)

mode=${1:-patch}
if [[ ! $mode =~ ^(major|minor|patch)$ ]]; then
  echo "mode must be major, minor, or patch" >&2
  exit 1
fi

parse() {
  IFS='.' read -r maj min pat <<< "$1"
  echo "$maj" "$min" "$pat"
}

if [ -z "$latest" ]; then
  base_major=0
  base_minor=0
  base_patch=-1
else
  tagsuf=${latest#v}
  read -r base_major base_minor base_patch <<< "$(parse "$tagsuf")"
fi
major=${MAJOR-}
minor=${MINOR-}
patch=${PATCH-}
case "$mode" in
major)
  major=$((base_major + 1))
  minor=0
  patch=0
  ;;
minor)
  major=$base_major
  minor=$((base_minor + 1))
  patch=0
  ;;
patch)
  major=$base_major
  minor=$base_minor
  patch=$((base_patch + 1))
  ;;
esac

if [ -n "$MAJOR" ]; then
  major=$MAJOR
fi
if [ -n "$MINOR" ]; then
  minor=$MINOR
fi
if [ -n "$PATCH" ]; then
  patch=$PATCH
fi
new_tag="v${major}.${minor}.${patch}"
if git rev-parse "$new_tag" >/dev/null 2>&1; then
  echo "tag $new_tag already exists" >&2
  exit 1
fi
git tag -a "$new_tag" -m "Release $new_tag"
git push origin "$new_tag"
echo "created release tag $new_tag"
