#!/bin/bash
# Build Go program for Windows and Linux, naming binaries after source file + OS + ARCH

SRC_FILE="main.go"


BASE_NAME=$(grep "^module " go.mod | awk '{print $2}')


BUILD_DIR="build"
mkdir -p "$BUILD_DIR"

# Targets: OS/ARCH
TARGETS=(
    "windows/amd64"
    "windows/386"
    "linux/amd64"
    "linux/386"
)

# Loop through targets and build
for target in "${TARGETS[@]}"; do
    IFS="/" read -r GOOS GOARCH <<< "$target"

    # Build output file name: <basename>_<os>_<arch>[.exe]
    OUTPUT="$BUILD_DIR/${BASE_NAME}_${GOOS}_${GOARCH}"
    [ "$GOOS" = "windows" ] && OUTPUT="$OUTPUT.exe"

    echo "Building for $GOOS/$GOARCH -> $OUTPUT"
    GOOS=$GOOS GOARCH=$GOARCH go build -o "$OUTPUT" "$SRC_FILE"
done

echo "All binaries built in $BUILD_DIR/"
