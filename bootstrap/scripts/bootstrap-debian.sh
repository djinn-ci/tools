#!/bin/sh

set -ex

argv0="$(basename "$0")"

[ "$(whoami)" != "root" ] && {
	>&2 printf "%s: must be run as root\n" "$argv0"
	exit 1
}

[ -z "$CODENAME" ] && {
	>&2 printf "%s: CODENAME for debootstrap not set\n" "$argv0"
	exit 1
}

. ./util.sh

cleanup_debian() {
	swapoff "$nbd_dev"p5
	cleanup_nbd
}

debootstrap="debootstrap_1.0.123_all.deb"

apt install -y qemu-utils

wget https://deb.debian.org/debian/pool/main/d/debootstrap/"$debootstrap"

dpkg -i "$debootstrap"

prepare_nbd debian

trap cleanup_debian EXIT

sfdisk -d /dev/vda | sfdisk "$nbd_dev"

mkfs.ext4 "$nbd_dev"p1
mkswap "$nbd_dev"p5

mount "$nbd_dev"p1 /mnt

swapon "$nbd_dev"p5

cd /mnt && {
	debootstrap "$CODENAME" . https://deb.debian.org/debian

	prepare_chroot .

	chroot . apt install -y firmware-linux-free linux-image-amd64

	root_uuid="$(blk_uuid "$nbd_dev"p1)"
	swap_uuid="$(blk_uuid "$nbd_dev"p5)"

	cat > etc/fstab <<-EOF
	UUID=$root_uuid  /               ext4    errors=remount-ro   0 1
	UUID=$swap_uuid  none            swap    sw                  0 0
	EOF

	cat > etc/resolv.conf <<-EOF
	nameserver 8.8.8.8
	nameserver 8.8.4.4
	EOF

	cat > etc/network/interfaces <<-EOF
	auto lo
	iface lo inet loopback

	allow-hotplug ens3
	iface ens3 inet dhcp
		hostname debian
	EOF

	cp /etc/hosts /etc/hostname etc

	cat > etc/apt/sources.list <<-EOF
	deb http://deb.debian.org/debian $CODENAME main
	deb-src http://deb.debian.org/debian $CODENAME main

	deb http://deb.debian.org/debian $CODENAME-updates main
	deb-src http://deb.debian.org/debian $CODENAME-updates main
	EOF

	chroot . apt update -y
	chroot . apt upgrade -y

	chroot . apt install -y build-essential git grub2 openssh-server wget

	cp /etc/ssh/sshd_config etc/ssh
	cp /etc/default/grub etc/default

	chroot . passwd -d root

	chroot . grub-install /dev/nbd0
	chroot . grub-mkconfig -o boot/grub/grub.cfg

	cat /dev/null > etc/motd

	cd - > /dev/null
}
