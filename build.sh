#!/bin/bash

GOGO_PROTO=`go list -f '{{ .Dir }}' -m github.com/gogo/protobuf`/gogoproto
KAD_PROTO=$GOPATH/src
mkdir -p internal/proto

protoc \
-I=pkg/pb:${GOGO_PROTO} \
--gofast_out=plugins=grpc:internal/proto \
pkg/pb/*.proto 

echo "build proto successfully"

go build -ldflags "-s -w" ./cmd/sshx
go build -ldflags "-s -w" ./cmd/signaling

if [ "$1" = "install" ];then
  cp ./sshx /usr/local/bin/
  cp ./sshx.service /etc/systemd/system/
  systemctl enable sshx.service
  systemctl start sshx.service
  if [ "$2" = "signaling" ];then
    cp ./signaling /usr/local/bin/
    cp ./signaling.service /etc/systemd/system/
    systemctl enable signaling.service
    systemctl start signaling.service
  fi
fi

echo "Build successfully"