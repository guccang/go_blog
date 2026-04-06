#!/bin/bash
set -e

cd "$(dirname "$0")"

echo "Building Flutter web..."
flutter build web

TIMESTAMP=$(date +"%Y-%m-%d-%H_%M_%S")
OUTPUT="flutter-web_${TIMESTAMP}.zip"

echo "Packaging build/web + scripts to ${OUTPUT}..."
cd build/web
zip -r "../../${OUTPUT}" .
cd ../..

# Include publish scripts
zip -g "${OUTPUT}" publish.sh publish.bat 2>/dev/null || true

echo "Generated: ${OUTPUT}"
