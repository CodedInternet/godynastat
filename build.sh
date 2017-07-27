#!/usr/bin/env bash

pushd "$(dirname "$0")" # change to the correct directory to minimise build

# Fetch dependencies for build
echo "Installing dependencies"
go get -u github.com/keroserene/go-webrtc # takes ages so need to install it manually
dep ensure

echo "Building"
go build

echo "Installing"
go install

echo "Copying config etc"
cp run.sh bbb_config.yaml $GOPATH/

#echo "Cleaning up"
popd # revert back to wherever we were before
#rm -rf $SRCDIR  # remove redundant src directory to reduce image size
