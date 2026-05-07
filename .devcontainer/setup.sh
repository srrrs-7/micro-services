#!/bin/bash
set -e

echo "🚀 Starting Dev Container setup..."

echo "👤 Current user:"
whoami

# Allow vscode to access the host Docker socket mounted at /var/run/docker.sock.
# The socket's GID is host-defined and won't match any group baked into the image,
# so detect it at runtime and add vscode to a group with that GID.
if [ -S /var/run/docker.sock ]; then
  SOCK_GID=$(stat -c '%g' /var/run/docker.sock)
  if ! getent group "$SOCK_GID" >/dev/null; then
    groupadd -g "$SOCK_GID" docker-host
  fi
  GROUP_NAME=$(getent group "$SOCK_GID" | cut -d: -f1)
  if ! id -nG vscode | tr ' ' '\n' | grep -qx "$GROUP_NAME"; then
    usermod -aG "$GROUP_NAME" vscode
    echo "🐳 Added vscode to group $GROUP_NAME (gid=$SOCK_GID) for docker.sock access"
  fi
fi

# Git hooks
make hooks

# init and execute personal setup script
if [ ! -f ".devcontainer/setup.personal.sh" ]; then
  cat << 'EOF' > .devcontainer/setup.personal.sh
#!/bin/bash
set -e

# Your personal setup steps here
EOF
  chmod +x .devcontainer/setup.personal.sh
fi
echo "🔧 Running personal setup..."
bash .devcontainer/setup.personal.sh

echo "✨ Dev Container setup completed successfully!"