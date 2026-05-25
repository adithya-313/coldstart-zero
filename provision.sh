#!/bin/sh
# Fail immediately if any command fails
set -e

# Define the paths for the data file and the hash storage file
DATA_FILE="/app/weights/dummy-model.bin"
SHA256_FILE="/app/weights/dummy-model.bin.sha256"

# Generate a 10MB dummy model file (10485760 bytes of random data)
FILE_SIZE_MB=10

# -- CACHE CHECK -----------------------------------------------------------
# Check if the data file and its hash file both already exist
if [ -f "$DATA_FILE" ] && [ -f "$SHA256_FILE" ]; then
    # Read the previously stored expected hash
    EXPECTED_HASH=$(cat "$SHA256_FILE")
    # Compute the SHA256 hash of the current data file
    ACTUAL_HASH=$(sha256sum "$DATA_FILE" | cut -d' ' -f1)

    # Compare the computed hash with the expected hash
    if [ "$ACTUAL_HASH" = "$EXPECTED_HASH" ]; then
        echo "CACHE HIT: Skipping network egress"
        exit 0
    else
        echo "CACHE MISS: Hash mismatch (expected $EXPECTED_HASH, got $ACTUAL_HASH)"
        echo "Removing corrupted file and re-generating..."
        rm -f "$DATA_FILE" "$SHA256_FILE"
    fi
fi

# -- CACHE MISS: Generate the dummy model file -----------------------------
echo "CACHE MISS: Initiating download"

# Generate a 10MB file using /dev/urandom for random model weights
dd if=/dev/urandom of="$DATA_FILE" bs=1M count="$FILE_SIZE_MB" 2>/dev/null

# Compute the SHA256 hash of the newly generated file
NEW_HASH=$(sha256sum "$DATA_FILE" | cut -d' ' -f1)

# Store the hash in the companion hash file for future cache checks
echo "$NEW_HASH" > "$SHA256_FILE"

# Confirm the result to the user
echo "Generated $DATA_FILE ($FILE_SIZE_MB MB, SHA256: $NEW_HASH)"
exit 0
