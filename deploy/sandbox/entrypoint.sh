#!/bin/bash
set -euo pipefail

echo "[agentbox] run: ${AGENTBOX_RUN_ID}"

echo "${AGENTBOX_AGENT_FILE}" > /workspace/AGENTS.md

# Execute agent runtime
if command -v claude &> /dev/null; then
    claude -p --dangerously-skip-permissions \
        "Read AGENTS.md and execute the workflow. Write output to /workspace/output/"
elif command -v codex &> /dev/null; then
    codex -q "Read AGENTS.md and execute the workflow."
else
    echo "[agentbox] no agent runtime found"
    exit 1
fi

# Webhook callback
if [ -n "${AGENTBOX_WEBHOOK_URL:-}" ]; then
    curl -s -X POST "${AGENTBOX_WEBHOOK_URL}" \
        -H "Content-Type: application/json" \
        -d "{\"run_id\":\"${AGENTBOX_RUN_ID}\",\"status\":\"completed\"}"
fi
