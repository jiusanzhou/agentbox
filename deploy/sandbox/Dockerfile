FROM node:22-slim

RUN apt-get update && apt-get install -y \
    git curl jq python3 python3-pip bash \
    && rm -rf /var/lib/apt/lists/*

# Install Claude Code (pin version to match local)
RUN npm i -g @anthropic-ai/claude-code@2.1.61

# Create non-root user
RUN useradd -m -s /bin/bash agent
RUN mkdir -p /workspace/output && chown -R agent:agent /workspace

COPY entrypoint.sh /opt/agentbox/entrypoint.sh
RUN chmod +x /opt/agentbox/entrypoint.sh

USER agent
WORKDIR /workspace

ENTRYPOINT ["/opt/agentbox/entrypoint.sh"]
