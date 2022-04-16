#!/bin/bash

go build -ldflags "-s -w" ./cmd/sshx
go build -ldflags "-s -w" ./cmd/signaling
echo "$1"
if [ "$1" = "install" ];then
  platform=`uname`
  echo "build for ${platform}"
  if [ "$platform" = "Linux" ];then
    cp ./sshx /usr/local/bin/
    cp ./scripts/sshx.service /etc/systemd/system/
    mkdir -p /etc/sshx
    cp -rf ./static /etc/sshx/noVNC
    systemctl enable sshx.service
    systemctl start sshx.service
  elif [ "$platform" = "Darwin" ];then
    cp ./sshx /usr/local/bin/
    cp ./scripts/com.sshx.sshxd.plist /Library/LaunchDaemons/
    mkdir -p /etc/sshx
    cp -rf ./static /etc/sshx/noVNC
    launchctl load /Library/LaunchDaemons/com.sshx.sshxd.plist
  else
    echo "TODO: ${platform}"
  fi

  if [ "$2" = "signaling" ];then
    if [ "$platform" = "Linux" ];then
      cp ./scripts/signaling /usr/local/bin/
      cp ./scripts/signaling.service /etc/systemd/system/
      systemctl enable signaling.service
      systemctl start signaling.service
    else
      echo "TODO: $platform"
    fi
   
  fi
fi

echo "Build successfully"