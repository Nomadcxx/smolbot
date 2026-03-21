#!/bin/bash
# smolbot one-line installer
# Usage: curl -fsSL https://raw.githubusercontent.com/Nomadcxx/smolbot/main/install.sh | bash

set -e

echo "SMOLBOT"
echo "smolbot installer"
echo ""

# Check prerequisites
if ! command -v go &> /dev/null; then
    echo "Error: Go is not installed"
    echo "Install Go 1.21+: https://golang.org/dl/"
    exit 1
fi

if ! command -v git &> /dev/null; then
    echo "Error: Git is not installed"
    exit 1
fi

# Create temp directory with cleanup trap
TEMP_DIR=$(mktemp -d)
trap "cd / && rm -rf '$TEMP_DIR'" EXIT

cd "$TEMP_DIR"

echo "Cloning smolbot..."
git clone --depth 1 https://github.com/Nomadcxx/smolbot.git
cd smolbot

echo "Building installer..."
go build -o install-smolbot ./cmd/installer/

echo "Starting installer..."
./install-smolbot

# Cleanup happens via trap on exit
