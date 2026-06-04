#!/usr/bin/env sh
set -eu

REPO_URL=${AHUB_REPO:-https://github.com/AmooVPN/hub.git}
INSTALL_DIR=${AHUB_DIR:-${XDG_DATA_HOME:-$HOME/.local/share}/ahub}

run_as_root() {
	if [ "$(id -u)" -eq 0 ]; then
		"$@"
		return
	fi
	if command -v sudo >/dev/null 2>&1; then
		sudo "$@"
		return
	fi
	echo "sudo is required to install git" >&2
	exit 1
}

install_git() {
	if [ -r /etc/os-release ]; then
		. /etc/os-release
	fi
	case "${ID:-}" in
		ubuntu|debian|linuxmint|pop|elementary)
			run_as_root apt-get update
			run_as_root apt-get install -y git
			;;
		fedora|rhel|centos|rocky|almalinux)
			if command -v dnf >/dev/null 2>&1; then
				run_as_root dnf install -y git
			else
				run_as_root yum install -y git
			fi
			;;
		arch|manjaro)
			run_as_root pacman -Sy --noconfirm git
			;;
		alpine)
			run_as_root apk add --no-cache git
			;;
		sles|suse|opensuse*|opensuse)
			run_as_root zypper --non-interactive install git
			;;
		*)
			echo "git is required and automatic installation is not supported on this distro" >&2
			exit 1
			;;
	esac
}

install_go() {
	if [ -r /etc/os-release ]; then
		. /etc/os-release
	fi
	case "${ID:-}" in
		ubuntu|debian|linuxmint|pop|elementary)
			run_as_root apt-get update
			run_as_root apt-get install -y golang-go
			;;
		fedora|rhel|centos|rocky|almalinux)
			run_as_root dnf install -y golang
			;;
		arch|manjaro)
			run_as_root pacman -Sy --noconfirm go
			;;
		alpine)
			run_as_root apk add --no-cache go
			;;
		sles|suse|opensuse*|opensuse)
			run_as_root zypper --non-interactive install go
			;;
		*)
			echo "go is required and automatic installation is not supported on this distro" >&2
			exit 1
			;;
	esac
}

build_ahub() {
	if command -v docker >/dev/null 2>&1; then
		docker run --rm \
			-u "$(id -u):$(id -g)" \
			-v "$INSTALL_DIR:/src" \
			-v "$BIN_DIR:/out" \
			-w /src \
			golang:1.22 \
			go build -buildvcs=false -o /out/ahub.tmp ./cmd/ahub
	else
		if ! command -v go >/dev/null 2>&1; then
			install_go
		fi
		( cd "$INSTALL_DIR" && go build -buildvcs=false -o "$BIN_DIR/ahub.tmp" ./cmd/ahub )
	fi
	mv -f "$BIN_DIR/ahub.tmp" "$BIN_DIR/ahub"
}

if ! command -v git >/dev/null 2>&1; then
	install_git
fi

if [ -d "$INSTALL_DIR/.git" ]; then
	git -C "$INSTALL_DIR" pull --ff-only
else
	mkdir -p "$(dirname "$INSTALL_DIR")"
	if [ -d "$INSTALL_DIR" ]; then
		echo "$INSTALL_DIR already exists and is not a git repo" >&2
		exit 1
	fi
	git clone "$REPO_URL" "$INSTALL_DIR"
fi

BIN_DIR=${AHUB_BIN_DIR:-$HOME/.local/bin}
mkdir -p "$BIN_DIR"

build_ahub

if [ ! -f "$INSTALL_DIR/.env" ]; then
	"$BIN_DIR/ahub" create env
fi
"$BIN_DIR/ahub" create docker --force

if [ "${AHUB_SETUP_NOW:-false}" = "true" ]; then
	if [ -z "${AHUB_ADMIN_PASSWORD:-}" ]; then
		echo "AHUB_ADMIN_PASSWORD is required when AHUB_SETUP_NOW=true" >&2
		exit 1
	fi
	"$BIN_DIR/ahub" setup --username "${AHUB_ADMIN_USERNAME:-admin}" --password "$AHUB_ADMIN_PASSWORD" --email "${AHUB_ADMIN_EMAIL:-}" --role "${AHUB_ADMIN_ROLE:-owner}"
fi

echo "Installed ahub to $BIN_DIR/ahub"
echo "Run: cd $INSTALL_DIR && ahub start"
