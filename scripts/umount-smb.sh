#!/bin/sh
set -e

. ./.env

SMB_DIR="${SMB_MOUNT_PATH:-./smb}"

OS=$(uname -s)
if [ "$OS" = "Darwin" ]; then
  umount "$SMB_DIR" 2>/dev/null || true
elif [ -d /proc ]; then
  umount "$SMB_DIR" 2>/dev/null || true
fi

echo "SMB share unmounted."
