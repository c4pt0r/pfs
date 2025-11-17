#!/bin/bash
# Stop task_loop agents running in tmux session

set -e

# Configuration
SESSION_NAME=${SESSION_NAME:-"agfs-agents"}

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

# Check if tmux is installed
if ! command -v tmux &> /dev/null; then
    echo -e "${RED}Error: tmux is not installed${NC}"
    exit 1
fi

# Check if session exists
if tmux has-session -t "$SESSION_NAME" 2>/dev/null; then
    echo -e "${YELLOW}Found tmux session: ${SESSION_NAME}${NC}"

    # List panes before killing
    echo -e "${BLUE}Active panes:${NC}"
    tmux list-panes -t "$SESSION_NAME" -F "  Pane #{pane_index}: #{pane_current_command}" 2>/dev/null || true
    echo ""

    # Kill the session
    echo -e "${YELLOW}Killing tmux session: ${SESSION_NAME}${NC}"
    tmux kill-session -t "$SESSION_NAME"

    echo -e "${GREEN}✅ Tmux session stopped${NC}"
else
    echo -e "${YELLOW}No tmux session found with name: ${SESSION_NAME}${NC}"
fi

# Check for any stray task_loop.py processes
echo ""
echo -e "${BLUE}Checking for stray task_loop.py processes...${NC}"

# Find task_loop.py processes (excluding grep itself)
STRAY_PIDS=$(ps aux | grep '[t]ask_loop.py' | awk '{print $2}' || true)

if [ -n "$STRAY_PIDS" ]; then
    echo -e "${YELLOW}Found stray task_loop.py processes:${NC}"
    ps aux | grep '[t]ask_loop.py' | awk '{print "  PID: " $2 " - " $11 " " $12 " " $13}'
    echo ""
    echo -e "${YELLOW}Killing stray processes...${NC}"
    echo "$STRAY_PIDS" | xargs kill 2>/dev/null || true
    sleep 1

    # Check if any are still running
    REMAINING=$(ps aux | grep '[t]ask_loop.py' | awk '{print $2}' || true)
    if [ -n "$REMAINING" ]; then
        echo -e "${RED}Some processes didn't stop, using kill -9...${NC}"
        echo "$REMAINING" | xargs kill -9 2>/dev/null || true
    fi

    echo -e "${GREEN}✅ Stray processes killed${NC}"
else
    echo -e "${GREEN}No stray processes found${NC}"
fi

echo ""
echo -e "${BLUE}================================${NC}"
echo -e "${GREEN}✅ All agents stopped${NC}"
echo -e "${BLUE}================================${NC}"
echo ""
