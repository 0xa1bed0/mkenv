FROM debian:bookworm-slim
SHELL ["/bin/bash", "-lc"]

ARG DEBIAN_FRONTEND=noninteractive

ARG USERNAME=dev
ARG USER_UID=1000
ARG USER_GID=1000

RUN apt-get update \
	&& apt-get install -y --no-install-recommends \
	ca-certificates \
	curl \
	git \
	zsh \
	openssh-client \
	less \
	build-essential \
	pkg-config \
	cmake \
  bison \
  bsdmainutils \
  tmux \
	&& rm -rf /var/lib/apt/lists/*

RUN curl -LO https://github.com/neovim/neovim/releases/download/v0.11.4/nvim-linux-arm64.tar.gz \
  && tar xzf nvim-linux-arm64.tar.gz \
  && mv nvim-linux-arm64 /opt/nvim \
  && ln -s /opt/nvim/bin/nvim /usr/local/bin/nvim \
  && rm nvim-linux-arm64.tar.gz

RUN groupadd --gid ${USER_GID} ${USERNAME} \
	&& useradd --uid ${USER_UID} --gid ${USER_GID} -m -s /usr/bin/zsh ${USERNAME}

USER ${USERNAME}
ENV HOME=/home/${USERNAME}

RUN echo '# ~/.zshrc (base)\n' > ~/.zshrc \
	&& echo 'export ZSH="$HOME/.oh-my-zsh"' >> ~/.zshrc \
	&& echo 'ZSH_THEME="robbyrussell"' >> ~/.zshrc \
	&& echo 'plugins=(git)' >> ~/.zshrc \
	&& echo 'source $ZSH/oh-my-zsh.sh' >> ~/.zshrc \
  && echo 'alias vim="nvim"' >> ~/.zshrc

RUN export RUNZSH=no; export CHSH=no; export KEEP_ZSHRC=yes; sh -c "$(curl -fsSL https://raw.githubusercontent.com/ohmyzsh/ohmyzsh/master/tools/install.sh)"

ENV NVM_DIR=$HOME/.nvm
RUN mkdir -p "$NVM_DIR" \
	&& curl -fsSL https://raw.githubusercontent.com/nvm-sh/nvm/v0.39.7/install.sh | bash \
	&& /bin/zsh -lc "source $NVM_DIR/nvm.sh && nvm install --lts && nvm alias default 'lts/*'"

ENV GVM_DIR=$HOME/.gvm
RUN curl -fsSL https://raw.githubusercontent.com/moovweb/gvm/master/binscripts/gvm-installer | bash \
  && source $GVM_DIR/scripts/gvm \
  && gvm install go1.25.3 -B \ 
  && gvm use go1.25.3 --default \
  && go install golang.org/x/tools/gopls@latest \
  && go install golang.org/x/tools/cmd/goimports@latest \
  && go install golang.org/x/lint/golint@latest \
  && go install go.uber.org/mock/mockgen@latest

RUN mkdir -p $HOME/.config/tmux/plugins/catppuccin
RUN git clone -b v2.1.3 https://github.com/catppuccin/tmux.git ~/.config/tmux/plugins/catppuccin/tmux

RUN mkdir -p $HOME/.config/nvim $HOME/.config/nvim
RUN mkdir -p $HOME/workspace 

USER ${USERNAME}
WORKDIR $HOME/workspace

CMD ["tmux"]
