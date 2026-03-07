#!/bin/bash
set -euo pipefail

echo "[abox] run: ${AGENTBOX_RUN_ID}"
mkdir -p /workspace/output

echo "${AGENTBOX_AGENT_FILE}" > /workspace/AGENTS.md

if [ "${AGENTBOX_MODE:-run}" = "session" ]; then
    mkdir -p /home/agent/bin
    export PATH="/home/agent/bin:$PATH"

    # === WebDAV tools (local file access) ===
    if [ -n "${ABOX_WEBDAV_URL:-}" ]; then
        cat > /home/agent/bin/local-ls << 'HELPER'
#!/bin/bash
curl -s -X PROPFIND "${ABOX_WEBDAV_URL}${1:-/}" -H "Depth: 1" 2>/dev/null | grep -oP 'href>[^<]+' | sed 's/href>//' | tail -n +2
HELPER
        cat > /home/agent/bin/local-cat << 'HELPER'
#!/bin/bash
curl -s "${ABOX_WEBDAV_URL}${1}"
HELPER
        cat > /home/agent/bin/local-get << 'HELPER'
#!/bin/bash
DST="${2:-/workspace/$(basename $1)}"
curl -s "${ABOX_WEBDAV_URL}${1}" -o "${DST}" && echo "Downloaded: ${1} -> ${DST}"
HELPER
        cat > /home/agent/bin/local-put << 'HELPER'
#!/bin/bash
curl -s -T "${1}" "${ABOX_WEBDAV_URL}${2}" && echo "Uploaded: ${1} -> ${2}"
HELPER
        cat > /home/agent/bin/local-find << 'HELPER'
#!/bin/bash
curl -s -X PROPFIND "${ABOX_WEBDAV_URL}${1:-/r0/}" -H "Depth: infinity" 2>/dev/null | grep -oP 'href>[^<]+' | sed 's/href>//' | { [ -n "${2:-}" ] && grep "$2" || cat; } | head -100
HELPER
        chmod +x /home/agent/bin/local-*
        echo "[abox] local file access enabled via WebDAV"
    fi

    # === Tunnel tools (full local capabilities) ===
    if [ -n "${ABOX_TUNNEL_URL:-}" ]; then
        TUNNEL="${ABOX_TUNNEL_URL}"

        # --- Browser ---
        cat > /home/agent/bin/browser-open << 'HELPER'
#!/bin/bash
curl -s -X POST "${ABOX_TUNNEL_URL}/browser/navigate" -H "Content-Type: application/json" -d "{\"url\":\"$1\"}"
HELPER
        cat > /home/agent/bin/browser-screenshot << 'HELPER'
#!/bin/bash
OUT="${1:-/workspace/screenshot.png}"
curl -s -X POST "${ABOX_TUNNEL_URL}/browser/screenshot" -H "Content-Type: application/json" -d '{}' | node -e "const d=JSON.parse(require('fs').readFileSync('/dev/stdin','utf8'));require('fs').writeFileSync('$OUT',Buffer.from(d.image||'','base64'))" 2>/dev/null && echo "Screenshot: $OUT"
HELPER
        cat > /home/agent/bin/browser-content << 'HELPER'
#!/bin/bash
curl -s -X POST "${ABOX_TUNNEL_URL}/browser/content" -H "Content-Type: application/json" -d '{}'
HELPER
        cat > /home/agent/bin/browser-click << 'HELPER'
#!/bin/bash
curl -s -X POST "${ABOX_TUNNEL_URL}/browser/click" -H "Content-Type: application/json" -d "{\"selector\":\"$1\"}"
HELPER
        cat > /home/agent/bin/browser-type << 'HELPER'
#!/bin/bash
curl -s -X POST "${ABOX_TUNNEL_URL}/browser/type" -H "Content-Type: application/json" -d "{\"selector\":\"$1\",\"text\":\"$2\"}"
HELPER
        cat > /home/agent/bin/browser-eval << 'HELPER'
#!/bin/bash
curl -s -X POST "${ABOX_TUNNEL_URL}/browser/evaluate" -H "Content-Type: application/json" -d "{\"expression\":\"$1\"}"
HELPER
        cat > /home/agent/bin/browser-tabs << 'HELPER'
#!/bin/bash
curl -s "${ABOX_TUNNEL_URL}/browser/tabs"
HELPER

        # --- Shell ---
        cat > /home/agent/bin/remote-exec << 'HELPER'
