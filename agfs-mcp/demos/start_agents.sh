#!/bin/bash
# Start multiple task_loop agents in the background

set -e

# Configuration
AGENTS=${AGENTS:-"agent1 agent2 agent3 agent4 agent5 agent6 agent7 agent8 agent9 agent10"}
QUEUE_PREFIX=${QUEUE_PREFIX:-"/queuefs"}
API_URL=${API_URL:-"http://localhost:8080"}
WORKING_DIR=${WORKING_DIR:-"."}
CLAUDE_TIMEOUT=${CLAUDE_TIMEOUT:-600}
ALLOWED_TOOLS=${ALLOWED_TOOLS:-"WebFetch,Read,Write,Bash,Glob,Grep,agfs"}

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${BLUE}================================${NC}"
echo -e "${BLUE}🚀 Starting AGFS Task Loop Agents${NC}"
echo -e "${BLUE}================================${NC}"
echo ""

# Create logs directory
LOGS_DIR="./logs"
mkdir -p "$LOGS_DIR"

echo -e "${YELLOW}Configuration:${NC}"
echo -e "  Agents:        ${AGENTS}"
echo -e "  Queue prefix:  ${QUEUE_PREFIX}"
echo -e "  API URL:       ${API_URL}"
echo -e "  Working dir:   ${WORKING_DIR}"
echo -e "  Logs dir:      ${LOGS_DIR}"
echo -e "  Timeout:       ${CLAUDE_TIMEOUT}s"
echo -e "  Allowed tools: ${ALLOWED_TOOLS}"
echo ""

# Array to store PIDs
declare -a PIDS=()

# Start each agent
for agent in $AGENTS; do
    QUEUE_PATH="${QUEUE_PREFIX}/${agent}"
    LOG_FILE="${LOGS_DIR}/${agent}.log"
    PID_FILE="${LOGS_DIR}/${agent}.pid"

    echo -e "${GREEN}Starting ${agent}...${NC}"
    echo -e "  Queue:    ${QUEUE_PATH}"
    echo -e "  Log file: ${LOG_FILE}"

    # Start task_loop in background
    nohup uv run python -u task_loop.py \
        --queue-path "$QUEUE_PATH" \
        --api-url "$API_URL" \
        --claude-timeout "$CLAUDE_TIMEOUT" \
        --allowed-tools "$ALLOWED_TOOLS" \
        --working-dir "$WORKING_DIR" \
        --name "$agent" \
        > "$LOG_FILE" 2>&1 &

    # Save PID
    AGENT_PID=$!
    echo $AGENT_PID > "$PID_FILE"
    PIDS+=($AGENT_PID)

    echo -e "  ${GREEN}✓${NC} Started (PID: ${AGENT_PID})"
    echo ""

    # Small delay between agent starts
    sleep 1
done

echo -e "${BLUE}================================${NC}"
echo -e "${GREEN}✅ All agents started!${NC}"
echo -e "${BLUE}================================${NC}"
echo ""
echo -e "${YELLOW}Agent PIDs:${NC}"
for agent in $AGENTS; do
    PID_FILE="${LOGS_DIR}/${agent}.pid"
    if [ -f "$PID_FILE" ]; then
        PID=$(cat "$PID_FILE")
        echo -e "  ${agent}: ${PID}"
    fi
done
echo ""

echo -e "${YELLOW}Useful commands:${NC}"
echo -e "  View all logs:     tail -f ${LOGS_DIR}/*.log"
echo -e "  View agent1 log:   tail -f ${LOGS_DIR}/agent1.log"
echo -e "  Stop all agents:   ./stop_agents.sh"
echo -e "  Check status:      ps aux | grep task_loop"
echo ""

echo -e "${GREEN}Agents are now running in the background!${NC}"
