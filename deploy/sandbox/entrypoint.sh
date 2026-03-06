#!/bin/bash
set -euo pipefail

echo "[abox] run: ${AGENTBOX_RUN_ID}"
mkdir -p /workspace/output

echo "${AGENTBOX_AGENT_FILE}" > /workspace/AGENTS.md

AGENTBOX_MODE="${AGENTBOX_MODE:-run}"

if [ "$AGENTBOX_MODE" = "session" ]; then
    echo "[abox] session mode: container staying alive"
    # Keep the container running for interactive use via docker exec
    exec tail -f /dev/null
fi

# Default: one-shot run mode
if command -v claude &> /dev/null; then
    if [ -n "${ANTHROPIC_API_KEY:-}" ] || [ -f "$HOME/.claude/settings.json" ]; then
        echo "[abox] executing with Claude Code..."
        export CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS=1
        claude -p --dangerously-skip-permissions \
            "Read /workspace/AGENTS.md and execute the workflow described in it. Write all output files to /workspace/output/" 2>&1 || true
    else
        echo "[abox] claude found but no API key configured"
        cat /workspace/AGENTS.md
    fi
else
    echo "[abox] no agent runtime"
    cat /workspace/AGENTS.md
fi

echo "[abox] done"
