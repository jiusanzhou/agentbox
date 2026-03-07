FROM python:3.12-slim
RUN apt-get update && apt-get install -y --no-install-recommends curl git jq && rm -rf /var/lib/apt/lists/*
RUN pip install --no-cache-dir goose-ai
RUN useradd -m -s /bin/bash agent && mkdir -p /workspace/output && chown -R agent:agent /workspace
COPY entrypoint.sh /opt/agentbox/entrypoint.sh
RUN chmod +x /opt/agentbox/entrypoint.sh
USER agent
WORKDIR /workspace
ENTRYPOINT ["/opt/agentbox/entrypoint.sh"]
