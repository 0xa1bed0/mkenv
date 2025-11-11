# ───────────────────────────────────────────
# SYSTEM BASE IMAGE (SECURITY-ALLOWED)
FROM debian:bookworm-slim

# ───────────────────────────────────────────
# ENVIRONMENT
ENV GVM_DIR=/home/dev/.gvm
ENV NVM_DIR=/home/dev/.nvm

# ───────────────────────────────────────────
# TMP BUILD TIME WORKDIR (root scope)
WORKDIR /tmp/build/root

# ───────────────────────────────────────────
# ROOT-LEVEL SETUP STEPS (exec form)
RUN ["groupadd","--gid","10000","dev"]
RUN ["useradd","--uid","10000","--gid","10000","-m","dev"]
RUN ["mkdir","-p","/home/dev/workspace"]
RUN ["chown","-R","dev:dev","/home/dev/workspace"]
RUN ["mkdir","-p","/home/dev/local/bin"]
RUN ["chown","-R","dev:dev","/home/dev/local/bin"]
RUN ["chsh","-s","/bin/zsh","dev"]
RUN ["apt-get","update"]
RUN ["apt-get","install","-y","--no-install-recommends","binutils","bison","bsdmainutils","build-essential","ca-certificates","curl","git","tar","tmux","zsh"]
RUN ["rm","-rf","/var/lib/apt/lists/*"]

# ───────────────────────────────────────────
# DEFAULT USER (NON-ROOT) — SECURITY REQUIREMENT
USER dev

# ───────────────────────────────────────────
# TMP BUILD TIME WORKDIR (User scope)
WORKDIR /tmp/build/user

# ───────────────────────────────────────────
# USER-LEVEL BUILD STEPS (exec form)
RUN ["/bin/bash","-lc","set -eo pipefail \nexport GOLANG_VERSION=go1.25.3 \ncurl -fsSL https://raw.githubusercontent.com/moovweb/gvm/master/binscripts/gvm-installer | bash || true\nif [ ! -s \"$GVM_DIR/scripts/gvm\" ]; then\n  echo \"gvm install failed\"; exit 1\nfi\n# gvm scripts sometimes assume looser shell settings,\n# so don't let a weird non-zero blow up the build here.\nset +e\nsource \"$GVM_DIR/scripts/gvm\"\nrc=$?\nset -e\nif [ $rc -ne 0 ]; then\n  echo 'failed to source gvm'; exit $rc\nfi\ngvm install $GOLANG_VERSION -B \ngvm use $GOLANG_VERSION --default \ngo install golang.org/x/tools/gopls@latest \ngo install golang.org/x/tools/cmd/goimports@latest \ngo install golang.org/x/lint/golint@latest \ngo install go.uber.org/mock/mockgen@latest"]
RUN ["/bin/bash","-lc","set -eo pipefail\nexport NODE_VERSION=20.0.0\ncurl -fsSL https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.7/install.sh | bash\nsource \"$NVM_DIR/nvm.sh\"\n\nguess=\"${NODE_VERSION#v}\"\nmajor=\"${guess%%.*}\"\n\ncase \"$NODE_VERSION\" in\n  lts/*)\n    nvm install \"$NODE_VERSION\"\n    ;;\n  *)\n    # try exact/partial match (e.g. 20 or 20.1)\n    resolved=$(nvm ls-remote --no-colors | awk '{print $1}' | sed 's/^v//' \\\n      | grep -E \"^${guess}(\\.|$)\" | tail -n1 || true)\n\n    # if nothing, fallback to same major\n    [ -z \"$resolved\" ] \u0026\u0026 resolved=$(nvm ls-remote --no-colors | awk '{print $1}' | sed 's/^v//' \\\n      | grep -E \"^${major}\\.\" | tail -n1)\n\n    [ -z \"$resolved\" ] \u0026\u0026 exit 1\n\n    NODE_VERSION=\"$resolved\"\n    nvm install \"v$NODE_VERSION\"\n    ;;\nesac\nnvm alias default \"v$NODE_VERSION\"\nnvm use default\n\t"]
RUN ["/bin/bash","-lc","\nexport RUNZSH=no\nexport CHSH=no \nexport KEEP_ZSHRC=yes\nexport HOME=/tmp\nsh -c \"$(curl -fsSL https://raw.githubusercontent.com/ohmyzsh/ohmyzsh/master/tools/install.sh)\"\nmv /tmp/.oh-my-zsh /home/dev/.oh-my-zsh"]
RUN ["curl","-fL","-o","nvim.tar.gz","https://github.com/neovim/neovim/releases/download/v0.11.4/nvim-linux-arm64.tar.gz"]
RUN ["mkdir","nvim"]
RUN ["tar","xzf","nvim.tar.gz","-C","nvim","--strip-components=1"]
RUN ["ls","-la"]
RUN ["mkdir","-p","/home/dev/.opt/nvim"]
RUN ["mv","nvim","/home/dev/.opt"]
RUN ["ln","-s","/home/dev/.opt/nvim/bin/nvim","/home/dev/local/bin/nvim"]
RUN ["rm","nvim.tar.gz"]
RUN ["mkdir","-p","/home/dev/.config/nvim"]
RUN ["mkdir","-p","/home/dev/.config/tmux/plugins/catppuccin"]
RUN ["git","clone","-b","v2.1.3","https://github.com/catppuccin/tmux.git","/home/dev/.config/tmux/plugins/catppuccin/tmux"]

