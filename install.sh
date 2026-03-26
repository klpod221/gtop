#!/usr/bin/env bash
set -e

echo "=> Building gtop frontend..."
if [ -d "web" ]; then
    cd web
    npm install
    npm run build
    cd ..
else
    echo "Warning: 'web' directory not found, skipping frontend build."
fi

echo "=> Building gtop backend binary..."
go build -ldflags="-s -w" -o gtop .

echo "=> Installing to /usr/local/bin (requires sudo)..."
sudo cp gtop /usr/local/bin/gtop
sudo chmod +x /usr/local/bin/gtop

echo ""
echo "=> Install complete! You can now run 'gtop' from anywhere."
echo "   Try 'gtop web' or 'gtop help' to get started."
