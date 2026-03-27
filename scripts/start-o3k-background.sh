#!/bin/bash
# Start O3K in background for CI testing
# This script fully detaches O3K from the calling shell

set -e

# Start O3K with all I/O redirected and fully detached
(./bin/o3k serve > /tmp/o3k.log 2>&1 < /dev/null &)

# Get the PID
sleep 1
O3K_PID=$(pgrep -f "o3k serve" | head -1)

if [ -z "$O3K_PID" ]; then
    echo "❌ Failed to find O3K process"
    exit 1
fi

echo "$O3K_PID" > /tmp/o3k.pid
echo "O3K started (PID: $O3K_PID)"
