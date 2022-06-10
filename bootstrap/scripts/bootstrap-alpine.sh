#!/bin/sh

set -ex

argv0="$(basename "$0")"

[ "$(whoami)" != "root" ] && {
	>&2 printf "%s: must be run as root\n" "$argv0"
	exit 1
}

. ./util.sh

_apk() {
	apk --no-progress "$@"
}

_apk add e2fsprogs qemu-img

prepare_nbd alpine

trap cleanup_nbd EXIT

mkfs.ext4 -L root -O ^64bit -E nodiscard "$nbd_dev"

mount "$nbd_dev" /mnt

cd /mnt && {
	mkdir -p etc/apk/keys

	cp /etc/apk/repositories etc/apk
	cp /etc/apk/keys/* etc/apk/keys

	_apk add --root . --update-cache --initdb alpine-base
	prepare_chroot .

	_apk add --root . mkinitfs
	cp /etc/mkinitfs/mkinitfs.conf etc/mkinitfs/mkinitfs.conf

	_apk add --root . linux-lts
	_apk add --root . --no-scripts syslinux

	replace="$(grep -E ^root etc/update-extlinux.conf)"
	root_uuid="$(blk_uuid "$nbd_dev")"

	sed -i "s/$replace/root=UUID=$(blk_uuid "$nbd_dev")/" etc/update-extlinux.conf

	chroot . extlinux --install /boot
	chroot . update-extlinux

	sed -i "s/DEFAULT menu.c32/DEFAULT lts/" boot/extlinux.conf
	sed -i "s/TIMEOUT 30/TIMEOUT 0/" boot/extlinux.conf

	cat > etc/fstab <<-EOF
	# <fs>           <mountpoint>    <type>  <opts>        <dump/pass>
	UUID=$root_uuid  /               ext4    rw,relatime   0 1
	EOF

	cat > etc/resolv.conf <<-EOF
	nameserver 8.8.8.8
	nameserver 8.8.4.4
	EOF

	cat > etc/network/interfaces <<-EOF
	auto lo
	iface lo inet loopback

	auto eth0
	iface eth0 inet dhcp
		hostname alpine
	EOF

	ln -sf /etc/init.d/cgroups etc/runlevels/sysinit/cgroups
	ln -sf /etc/init.d/devfs etc/runlevels/sysinit/devfs
	ln -sf /etc/init.d/dmesg etc/runlevels/sysinit/dmesg
	ln -sf /etc/init.d/hwdrivers etc/runlevels/sysinit/hwdrivers
	ln -sf /etc/init.d/mdev etc/runlevels/sysinit/mdev

	ln -sf /etc/init.d/bootmisc etc/runlevels/boot/bootmisc
	ln -sf /etc/init.d/modules etc/runlevels/boot/modules
	ln -sf /etc/init.d/networking etc/runlevels/boot/networking
	ln -sf /etc/init.d/hostname etc/runlevels/boot/hostname
	ln -sf /etc/init.d/hwclock etc/runlevels/boot/hwclock
	ln -sf /etc/init.d/swap etc/runlevels/boot/swap
	ln -sf /etc/init.d/sysctl etc/runlevels/boot/sysctl
	ln -sf /etc/init.d/syslog etc/runlevels/boot/syslog

	ln -sf /etc/init.d/sshd etc/runlevels/default/sshd

	ln -sf /etc/init.d/killprocs etc/runlevels/shutdown/killprocs
	ln -sf /etc/init.d/mount-ro etc/runlevels/shutdown/mount-ro
	ln -sf /etc/init.d/savecache etc/runlevels/shutdown/savecache

	_apk add --root . gcc git make openssh
	cp /etc/ssh/sshd_config etc/ssh/sshd_config

	cp /etc/hosts etc/hosts
	cp /etc/hostname etc/hostname

	cat /dev/null > etc/motd

	cd - > /dev/null
}
