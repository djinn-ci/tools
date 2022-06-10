#!/bin/sh

set -e

argv0="$(basename "$0")"

[ -z "$DJINN_CONF_DIR" ] && DJINN_CONF_DIR="/etc/djinn"
[ -z "$DJINN_USER" ] && DJINN_USER="djinn"
[ -z "$DJINN_GROUP" ] && DJINN_GROUP="djinn"

[ ! -d bin ] && mkdir bin

build() {
	ldflags="-X 'djinn-ci.com/x/tools/internal.ConfigDir=$DJINN_CONF_DIR'"

	for d in $(ls cmd); do
		set -x
		go build -ldflags "$ldflags" -tags netgo -o bin/"$d" ./cmd/"$d"
		set +x
	done
}

install_bootstrap() {
	timers=$(ls bootstrap/systemd/*.timer | sed 's|bootstrap/systemd/||')

	install -m 0644 bootstrap/systemd/* /usr/lib/systemd/system
	install -m 0644 -o "$DJINN_USER" -g "$DJINN_GROUP" bootstrap/driver.conf "$DJINN_CONF_DIR"/bootstrap.conf

	systemctl enable $timers
	systemctl start $timers
}

install() {
	[ $(whoami) != "root" ] && {
		>&2 printf "%s: install must be run as root\n" "$argv0"
		exit 1
	}

	[ ! -d "$DJINN_CONF_DIR" ] && {
		>&2 printf "%s: no such directory %s\n" "$argv0" "$DJINN_CONF_DIR"
		exit 1
	}

	[ ! -d bin || $(ls bin | wc -l ) -eq 0 ] && {
		>&2 printf "%s: no binaries to install, make sure you run './make.sh'\n" "$argv0"
		exit 1
	}

	if systemctl status djinn-worker &> /dev/null; then
		install_bootstrap
	fi

	install -m 0755 djinn-ps /usr/local/bin

	for b in $(ls bin); do
		install -m 0755 bin/"$b" /usr/local/bin
	done
}

if [ "$1" = "install" ]; then
	install
fi

build
