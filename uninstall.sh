#!/usr/bin/env sh
set -eu

INSTALL_DIR=${AHUB_DIR:-/opt/ahub}
BIN_DIR=${AHUB_BIN_DIR:-${HOME:-/root}/.local/bin}
BIN_PATH=$BIN_DIR/ahub

if [ -d "$INSTALL_DIR" ] && [ -f "$INSTALL_DIR/docker-compose.yml" ]; then
	( cd "$INSTALL_DIR" && docker compose down -v --remove-orphans )
fi

rm -f "$BIN_PATH"
rm -rf "$INSTALL_DIR"

echo "uninstalled ahub"
