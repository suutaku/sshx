# SSHX

[![Build Status](https://travis-ci.com/suutaku/sshx.svg?branch=master)](https://travis-ci.com/suutaku/sshx)

ssh p2p tunneling service. An enhanced version of `https://github.com/nobonobo/ssh-p2p.git`.

## Connection sequence

```
ssh ---dial---> sshx client
sshx client <----negotiation----> sshx server
sshd <--dial--- sshx server
```

## Backend protocol

* RTCDataChannel/WebRTC: [https://github.com/pion/webrtc/v3](https://github.com/pion/webrtc/v3)
* Signaling server: [http://peer1.cotnetwork.com:8990](http://peer1.cotnetwork.com:8990)

Server is not stable, just for testing. **Please use your own signaling server in production**.

## Install

### Signaling server
```bash
go get -u github.com/suutaku/sshx/cmd/signaling
```

### SSHX
```bash
go get -u github.com/suutaku/sshx/cmd/sshx
```

## Configure
Configure file will created at first time at path: `$HOME/.sshx_config.json`.
Default configure as below:

```json
{
  "fullnode": false,
  "id": "dd88229c-ad13-4210-a1ad-3d59f12e0655",
  "key": "75943077-3df7-4885-83f0-ef4361a4252f",
  "locallistenaddr": "127.0.0.1:2222",
  "localsshaddr": "127.0.0.1:22",
  "rtcconf": {
    "iceservers": [
      {
        "urls": [
          "stun:stun.l.google.com:19302"
        ]
      }
    ]
  },
  "signalingserveraddr": "http://peer1.cotnetwork.com:8990"
}
```
* `fullnode`: set to **false**, node will runing only as a **client**,set to **true**, node will runing as both **server** and **client**.
* `locallistenaddr` : sshx listen address.
* `localsshaddr`: server sshd  listen address.
* `rtcconf`: STUN server configure.
* `signalingserveraddr`: signaling server address.

## Usage
### Signaling server
Specify server listening port by environment variable **PORT**, default **8080**.

```bash
export PORT=8990
signaling
```

### SSHX

At **server** mode

```bash
sshx
```
At **client** mode, specify target ID by **-t** option.

```
sshx -t [your target device id]
ssh -p 2222 [user]@127.0.0.1
```



