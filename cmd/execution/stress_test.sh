#!/bin/bash
date
echo "Start testing 50 concurrent requests"

# Total requests (e.g., 500 to keep it manageable)
TOTAL_REQUESTS=500
# Max concurrent requests
MAX_CONCURRENT=50

# Function to send a request
send_request() {
    hostid=$(( $1 % 4 ))
    # Add -w to see response time for each request
#	curl -s -w "%{time_total}s for hostid $hostid\n" localhost:8081/executor -d "{\"hostid\":$hostid,\"scriptid\":1}"

    #curl -s -w "%{time_total}" localhost:8081/executor -d "{\"hostid\":$hostid,\"scriptid\":1}\""

    time=$(curl -s -w "%{time_total}" -o /dev/null localhost:8081/executor -d "{\"hostid\":$hostid,\"scriptid\":1}")
    echo "Request $1 (hostid $hostid) took ${time}s"
}

# Send requests with concurrency limit
for ((i=0; i<TOTAL_REQUESTS; i++)); do
    # Wait if we hit the concurrency limit
    while [ $(jobs -r | wc -l) -ge "$MAX_CONCURRENT" ]; do
        sleep 0.1
    done
    # Run in background
    send_request "$i" &
done

# Wait for all requests to finish
wait
date
echo "Test complete"
