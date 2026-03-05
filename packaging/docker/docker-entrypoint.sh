#!/bin/sh
set -e

ARGS=""
[ -n "$RECUR_CONFIG_PATH" ] && ARGS="$ARGS --config $RECUR_CONFIG_PATH"
[ -n "$RECUR_SOCKET" ] && ARGS="$ARGS --socket $RECUR_SOCKET"

# Start daemon in background
recurd $ARGS &
DAEMON_PID=$!

# Wait for daemon to be ready, then print status
sleep 2
echo "=== recur status ==="
recur ${RECUR_SOCKET:+--socket $RECUR_SOCKET} status 2>/dev/null || echo "Daemon starting..."
echo ""
echo "=== installed plugins ==="
recur ${RECUR_SOCKET:+--socket $RECUR_SOCKET} list plugins 2>/dev/null || echo "No plugins registered"
echo ""

# Wait for daemon process
wait $DAEMON_PID
