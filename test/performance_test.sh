#!/bin/bash
# Performance and Load Testing Suite

set -e

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

# Configuration
CONCURRENT_REQUESTS=10
TOTAL_REQUESTS=100
SLOW_THRESHOLD_MS=1000

echo "========================================"
echo " Performance & Load Test Suite"
echo "========================================"
echo ""

# Get authentication token
echo "Authenticating..."
TOKEN=$(curl -s -X POST "http://localhost:35357/v3/auth/tokens" \
    -H "Content-Type: application/json" \
    -d '{"auth":{"identity":{"methods":["password"],"password":{"user":{"name":"admin","password":"secret","domain":{"name":"Default"}}}},"scope":{"project":{"name":"default","domain":{"name":"Default"}}}}}' \
    | jq -r '.token.token')

if [ -z "$TOKEN" ]; then
    echo -e "${RED}Failed to get authentication token${NC}"
    exit 1
fi

echo "Token obtained"
echo ""

# Test 1: Token issue performance
echo -e "${CYAN}=== Test 1: Token Issue Performance ===${NC}"
echo "Measuring token creation latency..."

TOKEN_TIMES=()
for i in {1..20}; do
    START=$(python3 -c 'import time; print(int(time.time() * 1000))')
    curl -s -X POST "http://localhost:35357/v3/auth/tokens" \
        -H "Content-Type: application/json" \
        -d '{"auth":{"identity":{"methods":["password"],"password":{"user":{"name":"admin","password":"secret","domain":{"name":"Default"}}}},"scope":{"project":{"name":"default","domain":{"name":"Default"}}}}}' > /dev/null
    END=$(python3 -c 'import time; print(int(time.time() * 1000))')
    DURATION=$((END - START))
    TOKEN_TIMES+=($DURATION)
done

# Calculate average
TOKEN_AVG=$(IFS=+; echo "scale=2; (${TOKEN_TIMES[*]}) / ${#TOKEN_TIMES[@]}" | bc)
echo -e "Token issue average: ${GREEN}${TOKEN_AVG}ms${NC}"

if (( $(echo "$TOKEN_AVG < 100" | bc -l) )); then
    echo -e "${GREEN}✓${NC} Token performance acceptable (< 100ms)"
else
    echo -e "${YELLOW}⚠${NC} Token performance slow (> 100ms)"
fi
echo ""

# Test 2: Server list performance
echo -e "${CYAN}=== Test 2: Server List Performance ===${NC}"
echo "Measuring server list latency with varying sizes..."

# Create test servers
echo "Creating 20 test servers..."
SERVER_IDS=()
for i in {1..20}; do
    SERVER_ID=$(curl -s -X POST "http://localhost:8774/v2.1/servers" \
        -H "X-Auth-Token: $TOKEN" \
        -H "Content-Type: application/json" \
        -d '{"server":{"name":"perf-test-server-'$i'","flavorRef":"m1.tiny"}}' \
        | jq -r '.server.id')
    if [ -n "$SERVER_ID" ] && [ "$SERVER_ID" != "null" ]; then
        SERVER_IDS+=($SERVER_ID)
    fi
done

echo "Created ${#SERVER_IDS[@]} servers"

# Measure list performance
LIST_TIMES=()
for i in {1..10}; do
    START=$(python3 -c 'import time; print(int(time.time() * 1000))')
    curl -s -X GET "http://localhost:8774/v2.1/servers/detail" \
        -H "X-Auth-Token: $TOKEN" > /dev/null
    END=$(python3 -c 'import time; print(int(time.time() * 1000))')
    DURATION=$((END - START))
    LIST_TIMES+=($DURATION)
done

LIST_AVG=$(IFS=+; echo "scale=2; (${LIST_TIMES[*]}) / ${#LIST_TIMES[@]}" | bc)
echo -e "Server list (20 items) average: ${GREEN}${LIST_AVG}ms${NC}"

if (( $(echo "$LIST_AVG < 500" | bc -l) )); then
    echo -e "${GREEN}✓${NC} List performance acceptable (< 500ms)"
else
    echo -e "${YELLOW}⚠${NC} List performance slow (> 500ms)"
fi
echo ""

