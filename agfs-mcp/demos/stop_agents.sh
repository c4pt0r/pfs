#!/bin/bash
# Stop all running task_loop agents

set -e

# Configuration
LOGS_DIR=${LOGS_DIR:-"./logs"}

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${BLUE}================================${NC}"
echo -e "${BLUE}🛑 Stopping AGFS Task Loop Agents${NC}"
echo -e "${BLUE}================================${NC}"
echo ""

if [ ! -d "$LOGS_DIR" ]; then
    echo -e "${YELLOW}No logs directory found. No agents to stop.${NC}"
    exit 0
fi

# Find all PID files
PID_FILES=$(find "$LOGS_DIR" -name "*.pid" 2>/dev/null)

if [ -z "$PID_FILES" ]; then
    echo -e "${YELLOW}No PID files found. No agents to stop.${NC}"
    exit 0
fi

# Stop each agent
STOPPED=0
FAILED=0

for PID_FILE in $PID_FILES; do
    AGENT_NAME=$(basename "$PID_FILE" .pid)

    if [ -f "$PID_FILE" ]; then
        PID=$(cat "$PID_FILE")

        echo -e "${YELLOW}Stopping ${AGENT_NAME} (PID: ${PID})...${NC}"

        # Check if process is running
        if ps -p $PID > /dev/null 2>&1; then
            # Try graceful shutdown first (SIGTERM)
            kill $PID 2>/dev/null || true
            sleep 1

            # Check if still running, force kill if needed
            if ps -p $PID > /dev/null 2>&1; then
                echo -e "  ${YELLOW}Forcing shutdown...${NC}"
                kill -9 $PID 2>/dev/null || true
            fi

            # Verify it's stopped
            if ! ps -p $PID > /dev/null 2>&1; then
                echo -e "  ${GREEN}✓${NC} Stopped successfully"
                ((STOPPED++))
            else
                echo -e "  ${RED}✗${NC} Failed to stop"
                ((FAILED++))
            fi
        else
            echo -e "  ${YELLOW}⚠${NC}  Process not running"
        fi

        # Remove PID file
        rm -f "$PID_FILE"
    fi
done

echo ""
echo -e "${BLUE}================================${NC}"
if [ $FAILED -eq 0 ]; then
    echo -e "${GREEN}✅ All agents stopped (${STOPPED} stopped)${NC}"
else
    echo -e "${YELLOW}⚠️  Stopped ${STOPPED}, failed ${FAILED}${NC}"
fi
echo -e "${BLUE}================================${NC}"
echo ""
