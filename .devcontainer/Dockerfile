FROM mcr.microsoft.com/devcontainers/base:2-trixie

# Install Bun as vscode user
USER vscode

WORKDIR /workspace/main

# Install claude code
RUN curl -fsSL https://claude.ai/install.sh | bash

# go migration atlas installation
RUN curl -sSf https://atlasgo.sh | sh

# go orm sqlc installation
RUN curl -L "$(curl -s https://api.github.com/repos/sqlc-dev/sqlc/releases/latest | grep "browser_download_url.*$(uname -s | tr '[:upper:]' '[:lower:]')_$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/').tar.gz" | cut -d '"' -f 4)" | tar xz -C ~/.local/bin
