#!/bin/sh

set -ex

argv0="$(basename "$0")"

[ "$(whoami)" != "root" ] && {
	>&2 printf "%s: must be run as root\n" "$argv0"
	exit 1
}

[ ! -f /etc/arch-release ] && {
	>&2 printf "%s: must be run on arch\n" "$argv0"
	exit 1
}

[ -z "$COUNTRY" ] && {
	>&2 printf "%s: COUNTRY for mirror list not set\n" "$argv0"
	exit 1
}

. ./util.sh

mirrorlist="https://archlinux.org/mirrorlist/?country=$COUNTRY&protocol=http&protocol=https&ip_version=4"

_pacman() {
	pacman --noconfirm --noprogressbar "$@"
}

cleanup_arch() {
	swapoff "$nbd_dev"p1
	cleanup_nbd
}

build_yay() {
	root="$1"
	tmp="$(mktemp -d)"

	git clone https://github.com/jguer/yay "$tmp"

	cd "$tmp" && {
		git checkout $(git describe)

		make
		yaybin="$(pwd)/yay"

		cd -
	}

	[ "$yaybin" != "" ] && mv "$yaybin" "$root"/usr/bin
}

_pacman -Syy
_pacman -S arch-install-scripts go gcc qemu-headless make

prepare_nbd arch

trap cleanup_arch EXIT

sfdisk -d /dev/vda | sfdisk "$nbd_dev"

mkswap /dev/nbd0p1
mkfs.ext4 /dev/nbd0p2

mount "$nbd_dev"p2 /mnt

swapon "$nbd_dev"p1

cd /mnt && {
	_pacman -Sy archlinux-keyring

	pacstrap . base base-devel linux --noprogressbar

	prepare_chroot .

	root_uuid="$(blk_uuid "$nbd_dev"p2)"
	swap_uuid="$(blk_uuid "$nbd_dev"p1)"

	cat > etc/fstab <<-EOF
	UUID=$root_uuid  /               ext4    rw,relatime   0 1
	UUID=$swap_uuid  none            swap    defaults      0 0
	EOF

	cp /etc/hosts /etc/hostname /etc/resolv.conf etc

	curl -s "$mirrorlist" | sed 's/#Server/Server/' > etc/pacman.d/mirrorlist

	build_yay /mnt

	_pacman -Syy

	chroot . ln -sf usr/share/zoneinfo/Etc/UTC etc/localtime
	chroot . hwclock --systohc

	chroot . echo "en_GB.UTF-8 UTF-8" > /etc/locale.gen
	chroot . locale-gen
	chroot . echo "LANG=en_GB.UTF-8" > /etc/locale.conf

	_pacman -S dhcpcd gcc git grub make openssh --sysroot .

	chroot . systemctl enable dhcpcd systemd-resolved sshd

	cp /etc/ssh/sshd_config etc/ssh
	cp /etc/systemd/resolved.conf etc/systemd

	cp /etc/pacman.d/gnupg/gpg.conf etc/pacman.d/gnupg

	cp /etc/default/grub etc/default

	chroot . passwd -d root

	chroot . grub-install /dev/nbd0
	chroot . grub-mkconfig -o boot/grub/grub.cfg

	cat /dev/null > /etc/motd

	cd - > /dev/null
}
