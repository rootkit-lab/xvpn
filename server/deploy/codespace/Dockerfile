# Imagem de desenvolvimento do XCODESPACES (Fase 51).
# Build (raiz do monorepo):
#   docker build -f server/deploy/codespace/Dockerfile -t ihuull/codespace:1.98.2 .
# Sem Docker Engine, sem docker.sock, sem privileged.
FROM gitpod/openvscode-server:1.98.2

ARG GO_VER=1.25.0
ARG NODE_VER=22.18.0

USER root
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates curl git build-essential \
    && rm -rf /var/lib/apt/lists/* \
    && curl -fsSL "https://go.dev/dl/go${GO_VER}.linux-amd64.tar.gz" | tar -C /usr/local -xz \
    && curl -fsSL "https://nodejs.org/dist/v${NODE_VER}/node-v${NODE_VER}-linux-x64.tar.xz" \
      | tar -C /usr/local -xJ \
    && ln -s "/usr/local/node-v${NODE_VER}-linux-x64/bin/node" /usr/local/bin/node \
    && ln -s "/usr/local/node-v${NODE_VER}-linux-x64/bin/npm" /usr/local/bin/npm \
    && ln -s "/usr/local/node-v${NODE_VER}-linux-x64/bin/npx" /usr/local/bin/npx

ENV PATH="/usr/local/go/bin:/usr/local/node-v${NODE_VER}-linux-x64/bin:${PATH}" \
    GOPATH=/home/workspace/go \
    GOPROXY=https://proxy.golang.org,direct

COPY shared/vscode-theme /home/.openvscode-server/extensions/ihuull.theme-0.1.0
COPY server/deploy/codespace/machine-settings.json /home/.openvscode-server/data/Machine/settings.json
COPY server/deploy/codespace/gitignore-global /tmp/ihuull-gitignore
RUN mkdir -p /home/workspace/.config/git /home/.openvscode-server/data/Machine \
    && cp /tmp/ihuull-gitignore /home/workspace/.config/git/ignore \
    && printf '[core]\n\texcludesFile = ~/.config/git/ignore\n' > /home/workspace/.gitconfig \
    && chown -R 1000:1000 \
      /home/.openvscode-server/extensions/ihuull.theme-0.1.0 \
      /home/.openvscode-server/data \
      /home/workspace/.config \
      /home/workspace/.gitconfig

# A imagem-base injeta --without-connection-token no ENTRYPOINT; o proxy exige token.
USER 1000
ENTRYPOINT ["/home/.openvscode-server/bin/openvscode-server"]
