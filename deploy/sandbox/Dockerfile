FROM node:22-slim

RUN apt-get update && apt-get install -y \
    git curl jq python3 python3-pip \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /workspace

COPY entrypoint.sh /opt/agentbox/entrypoint.sh
RUN chmod +x /opt/agentbox/entrypoint.sh

ENTRYPOINT ["/opt/agentbox/entrypoint.sh"]
