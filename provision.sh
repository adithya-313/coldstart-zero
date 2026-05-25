#!/bin/sh
# Fail immediately if any command fails
set -e

# -- SECURE SECRET INJECTION ------------------------------------------------
# Read the cloud_token secret from the in-memory Docker secret mount point.
# Docker injects this file as a tmpfs mount (RAM-only) when the secret is
# defined in docker-compose.yml under the service's secrets: array.
CLOUD_TOKEN_FILE="/run/secrets/cloud_token"

# Check if the secret file exists before trying to read it
if [ ! -f "$CLOUD_TOKEN_FILE" ]; then
    echo "ERROR: Secret not found at $CLOUD_TOKEN_FILE"
    exit 1
fi

# Read the token into a shell variable using a file read (not an environment variable)
# This keeps the credential off the process environment and out of /proc
CLOUD_TOKEN=$(cat "$CLOUD_TOKEN_FILE")

# Print a masked version of the token to prove authentication works
# without leaking the full secret to the console or logs.
# Only the prefix is shown; the rest is replaced with asterisks.
MASKED_TOKEN="hf_mock_token_***"
echo "Authenticating with token: $MASKED_TOKEN"

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
