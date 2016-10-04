#!/bin/bash

mkdir -p tmp/android_client
cd tmp/android_client
env GOOS=linux GOARCH=arm GOARM=7 go build -v github.com/awgh/hushcom/hushcom
