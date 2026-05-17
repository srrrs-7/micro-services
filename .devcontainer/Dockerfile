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

# istioctl
ARG ISTIO_VERSION=1.29.2
RUN ARCH=$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/') \
    && curl -fsSLo /tmp/istioctl.tar.gz "https://github.com/istio/istio/releases/download/${ISTIO_VERSION}/istioctl-${ISTIO_VERSION}-linux-${ARCH}.tar.gz" \
    && tar -xzf /tmp/istioctl.tar.gz -C /usr/local/bin istioctl \
    && chmod +x /usr/local/bin/istioctl \
    && rm /tmp/istioctl.tar.gz

# ┌─────────────────────────────────────────────────────────────────┐
# │ Go toolchain — /usr/local/go に入れて devcontainer Feature と   │
# │ 同じパスに統一することで Feature による上書きを防ぐ              │
# └─────────────────────────────────────────────────────────────────┘
ARG GO_VERSION=1.26.3
RUN ARCH=$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/') \
    && curl -fsSLo /tmp/go.tar.gz "https://go.dev/dl/go${GO_VERSION}.linux-${ARCH}.tar.gz" \
    && rm -rf /usr/local/go \
    && tar -C /usr/local -xzf /tmp/go.tar.gz \
    && rm /tmp/go.tar.gz

ENV GOROOT="/usr/local/go"
ENV PATH="${GOROOT}/bin:/usr/local/bin:${PATH}"

USER vscode
WORKDIR /workspace/main

# Install claude code
RUN curl -fsSL https://claude.ai/install.sh | bash

# opencode
RUN curl -fsSL https://opencode.ai/install | bash

# go migration atlas installation
RUN curl -sSf https://atlasgo.sh | sh

# golangci-lint installation
RUN curl -sSfL https://golangci-lint.run/install.sh | sh -s -- -b ~/.local/bin

# go orm sqlc installation
RUN curl -L "$(curl -s https://api.github.com/repos/sqlc-dev/sqlc/releases/latest \
      | grep "browser_download_url.*$(uname -s | tr '[:upper:]' '[:lower:]')_$(uname -m \
      | sed 's/x86_64/amd64/;s/aarch64/arm64/').tar.gz" \
      | cut -d '"' -f 4)" \
    | tar xz -C ~/.local/bin

# 1. Define HOME explicitly
ENV HOME="/home/vscode"
# 2. Define GOPATH
ENV GOPATH="/home/vscode/go"
# 3. Now both variables can safely be appended to the PATH
ENV PATH="${GOPATH}/bin:${HOME}/.local/bin:${PATH}"

# protoc + Go plugins
ARG PROTOC_VERSION=28.3
ARG PROTOC_GEN_GO_VERSION=v1.34.2
ARG PROTOC_GEN_GO_GRPC_VERSION=v1.5.1
RUN ARCH=$(uname -m | sed 's/x86_64/x86_64/;s/aarch64/aarch_64/') \
    && curl -fsSLo /tmp/protoc.zip "https://github.com/protocolbuffers/protobuf/releases/download/v${PROTOC_VERSION}/protoc-${PROTOC_VERSION}-linux-${ARCH}.zip" \
    && unzip -oq /tmp/protoc.zip -d ~/.local/protoc \
    && ln -sf ~/.local/protoc/bin/protoc ~/.local/bin/protoc \
    && rm /tmp/protoc.zip

RUN GOBIN=$HOME/.local/bin go install google.golang.org/protobuf/cmd/protoc-gen-go@${PROTOC_GEN_GO_VERSION} \
    && GOBIN=$HOME/.local/bin go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@${PROTOC_GEN_GO_GRPC_VERSION}
