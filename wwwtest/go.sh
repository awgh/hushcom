#!/bin/bash

mkdir -p tmp/hushcom
mkdir -p tmp/hushcomd

mkdir -p tmp/hushcom2

cp configs/hushcomd/ratnet.ql tmp/hushcomd/ratnet.ql

cd tmp/hushcomd
go build github.com/awgh/hushcom/hushcomd
screen -dmSL server ./hushcomd

cd ../hushcom
cp -R ../../js ./
go build github.com/awgh/hushcom/hushcom
screen -dmSL client ./hushcom

cd ../hushcom2
cp -R ../../js ./
go build github.com/awgh/hushcom/hushcom
screen -dmSL client2 ./hushcom -dbfile=ratnet2.ql -p=20003

screen -r server
