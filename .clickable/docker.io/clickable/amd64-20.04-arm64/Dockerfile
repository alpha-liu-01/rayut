FROM docker.io/clickable/amd64-20.04-arm64
LABEL base_image_hash="sha256:8cfcaabc63c71fa27e130f4c8ea4079d59a99fb6eeab521f8ebcd674508c9a94"
ARG GOPATH="/home/alpha/.clickable/go"
ARG SDK_FRAMEWORK="ubuntu-sdk-20.04"
ARG CLICK_FRAMEWORK="ubuntu-sdk-20.04"
ARG CLICK_FRAMEWORK_BASE="20.04"
ARG APPARMOR_POLICY="20.04"
ARG QT_VERSION="5.12"
ARG ARCH="arm64"
ARG ARCH_TRIPLET="aarch64-linux-gnu"
ARG ARCH_RUST="aarch64-unknown-linux-gnu"
ARG NUM_PROCS="56"
ARG ROOT="/home/alpha/Documents/GitHub/rayut"
ARG BUILD_DIR="/home/alpha/Documents/GitHub/rayut/build/aarch64-linux-gnu/app"
ARG SRC_DIR="/home/alpha/Documents/GitHub/rayut/app"
ARG INSTALL_DIR="/home/alpha/Documents/GitHub/rayut/build/aarch64-linux-gnu/app/install"
ARG CLICK_LD_LIBRARY_PATH="/home/alpha/Documents/GitHub/rayut/build/aarch64-linux-gnu/app/install/lib/aarch64-linux-gnu"
ARG CLICK_PATH="/home/alpha/Documents/GitHub/rayut/build/aarch64-linux-gnu/app/install/lib/aarch64-linux-gnu/bin"
ARG CLICK_QML2_IMPORT_PATH="/home/alpha/Documents/GitHub/rayut/build/aarch64-linux-gnu/app/install/lib/aarch64-linux-gnu"

RUN echo set debconf/frontend Noninteractive | debconf-communicate && echo set debconf/priority critical | debconf-communicate
RUN apt-get update && apt-get install -y --force-yes --no-install-recommends ca-certificates curl && apt-get clean
RUN curl -fsSL -o /tmp/go.tgz https://go.dev/dl/go1.27.1.linux-amd64.tar.gz && echo "63d339f0da5ab53635a56f2490a7984dfe12dfcff22ad749f63edaf590168445  /tmp/go.tgz" | sha256sum -c - && rm -rf /usr/local/go && tar -C /usr/local -xzf /tmp/go.tgz && rm /tmp/go.tgz