#!/bin/bash
set -euo pipefail

echo "[abox] run: ${AGENTBOX_RUN_ID}"
mkdir -p /workspace/output

echo "${AGENTBOX_AGENT_FILE}" > /workspace/AGENTS.md

if [ "${AGENTBOX_MODE:-run}" = "session" ]; then
    # Session mode: keep container alive

    mkdir -p /home/agent/bin
    export PATH="/home/agent/bin:$PATH"

    # If WebDAV bridge available, create helper tools
    if [ -n "${ABOX_WEBDAV_URL:-}" ]; then
        cat > /home/agent/bin/local-ls << 'HELPER'
#!/bin/bash
# List files on host via WebDAV
PATH_ARG="${1:-/}"
curl -s -X PROPFIND "${ABOX_WEBDAV_URL}${PATH_ARG}" -H "Depth: 1" 2>/dev/null | \
  grep -oP 'href>[^<]+' | sed 's/href>//' | tail -n +2
HELPER
        chmod +x /home/agent/bin/local-ls

        cat > /home/agent/bin/local-cat << 'HELPER'
#!/bin/bash
# Read a file from host via WebDAV
curl -s "${ABOX_WEBDAV_URL}${1}"
HELPER
        chmod +x /home/agent/bin/local-cat

        cat > /home/agent/bin/local-get << 'HELPER'
#!/bin/bash
# Download a file from host to workspace
SRC="$1"
DST="${2:-/workspace/$(basename $1)}"
curl -s "${ABOX_WEBDAV_URL}${SRC}" -o "${DST}"
echo "Downloaded: ${SRC} -> ${DST}"
HELPER
        chmod +x /home/agent/bin/local-get

        cat > /home/agent/bin/local-put << 'HELPER'
#!/bin/bash
# Upload a file from workspace to host via WebDAV
SRC="$1"
DST="$2"
curl -s -T "${SRC}" "${ABOX_WEBDAV_URL}${DST}"
echo "Uploaded: ${SRC} -> ${DST}"
HELPER
        chmod +x /home/agent/bin/local-put

        cat > /home/agent/bin/local-find << 'HELPER'
#!/bin/bash
# Search files on host (recursive PROPFIND)
SEARCH_PATH="${1:-/r0/}"
PATTERN="${2:-}"
curl -s -X PROPFIND "${ABOX_WEBDAV_URL}${SEARCH_PATH}" -H "Depth: infinity" 2>/dev/null | \
  grep -oP 'href>[^<]+' | sed 's/href>//' | { [ -n "$PATTERN" ] && grep "$PATTERN" || cat; } | head -100
HELPER
        chmod +x /home/agent/bin/local-find

        # Write usage guide
        cat > /workspace/LOCAL_FILES.md << GUIDE
# Local File Access

Your host's local files are available via WebDAV at: ${ABOX_WEBDAV_URL}

## Quick Commands

\`\`\`bash
# List files in a directory
local-ls /r0/

# Read a file
local-cat /r0/README.md

# Download a file to workspace
local-get /r0/src/main.go

# Upload from workspace to host
local-put ./output/result.md /r0/output/result.md

# Search files by pattern
local-find /r0/ ".go"
\`\`\`

## Direct curl access

\`\`\`bash
# Read file
curl ${ABOX_WEBDAV_URL}/r0/path/to/file

# Write file
curl -T local-file ${ABOX_WEBDAV_URL}/r0/path/to/file

# List directory (PROPFIND)
curl -X PROPFIND ${ABOX_WEBDAV_URL}/r0/ -H "Depth: 1"
\`\`\`

## Roots
Check available roots: curl ${ABOX_WEBDAV_URL}/
GUIDE

        echo "[abox] local file access enabled via WebDAV"
    fi

    echo "export PATH=/home/agent/bin:\$PATH" >> /home/agent/.bashrc
    echo "[abox] session ready"
    exec tail -f /dev/null
else
    # Run mode (one-shot)
    export CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS=1

    if command -v claude &> /dev/null; then
        if [ -n "${ANTHROPIC_API_KEY:-}" ] || [ -f "$HOME/.claude/settings.json" ]; then
            echo "[abox] executing with Claude Code..."
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
fi
