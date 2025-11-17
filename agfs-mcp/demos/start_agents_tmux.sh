#!/bin/bash
# Start multiple task_loop agents in tmux panes (10 panes in 1 window)

set -e

# Configuration
AGENTS=${AGENTS:-"agent1 agent2 agent3 agent4 agent5 agent6 agent7 agent8 agent9 agent10"}
QUEUE_PREFIX=${QUEUE_PREFIX:-"/queuefs"}
API_URL=${API_URL:-"http://localhost:8080"}
WORKING_DIR=${WORKING_DIR:-"."}
CLAUDE_TIMEOUT=${CLAUDE_TIMEOUT:-600}
ALLOWED_TOOLS=${ALLOWED_TOOLS:-"WebFetch,Read,Write,Bash,Glob,Grep,agfs"}
SESSION_NAME=${SESSION_NAME:-"agfs-agents"}

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${BLUE}================================${NC}"
echo -e "${BLUE}🚀 Starting AGFS Task Loop Agents in Tmux${NC}"
echo -e "${BLUE}================================${NC}"
echo ""

# Check if tmux is installed
if ! command -v tmux &> /dev/null; then
    echo -e "${RED}Error: tmux is not installed${NC}"
    echo "Please install tmux first:"
    echo "  macOS:   brew install tmux"
    echo "  Ubuntu:  sudo apt-get install tmux"
    exit 1
fi

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
echo -e "  Session name:  ${SESSION_NAME}"
echo ""

# Check if already inside tmux
if [ -n "$TMUX" ]; then
    echo -e "${RED}Error: You are already inside a tmux session${NC}"
    echo -e "${YELLOW}Please exit tmux first or run from outside tmux:${NC}"
    echo -e "  ${GREEN}exit${NC}  (or press Ctrl-b + d to detach)"
    echo ""
    echo -e "${YELLOW}Or if you want to force it, run:${NC}"
    echo -e "  ${GREEN}TMUX= ./start_agents.sh${NC}"
    exit 1
fi

# Kill existing session if it exists
if tmux has-session -t "$SESSION_NAME" 2>/dev/null; then
    echo -e "${YELLOW}Killing existing session: ${SESSION_NAME}${NC}"
    tmux kill-session -t "$SESSION_NAME"
fi

# Convert agents to array
AGENTS_ARRAY=($AGENTS)
TOTAL_AGENTS=${#AGENTS_ARRAY[@]}

echo -e "${GREEN}Creating tmux session with ${TOTAL_AGENTS} panes...${NC}"
echo ""

# Create session with first pane and start first agent
FIRST_AGENT="${AGENTS_ARRAY[0]}"
FIRST_QUEUE_PATH="${QUEUE_PREFIX}/${FIRST_AGENT}"
FIRST_LOG_FILE="${LOGS_DIR}/${FIRST_AGENT}.log"

echo -e "${GREEN}Creating pane 1 and starting ${FIRST_AGENT}${NC}"
tmux new-session -d -s "$SESSION_NAME" -n "agents"
tmux send-keys -t "$SESSION_NAME" "uv run python -u task_loop.py --queue-path \"$FIRST_QUEUE_PATH\" --api-url \"$API_URL\" --claude-timeout \"$CLAUDE_TIMEOUT\" --allowed-tools \"$ALLOWED_TOOLS\" --working-dir \"$WORKING_DIR\" --name \"$FIRST_AGENT\" 2>&1 | tee \"$FIRST_LOG_FILE\"" C-m

# Create remaining panes and start agents immediately
for i in $(seq 1 $((TOTAL_AGENTS - 1))); do
    agent="${AGENTS_ARRAY[$i]}"
    QUEUE_PATH="${QUEUE_PREFIX}/${agent}"
    LOG_FILE="${LOGS_DIR}/${agent}.log"

    echo -e "${GREEN}Creating pane $((i + 1)) and starting ${agent}${NC}"
    tmux split-window -t "$SESSION_NAME" -h
    tmux send-keys -t "$SESSION_NAME" "uv run python -u task_loop.py --queue-path \"$QUEUE_PATH\" --api-url \"$API_URL\" --claude-timeout \"$CLAUDE_TIMEOUT\" --allowed-tools \"$ALLOWED_TOOLS\" --working-dir \"$WORKING_DIR\" --name \"$agent\" 2>&1 | tee \"$LOG_FILE\"" C-m
    tmux select-layout -t "$SESSION_NAME" tiled
done

echo ""
echo -e "${BLUE}================================${NC}"
echo -e "${GREEN}✅ All ${TOTAL_AGENTS} agents started in tmux!${NC}"
echo -e "${BLUE}================================${NC}"
echo ""

echo -e "${YELLOW}Tmux commands:${NC}"
echo -e "  Attach to session:     ${GREEN}tmux attach -t ${SESSION_NAME}${NC}"
echo -e "  List panes:            ${GREEN}tmux list-panes -t ${SESSION_NAME}${NC}"
echo -e "  Kill session:          ${GREEN}tmux kill-session -t ${SESSION_NAME}${NC}"
echo ""
echo -e "${YELLOW}Inside tmux:${NC}"
echo -e "  Switch panes:          ${GREEN}Ctrl-b + Arrow keys${NC}"
echo -e "  Switch to pane:        ${GREEN}Ctrl-b + q + <number>${NC}"
echo -e "  Zoom pane:             ${GREEN}Ctrl-b + z${NC}  (toggle fullscreen)"
echo -e "  Sync all panes:        ${GREEN}Ctrl-b + Ctrl-Y${NC}  (同时给所有agents发命令)"
echo -e "  Detach:                ${GREEN}Ctrl-b + d${NC}"
echo ""
echo -e "${YELLOW}Logs:${NC}"
echo -e "  View all logs:         tail -f ${LOGS_DIR}/*.log"
echo -e "  View agent1 log:       tail -f ${LOGS_DIR}/agent1.log"
echo ""

echo -e "${GREEN}🎬 Now attaching to tmux session...${NC}"
echo -e "${YELLOW}   Press Ctrl-b + d to detach${NC}"
echo ""
sleep 2

# Attach to the session
tmux attach -t "$SESSION_NAME"
