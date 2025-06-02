#!/bin/bash
date
echo "Start high-load testing"

REPNUM=3

# Check if parameter exists and is a positive integer
if [ -n "$1" ] && [ "$1" -gt 0 ] 2>/dev/null; then
    REPNUM=$1
else
    echo "Using default REPNUM=$REPNUM (provide a positive integer as argument to override)" >&2
fi

for ((i=0; i<REPNUM; i++)); do
    hostid=$((i % 4))
    curl -s localhost:8081/executor -d "{\"hostid\":$hostid,\"scriptid\":1}" > /dev/null &
done

wait
date
echo "Sent $((REPNUM)) requests" >&2
