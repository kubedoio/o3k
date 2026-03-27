#!/bin/bash
# Start O3K in background for CI testing
# Use proper job control to fully detach from shell

set +e  # Don't exit on error during detachment

# Start O3K, capture PID immediately, then disown
./bin/o3k serve </dev/null >/tmp/o3k.log 2>&1 &
O3K_PID=$!
disown

# Store PID
echo "$O3K_PID" > /tmp/o3k.pid

# Wait for process to initialize
sleep 3

# Verify process is running
if ! kill -0 "$O3K_PID" 2>/dev/null; then
    echo "❌ O3K process died (PID: $O3K_PID)"
    echo "Last 20 lines of log:"
    tail -20 /tmp/o3k.log 2>/dev/null || echo "No log file"
    exit 1
fi

echo "✅ O3K started successfully (PID: $O3K_PID)"
exit 0
