#!/bin/bash
set -e
source ~/.bash_localrc
repo="github.com/antongulenko/http-isolation-proxy"
dir="$(go list -f {{.Dir}} $repo/services)"
test -n "$dir" || { echo "Failed to get repo dir"; exit 1; }
cd "$dir"/..
echo "Executing ./make-all.sh in `pwd`"
./make-all.sh
