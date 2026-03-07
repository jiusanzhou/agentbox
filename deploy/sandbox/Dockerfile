FROM node:22-alpine

# System deps (minimal)
RUN apk add --no-cache curl git jq bash

# Install claude-code globally
RUN npm i -g @anthropic-ai/claude-code@2.1.61 && npm cache clean --force

# Setup user and workspace
RUN adduser -D -s /bin/bash agent && \
    mkdir -p /workspace/output && \
    chown -R agent:agent /workspace

COPY entrypoint.sh /opt/agentbox/entrypoint.sh
RUN chmod +x /opt/agentbox/entrypoint.sh

USER agent
WORKDIR /workspace
ENTRYPOINT ["/opt/agentbox/entrypoint.sh"]
