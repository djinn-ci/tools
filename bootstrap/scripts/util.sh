#!/bin/sh

nbd_dev="/dev/nbd0"

blk_uuid() {
	blkid -o export "$1" | grep -E ^UUID | cut -d = -f 2
}

mount_bind() {
	src="$1"
	dst="$2"

	mkdir -p "$dst"

	mount --bind "$src" "$dst"
	mount --make-private "$dst"
}

prepare_chroot() {
	dir="$1"

	mkdir -p "$dir"/proc
	mount -t proc none "$dir"/proc

	mount_bind /dev "$dir"/dev
	mount_bind /sys "$dir"/sys
}

prepare_nbd() {
	qemu-img create -f qcow2 "$1" 10G

	modprobe nbd
	qemu-nbd -c "$nbd_dev" -f qcow2 "$1"
}

cleanup_nbd() {
	grep /mnt /proc/mounts | cut -d ' ' -f 2 | sort -r | xargs umount || true

	qemu-nbd -d "$nbd_dev"
	modprobe -r nbd || true
}