# Test 3: Concurrent server creation
echo -e "${CYAN}=== Test 3: Concurrent Server Creation ===${NC}"
echo "Creating $CONCURRENT_REQUESTS servers concurrently..."

CONCURRENT_START=$(python3 -c 'import time; print(int(time.time() * 1000))')

# Create servers in parallel
PIDS=()
for i in $(seq 1 $CONCURRENT_REQUESTS); do
    (
        curl -s -X POST "http://localhost:8774/v2.1/servers" \
            -H "X-Auth-Token: $TOKEN" \
            -H "Content-Type: application/json" \
            -d '{"server":{"name":"concurrent-test-'$i'","flavorRef":"m1.tiny"}}' > /dev/null
    ) &
    PIDS+=($!)
done

# Wait for all requests
for PID in "${PIDS[@]}"; do
    wait $PID
done

CONCURRENT_END=$(python3 -c 'import time; print(int(time.time() * 1000))')
CONCURRENT_DURATION=$((CONCURRENT_END - CONCURRENT_START))
PER_REQUEST=$(echo "scale=2; $CONCURRENT_DURATION / $CONCURRENT_REQUESTS" | bc)

echo -e "Total time: ${GREEN}${CONCURRENT_DURATION}ms${NC}"
echo -e "Per request: ${GREEN}${PER_REQUEST}ms${NC}"

if (( $(echo "$PER_REQUEST < 200" | bc -l) )); then
    echo -e "${GREEN}✓${NC} Concurrent performance good"
else
    echo -e "${YELLOW}⚠${NC} Concurrent performance needs improvement"
fi
echo ""

# Test 4: Sustained load test
echo -e "${CYAN}=== Test 4: Sustained Load Test ===${NC}"
echo "Running $TOTAL_REQUESTS sequential requests..."

LOAD_START=$(python3 -c 'import time; print(int(time.time() * 1000))')
SUCCESS_COUNT=0
ERROR_COUNT=0

for i in $(seq 1 $TOTAL_REQUESTS); do
    HTTP_CODE=$(curl -s -w "%{http_code}" -o /dev/null \
        -X GET "http://localhost:8774/v2.1/servers" \
        -H "X-Auth-Token: $TOKEN")

    if [ "$HTTP_CODE" = "200" ]; then
        ((SUCCESS_COUNT++))
    else
        ((ERROR_COUNT++))
    fi

    # Progress indicator every 20 requests
    if [ $((i % 20)) -eq 0 ]; then
        echo -n "."
    fi
done

echo ""
LOAD_END=$(python3 -c 'import time; print(int(time.time() * 1000))')
LOAD_DURATION=$((LOAD_END - LOAD_START))
LOAD_AVG=$(echo "scale=2; $LOAD_DURATION / $TOTAL_REQUESTS" | bc)
REQUESTS_PER_SEC=$(echo "scale=2; $TOTAL_REQUESTS / ($LOAD_DURATION / 1000)" | bc)

echo -e "Total time: ${GREEN}${LOAD_DURATION}ms${NC}"
echo -e "Average per request: ${GREEN}${LOAD_AVG}ms${NC}"
echo -e "Requests/sec: ${GREEN}${REQUESTS_PER_SEC}${NC}"
echo -e "Success rate: ${GREEN}$SUCCESS_COUNT${NC}/${TOTAL_REQUESTS} ($(echo "scale=2; $SUCCESS_COUNT * 100 / $TOTAL_REQUESTS" | bc)%)"

if [ $ERROR_COUNT -eq 0 ]; then
    echo -e "${GREEN}✓${NC} No errors during load test"
else
    echo -e "${YELLOW}⚠${NC} $ERROR_COUNT errors occurred"
fi
echo ""

# Test 5: Database query performance
echo -e "${CYAN}=== Test 5: Database Query Performance ===${NC}"
echo "Creating resources and measuring query performance..."

# Create networks (joins test)
for i in {1..10}; do
    curl -s -X POST "http://localhost:9696/v2.0/networks" \
        -H "X-Auth-Token: $TOKEN" \
        -H "Content-Type: application/json" \
        -d '{"network":{"name":"perf-network-'$i'"}}' > /dev/null
done

