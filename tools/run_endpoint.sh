#!/bin/bash
set -e

# Set up the routing needed for the simulation.
#/setup.sh

mkdir -p /logs/qlog

if [ "$ROLE" == "sender" ]; then
    # Wait for the simulator to start up.
    #/wait-for-it.sh sim:57832 -s -t 10
    echo "Starting RTQ sender..."
    QUIC_GO_LOG_LEVEL=error ./rtq send -addr $RECEIVER $SENDER_PARAMS $VIDEOS
else
    echo "Running RTQ receiver."
    QUIC_GO_LOG_LEVEL=error ./rtq receive $RECEIVER_PARAMS $DESTINATION
fi
