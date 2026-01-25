#!/bin/sh

set -o errexit
set -o nounset
set -o pipefail

echo "Verbose: ${VERBOSE}"
GO_FLAGS=
if [[ "${VERBOSE:-0}" = "1" ]]; then
  echo "Building with VERBOSE"
  GO_FLAGS="-x"
  set -o xtrace
fi

if [ -z "${PKG}" ]; then
    echo "PKG must be set"
    exit 1
fi
if [ -z "${ARCH}" ]; then
    echo "ARCH must be set"
    exit 1
fi
if [ -z "${VERSION}" ]; then
    echo "VERSION must be set"
    exit 1
fi

export CGO_ENABLED=0
export GOARCH="${ARCH}"
export GO111MODULE=on

(
  cd client
  npm install --loglevel=error --legacy-peer-deps
  npm run build
)

go install                                                         \
  ${GO_FLAGS}                                                    \
  -installsuffix "static"                                        \
  -ldflags "-X ${PKG}/pkg/version.VERSION=${VERSION}"            \
  ./...
