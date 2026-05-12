#!/bin/sh
set -e

. ./.env

# Create local mount directory
SMB_DIR="${SMB_MOUNT_PATH:-./smb}"
mkdir -p "$SMB_DIR"

echo "Mounting SMB share ${SMB_HOST}/${SMB_SHARE} to ${SMB_DIR}..."

OS=$(uname -s)

if [ "$OS" = "Darwin" ]; then
  # macOS - use mount_smbfs
  MOUNT_URL="smb://${SMB_USERNAME}:${SMB_PASSWORD}@${SMB_HOST}/${SMB_SHARE}"
  if mount | grep -q "$SMB_DIR"; then
    echo "Already mounted at ${SMB_DIR}"
  else
    mount_smbfs "$MOUNT_URL" "$SMB_DIR"
  fi
elif [ -d /proc ]; then
  # Linux
  if mountpoint -q "$SMB_DIR" 2>/dev/null; then
    echo "Already mounted at ${SMB_DIR}"
  else
    mount -t cifs "//${SMB_HOST}/${SMB_SHARE}" "$SMB_DIR" \
      -o username="${SMB_USERNAME}",password="${SMB_PASSWORD}",uid=$(id -u),gid=$(id -g)
  fi
else
  echo "Unsupported OS: $OS"
  echo "Manually mount SMB share to $SMB_DIR"
  exit 1
fi

echo "SMB share mounted successfully."
