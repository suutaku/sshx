#!/bin/bash

 platform=`uname`

if [ "$platform" = "Linux" ];then
  sudo apt install -y libx11-dev xorg-dev libxtst-dev
  sudo apt install -y xcb libxcb-xkb-dev x11-xkb-utils libx11-xcb-dev libxkbcommon-x11-dev libxkbcommon-dev
  sudo apt install -y xsel xclip
fi
echo "preper..."
go mod tidy
echo "build..."
go build -ldflags "-s -w" ./cmd/sshx
go build -ldflags "-s -w" ./cmd/signaling
echo "$1"
if [ "$1" = "install" ];then
  echo "build for ${platform}"
  if [ "$platform" = "Linux" ];then
    xhost +
    sudo cp ./sshx /usr/local/bin/
    sudo cp ./scripts/sshx.service /etc/systemd/system/
    sudo mkdir -p /etc/sshx
    sudo chmod -R 777 /etc/sshx
    sudo cp -rf ./static /etc/sshx/noVNC
    sudo  systemctl enable sshx.service
    sudo systemctl start sshx.service
  elif [ "$platform" = "Darwin" ];then
    sudo cp ./sshx /usr/local/bin/
    sudo cp ./scripts/com.sshx.sshxd.plist /Library/LaunchDaemons/
    sudo mkdir -p /etc/sshx
    sudo chmod -R 777 /etc/sshx
    sudo cp -rf ./static /etc/sshx/noVNC
    sudo launchctl load /Library/LaunchDaemons/com.sshx.sshxd.plist
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