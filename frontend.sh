#!/usr/bin/env bash

pushd /usr/src/app/

JSON=`curl -sL "https://api.github.com/repos/CodedInternet/godynastat/contents/frontend/"`
SHA=`echo $JSON | jq -r '.sha'`
#REPO=`echo $JSON| jq -r '.submodule_git_url'`
git clone https://github.com/CodedInternet/dynastat-frontend.git .
git checkout $SHA

popd
