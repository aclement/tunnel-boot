#!/bin/bash

# call this with the version as first param (e.g. buildRelease.sh 0.0.1)

function build() {
  local platform=$1
  local architecture=$2
  local binary_name="${PLUGIN_NAME}-${platform}-${architecture}-${BUILD_VERSION}"
  if [ ${platform} == "windows" ]; then
    binary_name=${binary_name}.exe
  fi

  pushd ${GOPATH}/src/github.com/aclement/${PLUGIN_NAME}
    local version_flag="-X main.pluginVersion=${BUILD_VERSION}"
    GOOS=${platform} GOARCH=${architecture} go build -ldflags="${version_flag}" -o ${binary_name} \
      && echo "Built ${binary_name}"

  popd

  mv ${GOPATH}/src/github.com/aclement/${PLUGIN_NAME}/${binary_name} ${ARCHIVE_DIR}/
}

PLUGIN_NAME=tunnel-boot
BUILD_VERSION=$1
#"$(cat $VERSION_FILE)"
ARCHIVE_DIR=built-plugin

mkdir -p ${ARCHIVE_DIR}

build darwin amd64
build linux 386
build linux amd64
build windows 386
build windows amd64