# Measure network list with subnets (requires joins)
QUERY_TIMES=()
for i in {1..10}; do
    START=$(python3 -c 'import time; print(int(time.time() * 1000))')
    curl -s -X GET "http://localhost:9696/v2.0/networks" \
        -H "X-Auth-Token: $TOKEN" > /dev/null
    END=$(python3 -c 'import time; print(int(time.time() * 1000))')
    DURATION=$((END - START))
    QUERY_TIMES+=($DURATION)
done

QUERY_AVG=$(IFS=+; echo "scale=2; (${QUERY_TIMES[*]}) / ${#QUERY_TIMES[@]}" | bc)
echo -e "Network list average: ${GREEN}${QUERY_AVG}ms${NC}"

if (( $(echo "$QUERY_AVG < 100" | bc -l) )); then
    echo -e "${GREEN}✓${NC} Database query performance good"
else
    echo -e "${YELLOW}⚠${NC} Database query performance needs tuning"
fi
echo ""

# Test 6: Memory leak detection
echo -e "${CYAN}=== Test 6: Memory Leak Detection ===${NC}"
echo "Running repeated operations to detect memory leaks..."

# Take initial snapshot
INITIAL_CONNS=$(docker exec o3k-postgres psql -U lightstack -d lightstack -t -c "SELECT count(*) FROM pg_stat_activity WHERE application_name = 'pgx';" 2>/dev/null || echo "0")

# Run 100 operations
for i in {1..100}; do
    curl -s -X GET "http://localhost:8774/v2.1/flavors" \
        -H "X-Auth-Token: $TOKEN" > /dev/null
done

# Check final connections
sleep 2
FINAL_CONNS=$(docker exec o3k-postgres psql -U lightstack -d lightstack -t -c "SELECT count(*) FROM pg_stat_activity WHERE application_name = 'pgx';" 2>/dev/null || echo "0")

INITIAL_CONNS=$(echo $INITIAL_CONNS | xargs)
FINAL_CONNS=$(echo $FINAL_CONNS | xargs)

echo "Initial connections: $INITIAL_CONNS"
echo "Final connections: $FINAL_CONNS"

CONN_DIFF=$((FINAL_CONNS - INITIAL_CONNS))
if [ $CONN_DIFF -le 5 ]; then
    echo -e "${GREEN}✓${NC} No significant connection leak detected"
else
    echo -e "${YELLOW}⚠${NC} Possible connection leak: $CONN_DIFF extra connections"
fi
echo ""

# Cleanup
echo "Cleaning up test resources..."

# Delete servers
for SERVER_ID in "${SERVER_IDS[@]}"; do
    curl -s -X DELETE "http://localhost:8774/v2.1/servers/$SERVER_ID" \
        -H "X-Auth-Token: $TOKEN" 2>/dev/null || true
done

# Delete concurrent test servers
curl -s -X GET "http://localhost:8774/v2.1/servers" \
    -H "X-Auth-Token: $TOKEN" | jq -r '.servers[].id' | while read sid; do
    if [[ $sid == *"concurrent"* ]] || [[ $sid == *"perf"* ]]; then
        curl -s -X DELETE "http://localhost:8774/v2.1/servers/$sid" \
            -H "X-Auth-Token: $TOKEN" 2>/dev/null || true
    fi
done

# Delete networks
curl -s -X GET "http://localhost:9696/v2.0/networks" \
    -H "X-Auth-Token: $TOKEN" | jq -r '.networks[].id' | while read nid; do
    curl -s -X DELETE "http://localhost:9696/v2.0/networks/$nid" \
        -H "X-Auth-Token: $TOKEN" 2>/dev/null || true
done

echo ""
echo "========================================"
echo "Performance Test Summary"
echo "========================================"
echo -e "Token creation:    ${GREEN}${TOKEN_AVG}ms${NC}"
echo -e "Server list (20):  ${GREEN}${LIST_AVG}ms${NC}"
echo -e "Concurrent ($CONCURRENT_REQUESTS): ${GREEN}${PER_REQUEST}ms/req${NC}"
echo -e "Sustained load:    ${GREEN}${LOAD_AVG}ms/req${NC} (${REQUESTS_PER_SEC} req/s)"
echo -e "Database queries:  ${GREEN}${QUERY_AVG}ms${NC}"
echo ""
echo -e "${GREEN}Performance testing complete!${NC}"
