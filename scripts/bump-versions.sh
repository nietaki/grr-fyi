#!/usr/bin/env bash
set -euo pipefail

bump_patch() {
	local file="$1"
	local version
	version=$(<"$file")
	IFS='.' read -r major minor patch <<< "$version"
	patch=$((patch + 1))
	echo "${major}.${minor}.${patch}" > "$file"
	echo "$file: ${version} -> ${major}.${minor}.${patch}"
}

bump_patch "APP_VERSION.txt"
bump_patch "CHART_VERSION.txt"
