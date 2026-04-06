#!/bin/bash
set -e

cd "$(dirname "$0")"

# Kill any existing process on port 8884
echo "Stopping existing web server on port 8884..."
lsof -ti:8884 | xargs kill -9 2>/dev/null || true
sleep 1

# Extract the web files if not already extracted
shopt -s nullglob
zip_files=(flutter-web_*.zip)
if [ ${#zip_files[@]} -gt 0 ]; then
    echo "Extracting Flutter web files..."
    mkdir -p build/web
    unzip -o "${zip_files[$((${#zip_files[@]}-1))]}" -d build/web/
fi
shopt -u nullglob

# Start Python static server on port 8884
echo "Starting Flutter web server on port 8884..."
cd build/web
nohup python3 -m http.server 8884 > ../../flutter-web.log 2>&1 < /dev/null &
cd ..
disown

sleep 2

# Verify server is running
if lsof -ti:8884 > /dev/null; then
    echo "Flutter web server started on port 8884"
else
    echo "Failed to start server, check flutter-web.log"
    tail -20 ../../flutter-web.log
    exit 1
fi
