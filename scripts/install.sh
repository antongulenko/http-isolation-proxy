#!/bin/bash
set -e
source ~/.bash_localrc
repo="github.com/antongulenko/http-isolation-proxy"
go get -d $repo
dir="$(go list -f {{.Dir}} $repo/services)"
test -n "$dir" || { echo "Failed to get repo dir"; exit 1; }
cd "$dir"/..
echo "Running gpm in `pwd`"
gpm
