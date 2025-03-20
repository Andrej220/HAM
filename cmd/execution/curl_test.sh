#!/bin/bash
date
echo "Start high-load testing"


for i in {0..30000}; do
    hostid=$((i % 4)) #
    curl -s localhost:8081/executor -d "{\"hostid\":$hostid,\"scriptid\":1}" > /dev/null &
done


wait
date
echo "Stop high-load testing"
