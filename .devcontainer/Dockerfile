FROM mcr.microsoft.com/devcontainers/base:2-trixie

# docker CLI (server runs on the host; socket is mounted via compose.yaml)
RUN install -m 0755 -d /etc/apt/keyrings \
    && curl -fsSL https://download.docker.com/linux/debian/gpg -o /etc/apt/keyrings/docker.asc \
    && chmod a+r /etc/apt/keyrings/docker.asc \
    && echo "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.asc] https://download.docker.com/linux/debian trixie stable" \
        > /etc/apt/sources.list.d/docker.list \
    && apt-get update \
    && apt-get install -y --no-install-recommends docker-ce-cli \
    && rm -rf /var/lib/apt/lists/*

# kubectl (latest stable)
RUN ARCH=$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/') \
    && curl -fsSLo /usr/local/bin/kubectl "https://dl.k8s.io/release/$(curl -fsSL https://dl.k8s.io/release/stable.txt)/bin/linux/${ARCH}/kubectl" \
    && chmod +x /usr/local/bin/kubectl

# kind (Kubernetes IN Docker)
RUN ARCH=$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/') \
    && curl -fsSLo /usr/local/bin/kind "https://kind.sigs.k8s.io/dl/v0.24.0/kind-linux-${ARCH}" \
    && chmod +x /usr/local/bin/kind

USER vscode

WORKDIR /workspace/main

# Install claude code
RUN curl -fsSL https://claude.ai/install.sh | bash

# go migration atlas installation
RUN curl -sSf https://atlasgo.sh | sh

# golangci-lint installation
RUN curl -sSfL https://golangci-lint.run/install.sh  | sh -s -- -b ~/.local/bin

# go orm sqlc installation
RUN curl -L "$(curl -s https://api.github.com/repos/sqlc-dev/sqlc/releases/latest | grep "browser_download_url.*$(uname -s | tr '[:upper:]' '[:lower:]')_$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/').tar.gz" | cut -d '"' -f 4)" | tar xz -C ~/.local/bin