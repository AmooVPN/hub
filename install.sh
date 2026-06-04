#!/usr/bin/env sh
set -eu

REPO_URL=${AHUB_REPO:-https://github.com/AmooVPM/hub.git}
INSTALL_DIR=${AHUB_DIR:-/opt/ahub}

if ! command -v docker >/dev/null 2>&1; then
	echo "docker is required" >&2
	exit 1
fi

if ! command -v git >/dev/null 2>&1; then
	echo "git is required" >&2
	exit 1
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

docker run --rm \
	-v "$INSTALL_DIR:/src" \
	-w /src \
	golang:1.22 \
	go build -o "$BIN_DIR/ahub" ./cmd/ahub

chmod +x "$BIN_DIR/ahub"

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
