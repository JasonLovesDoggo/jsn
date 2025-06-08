#!/bin/bash
set -e

echo "Installing portkill utility..."

# Build the binary
go build -o portkill

# Create the pk alias
if [[ -f portkill ]]; then
    # Determine installation directory
    INSTALL_DIR=${GOBIN:-$GOPATH/bin}
    if [[ -z "$INSTALL_DIR" ]]; then
        INSTALL_DIR=/usr/local/bin
    fi

    # Install portkill
    echo "Installing to $INSTALL_DIR/portkill"
    cp portkill "$INSTALL_DIR/"

    # Create the pk symlink
    echo "Creating pk alias symlink"
    ln -sf "$INSTALL_DIR/portkill" "$INSTALL_DIR/pk"

    echo "Installation complete!"
    echo "You can now use either 'portkill' or 'pk' command"
else
    echo "Build failed, portkill binary not found"
    exit 1
fi
