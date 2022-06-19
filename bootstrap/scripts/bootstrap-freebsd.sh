#!/bin/sh

set -ex

argv0="$(basename "$0")"

[ "$(whoami)" != "root" ] && {
	>&2 printf "%s: must be run as root\n" "$argv0"
	exit 1
}

[ -z "$RELEASE" ] && {
	>&2 printf "%s: RELEASE not set\n" "$argv0"
	exit 1
}

download_url="https://download.freebsd.org/releases/amd64"

pkg install -y qemu-tools

truncate -s 10g freebsd.raw
mdconfig -a -t vnode -f freebsd.raw

gpart create -s gpt /dev/md0
gpart add -t freebsd-boot -l mdboot -b 40 -s 512K md0
gpart bootcode -b /boot/pmbr -p /boot/gptboot -i 1 md0
gpart add -t freebsd-ufs -l mdroot -b 1M -s 9G md0
newfs -U /dev/md0p2

mount /dev/md0p2 /mnt

mkdir /mnt/dev

mount -t devfs devfs /mnt/dev

wget -q "$download_url/$RELEASE/base.txz" "$download_url/$RELEASE/kernel.txz"

tar -C /mnt -xzf base.txz
tar -C /mnt -xzf kernel.txz

freebsd-update -b /mnt --currently-running $RELEASE --not-running-from-cron fetch install

cp /etc/resolv.conf /mnt/etc
cp /etc/ssh/sshd_config /mnt/etc/ssh
cp /etc/hosts /mnt/etc

cat > /mnt/etc/rc.conf <<EOF
ntpd_enable=YES
growfs_enable=YES
sshd_enable=YES
hostname=freebsd
ifconfig_vtnet0=DHCP
EOF

echo "/dev/vtbd0p2    /    ufs    rw,noatime    1    1" > /mnt/etc/fstab

[ -f /mnt/etc/motd.template ] && cat /dev/null > /mnt/etc/motd.template
[ -f /mnt/etc/motd ] && cat /dev/null > /mnt/etc/motd

cat > /mnt/boot/loader.conf <<EOF
kern.geom.label.disk_ident.enable="0"
kern.geom.label.gptid.enable="0"
autoboot_delay="0"
vfs.root.mountfrom="ufs:/dev/vtbd0p2"
EOF

tzsetup -s -C /mnt UTC
mkdir -p /mnt/usr/local/etc/pkg/repos

cat > /mnt/usr/local/etc/pkg/repos/FreeBSD.conf <<EOF
FreeBSD: {
	url: pkg+http://pkg.FreeBSD.org/\$\{ABI\}/latest
	enabled: yes
}
EOF

env ASSUME_ALWAYS_YES=YES pkg -c /mnt bootstrap -f

pkg -c /mnt install -y git wget

umount /mnt/dev /mnt
mdconfig -du md0

qemu-img convert -f raw -O qcow2 freebsd.raw freebsd
qemu-img resize freebsd 10G
