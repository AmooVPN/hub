#!/usr/bin/env sh
set -eu

INSTALL_DIR=${AHUB_DIR:-${XDG_DATA_HOME:-$HOME/.local/share}/ahub}
BIN_DIR=${AHUB_BIN_DIR:-${HOME:-/root}/.local/bin}
BIN_PATH=$BIN_DIR/ahub

if [ -d "$INSTALL_DIR" ] && [ -f "$INSTALL_DIR/docker-compose.yml" ]; then
	( cd "$INSTALL_DIR" && docker compose down -v --remove-orphans )
	( cd "$INSTALL_DIR" && docker compose images -q hub 2>/dev/null | xargs -r docker image rm -f )
fi

docker volume rm -f hub-data hub-backups 2>/dev/null || true

rm -f "$BIN_PATH"
rm -rf "$INSTALL_DIR"

echo "uninstalled ahub"
