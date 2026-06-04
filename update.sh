#!/usr/bin/env sh
set -eu

INSTALL_DIR=${AHUB_DIR:-${XDG_DATA_HOME:-$HOME/.local/share}/ahub}
BIN_DIR=${AHUB_BIN_DIR:-$HOME/.local/bin}
BIN_PATH=$BIN_DIR/ahub

if [ ! -x "$BIN_PATH" ]; then
	echo "ahub is not installed at $BIN_PATH; run ./install.sh first" >&2
	exit 1
fi

if [ ! -d "$INSTALL_DIR/.git" ]; then
	echo "$INSTALL_DIR is not a git repository" >&2
	exit 1
fi

( cd "$INSTALL_DIR" && "$BIN_PATH" update )
