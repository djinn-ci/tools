#!/bin/sh

set -e

argv0="$0"

usage() {
	>&2 printf "usage: %s <driver config> <image group>\n" "$argv0"
	exit 1
}

if [ -z "$DJINN_IMAGE_DIR" ]; then
	>&2 printf "%s: DJINN_IMAGE_DIR not set\n" "$argv0"
	exit 1
fi

[ ! $# -eq 2 ] && usage

arch="x86_64"
conf="$1"
group="$2"

[ ! -d artifacts ] && mkdir artifacts

[ -d manifests/"$group" ] && mkdir -p artifacts/"$group"

for f in $(find manifests -type f | grep "$group"); do
	djinn -artifacts artifacts -objects scripts -driver "$conf" -manifest "$f"
done

for img in $(find artifacts -type f | grep "$group"); do
	name="$(echo -n "$img" | sed 's|artifacts/||')"
	dst="$DJINN_IMAGE_DIR"/qemu/"$arch"/"$name"

	mv "$img" "$dst"
done
