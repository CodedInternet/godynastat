#!/usr/bin/env bash

pushd "$(dirname "$0")" # change to the correct directory to minimise build

# Fetch dependencies for build
echo "Installing dependencies"
go get .

# we need to patch webrtc for arm builds
curl -o /go/src/github.com/keroserene/go-webrtc/webrtc-linux-arm.pc https://raw.githubusercontent.com/CodedInternet/go-webrtc/arm/webrtc-linux-arm.pc
curl -o /go/src/github.com/keroserene/go-webrtc/lib/libwebrtc-linux-arm-magic.a https://raw.githubusercontent.com/CodedInternet/go-webrtc/arm/lib/libwebrtc-linux-arm-magic.a

echo "Building"
go build

echo "Installing"
go install

echo "Copying config etc"
cp bbb_config.yaml run.sh $GOPATH/

#echo "Cleaning up"
popd # revert back to wherever we were before
#rm -rf $SRCDIR  # remove redundant src directory to reduce image size
