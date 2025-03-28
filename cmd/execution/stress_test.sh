#!/bin/bash
date
echo "Start testing 50 concurrent requests"


TOTAL_REQUESTS=500

MAX_CONCURRENT=50

send_request() {
    hostid=$(( $1 % 4 ))

#	curl -s -w "%{time_total}s for hostid $hostid\n" localhost:8081/executor -d "{\"hostid\":$hostid,\"scriptid\":1}"

    #curl -s -w "%{time_total}" localhost:8081/executor -d "{\"hostid\":$hostid,\"scriptid\":1}\""

    time=$(curl -s -w "%{time_total}" -o /dev/null localhost:8081/executor -d "{\"hostid\":$hostid,\"scriptid\":1}")
    echo "Request $1 (hostid $hostid) took ${time}s"
}


for ((i=0; i<TOTAL_REQUESTS; i++)); do

    while [ $(jobs -r | wc -l) -ge "$MAX_CONCURRENT" ]; do
        sleep 0.1

    send_request "$i" &
done

wait
date
echo "Test complete"
