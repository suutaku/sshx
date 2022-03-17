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

Server is not stable, just for testing. **Please use your own signaling server on production**.

## Install

### Signaling server
```bash
go get -u github.com/suutaku/sshx/cmd/signaling
```

### SSHX
```bash
go get -u github.com/suutaku/sshx/cmd/sshx
```

## Configuration
Configure file will created at first time at path: `$HOME/.sshx_config.json`.
Default configure as below:

```json
{
  "id": "dd88229c-ad13-4210-a1ad-3d59f12e0655",
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
* `locallistenaddr` : sshx listen address.
* `localsshaddr`: server sshd  listen address.
* `rtcconf`: STUN server configure.
* `signalingserveraddr`: signaling server address.

## Usage
* Signaling server
Specify server listening port by environment variable **PORT**, default **8080**.

```bash
export SSHX_SIGNALING_PORT=[port you want] #default port is 8080
signaling
```

* SSHX

Start sshx:

```bash
sshx -d
```
Connect to target device with devie ID:

```bash
sshx -t [your target device id] # tell sshx deamon target id
ssh -p 2222 [user]@127.0.0.1 # connect sshx deamon
```