#!/bin/bash
CWD="${2:-}"
BODY="{\"command\":\"$1\""
[ -n "$CWD" ] && BODY="$BODY,\"cwd\":\"$CWD\""
BODY="$BODY}"
curl -s -X POST "${ABOX_TUNNEL_URL}/shell/exec" -H "Content-Type: application/json" -d "$BODY"
HELPER

        # --- Clipboard ---
        cat > /home/agent/bin/clip-get << 'HELPER'
#!/bin/bash
curl -s "${ABOX_TUNNEL_URL}/clipboard/"
HELPER
        cat > /home/agent/bin/clip-set << 'HELPER'
#!/bin/bash
curl -s -X POST "${ABOX_TUNNEL_URL}/clipboard/" -H "Content-Type: application/json" -d "{\"text\":\"$1\"}"
HELPER

        # --- Notifications ---
        cat > /home/agent/bin/notify << 'HELPER'
#!/bin/bash
curl -s -X POST "${ABOX_TUNNEL_URL}/notify/send" -H "Content-Type: application/json" -d "{\"title\":\"${1:-ABox Agent}\",\"body\":\"${2:-Task done}\"}"
HELPER
        cat > /home/agent/bin/ask-user << 'HELPER'
#!/bin/bash
curl -s -X POST "${ABOX_TUNNEL_URL}/notify/ask" -H "Content-Type: application/json" -d "{\"title\":\"$1\",\"body\":\"$2\",\"buttons\":[\"Allow\",\"Deny\"]}"
HELPER

        # --- Search ---
        cat > /home/agent/bin/search-files << 'HELPER'
#!/bin/bash
curl -s -X POST "${ABOX_TUNNEL_URL}/search/files" -H "Content-Type: application/json" -d "{\"pattern\":\"$1\",\"dir\":\"${2:-/}\"}"
HELPER
        cat > /home/agent/bin/search-grep << 'HELPER'
#!/bin/bash
curl -s -X POST "${ABOX_TUNNEL_URL}/search/grep" -H "Content-Type: application/json" -d "{\"pattern\":\"$1\",\"dir\":\"${2:-/}\"}"
HELPER

        chmod +x /home/agent/bin/browser-* /home/agent/bin/remote-exec /home/agent/bin/clip-* /home/agent/bin/notify /home/agent/bin/ask-user /home/agent/bin/search-*

        # --- Generate capability doc ---
        cat > /workspace/CAPABILITIES.md << CAPDOC
# Local Capabilities

Your user has connected their local machine. You can interact with it.

## 🌐 Browser (user's Chrome)
\`\`\`bash
browser-open "https://github.com"      # Navigate to URL
browser-screenshot                       # Save screenshot to /workspace/screenshot.png
browser-screenshot /workspace/page.png   # Save to custom path
browser-content                          # Get page HTML, text, title, URL (JSON)
browser-click "button.submit"            # Click element by CSS selector
browser-type "#input" "hello"            # Type text into element
browser-eval "document.title"            # Execute JavaScript
browser-tabs                             # List open tabs
\`\`\`

## 💻 Shell (run commands on user's machine)
\`\`\`bash
remote-exec "ls -la" "/Users/zoe/projects"   # Run command in directory
remote-exec "git status" "/path/to/repo"     # Git operations
remote-exec "npm test" "/path/to/project"    # Run tests
\`\`\`

## 📋 Clipboard
\`\`\`bash
clip-get                    # Read clipboard content
clip-set "copied text"      # Write to clipboard
\`\`\`

## 🔔 Notifications
\`\`\`bash
notify "Title" "Task completed!"             # Desktop notification
ask-user "Permission" "Delete file X?"       # Ask user, returns {"answer":"Allow"} or {"answer":"Deny"}
\`\`\`

## 🔍 Search (fast local file search)
\`\`\`bash
search-files "*.go" "/Users/zoe/projects"    # Find files by pattern
search-grep "func Main" "/Users/zoe/projects" # Search file contents
\`\`\`

## 📁 Files (read/write local files)
\`\`\`bash
local-ls /r0/                   # List directory
local-cat /r0/src/main.go       # Read file
local-get /r0/data.csv          # Download to workspace
local-put ./result.md /r0/out/  # Upload to host
local-find /r0/ ".go"           # Search files
\`\`\`

## Guidelines
- Always read CAPABILITIES.md before using local tools
- Use \`ask-user\` before destructive operations (delete, overwrite)
- Use \`notify\` when long tasks complete
- Browser operations affect the user's real Chrome — be careful
- Shell commands run on the user's actual machine
CAPDOC

        echo "[abox] tunnel capabilities enabled"
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
