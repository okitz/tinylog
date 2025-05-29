FROM golang:1.24.3


RUN apt-get update && apt-get install -y \
    git nano sudo dpkg mosquitto-clients protobuf-compiler\
    && rm -rf /var/lib/apt/lists/*

# install TinyGo
ARG TINYGO_VERSION=0.37.0
ADD https://github.com/tinygo-org/tinygo/releases/download/v${TINYGO_VERSION}/tinygo_${TINYGO_VERSION}_amd64.deb /tmp/tinygo.deb
RUN dpkg -i /tmp/tinygo.deb && \
    rm /tmp/tinygo.deb

# Create a non-root user
ARG USERNAME=vscode
ARG USER_UID=1234
ARG USER_GID=$USER_UID

RUN groupadd --gid $USER_GID $USERNAME \
    && useradd --uid $USER_UID --gid $USER_GID -m $USERNAME \
    && echo $USERNAME ALL=\(root\) NOPASSWD:ALL > /etc/sudoers.d/$USERNAME \
    && chmod 0440 /etc/sudoers.d/$USERNAME

ENV GOPATH=/home/$USERNAME/go
ENV PATH=$PATH:$GOPATH/bin

USER $USERNAME

RUN mkdir -p $GOPATH/pkg/mod/cache/download && \
    go install github.com/aperturerobotics/protobuf-go-lite/cmd/protoc-gen-go-lite@latest

WORKDIR /app