# ───────────────────────────────────────────
# FILE APPENDS (MERGED, HEREDOC, APPEND-ONLY)
RUN ["/bin/sh","-lc","cat \u003e\u003e /home/dev/.mkenvrc \u003c\u003c\"MKENV_system_config\"\nexport MKENV_LOCAL_BIN=\"/home/dev/local/bin\"\nexport PATH=\"$PATH:$MKENV_LOCAL_BIN\"\nMKENV_system_config"]
RUN ["/bin/sh","-lc","cat \u003e\u003e /home/dev/.mkenvrc \u003c\u003c\"MKENV_lang_golang\"\n# Golang version manager start\n[ -s \"$GVM_DIR/scripts/gvm\" ] \u0026\u0026 . \"$GVM_DIR/scripts/gvm\"\n# Golang version manager end\nMKENV_lang_golang"]
RUN ["/bin/sh","-lc","cat \u003e\u003e /home/dev/.mkenvrc \u003c\u003c\"MKENV_lang_nodejs\"\n# Nodejs version manager start \n[ -s \"$NVM_DIR/nvm.sh\" ] \u0026\u0026 . \"$NVM_DIR/nvm.sh\"\n[ -s \"$NVM_DIR/bash_completion\" ] \u0026\u0026 . \"$NVM_DIR/bash_completion\"\n# Nodejs version manager end\nMKENV_lang_nodejs"]
RUN ["/bin/sh","-lc","cat \u003e\u003e /home/dev/.zshrc \u003c\u003c\"MKENV_zshrc\"\n[ -s \"/home/dev/.mkenvrc\" ] \u0026\u0026 . \"/home/dev/.mkenvrc\"\nMKENV_zshrc"]
RUN ["/bin/sh","-lc","cat \u003e\u003e /home/dev/.zshrc \u003c\u003c\"MKENV_ohmyzshrc\"\n# OhMyZsh config start \nexport ZSH=\"$HOME\"/.oh-my-zsh\nZSH_THEME=\"robbyrussell\"\nplugins=(git)\nsource $ZSH/oh-my-zsh.sh\n# OhMyZsh config end\nMKENV_ohmyzshrc"]
RUN ["/bin/sh","-lc","cat \u003e\u003e /home/dev/.mkenvrc \u003c\u003c\"MKENV_nvim_aliases\"\n# NVIM aliases start\nalias vim=\"nvim\"\nalias vi=\"nvim\"\n# NVIM aliases end\nMKENV_nvim_aliases"]
RUN ["/bin/sh","-lc","cat \u003e\u003e /home/dev/.mkenvrc \u003c\u003c\"MKENV_set_nvim_as_default_editor\"\n# NVIM start\nexport EDITOR=\"nvim\"\n# NVIM end\nMKENV_set_nvim_as_default_editor"]

# ───────────────────────────────────────────
# WORKDIR
WORKDIR /home/dev/workspace

# ───────────────────────────────────────────
# ENTRYPOINT (exec form)
ENTRYPOINT ["/usr/bin/tmux","-u"]

# ───────────────────────────────────────────
# AUDIT LABELS
LABEL mkenv.bricks="system/debian,lang/golang,lang/nodejs,shell/ohmyzsh,tools/nvim,tools/tmux"