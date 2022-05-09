# SSHX

[![Build Status](https://travis-ci.com/suutaku/sshx.svg?branch=master)](https://travis-ci.com/suutaku/sshx)
[![Go Report Card](https://goreportcard.com/badge/github.com/suutaku/sshx)](https://goreportcard.com/report/github.com/suutaku/sshx)


ssh p2p tunneling service. An enhanced version of 
`https://github.com/nobonobo/ssh-p2p.git`.


## Connection sequence

```
.-----------.         .------.                  .----------------.                    .------.    .--------------.
|Impl Dialer|         |Node A|                  |Signaling server|                    |Node B|    |Impl Responser|
'-----------'         '------'                  '----------------'                    '------'    '--------------'
      |                  |                              |                                |               |        
      |connection request|                              |                                |               |        
      |----------------->|                              |                                |               |        
      |                  |                              |                                |               |        
      |                  |send signaling request (OFFER)|                                |               |        
      |                  |----------------------------->|                                |               |        
      |                  |                              |                                |               |        
      |                  |                              |         dispatch OFFER         |               |        
      |                  |                              |------------------------------->|               |        
      |                  |                              |                                |               |        
      |                  |                              |send signaling response (ANWSER)|               |        
      |                  |                              |<-------------------------------|               |        
      |                  |                              |                                |               |        
      |                  |       dispatch ANWSER        |                                |               |        
      |                  |<-----------------------------|                                |               |        
      |                  |                              |                                |               |        
      | wrap connection  |                              |                                |               |        
      |<-----------------|                              |                                |               |        
      |                  |                              |                                |               |        
      |                  |              establish connection (DATA CHANNEL)              |               |        
      |                  |-------------------------------------------------------------->|               |        
      |                  |                              |                                |               |        
      |                  |                              |                                |wrap connection|        
      |                  |                              |                                |-------------->|        
      |                  |                              |                                |               |        
      |                  |                        do response                            |               |        
      |<-------------------------------------------------------------------------------------------------|        
.-----------.         .------.                  .----------------.                    .------.    .--------------.
|Impl Dialer|         |Node A|                  |Signaling server|                    |Node B|    |Impl Responser|
'-----------'         '------'                  '----------------'                    '------'    '--------------'

```

## Backend protocol

* RTCDataChannel/WebRTC: [https://github.com/pion/webrtc/v3](https://github.com/pion/webrtc/v3)
* Signaling server: [http://peer1.xxxxxxxx.com:8990](http://peer1.xxxxx.com:8990)

Server is not stable, just for testing. **Please use your own signaling server on production**.

## Install

### Requirements

`https://github.com/go-vgo/robotgo #Requirements`

### Signaling server
```bash
go get -u github.com/suutaku/sshx/cmd/signaling
```

### SSHX
```bash
go get -u github.com/suutaku/sshx/cmd/sshx
```

### Install as a system daemon

#### Mac OSX & Linux

```bash
git clone https://github.com/suutaku/sshx
cd sshx
sudo ./build.sh install ## for sshx
sudo ./build.sh install signaling ## both sshx and signaling server
```

### Windows
I don't have Windows device so i don't know how to create and test install scripts, maybe some can write a script for windows user.


## Configuration
Configure file will created at first time at path: `$HOME/.sshx_config.json`. You can also set root path of sshx with `SSHX_HOME` environment value.
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
  "signalingserveraddr": "http://signalingserver.xxxxx.com:8990"
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
Usage: sshx COMMAND [arg...]

a webrtc based ssh remote toolbox
               
Commands:      
  daemon       launch a sshx daemon
  config       list configure informations
  connect      connect to remote host
  copy-id      copy public key to server
  copy         copy files or directory from/to remote host
  proxy        start proxy
  status       get status
  fs           sshfs filesystem
               
Run 'sshx COMMAND --help' for more information on a command.
```
Daemoon

```bash
sshx daemon
```
**Note:** befor you run any command of sshx, you must run sshx as a daemon first.

List configure informations

```bash
sshx list
```

Connect a remote device with ID or IP(domain)

```bash
Usage: sshx connect [ -X ] [ -i ] [ -p ] ADDR

connect to remote host

Arguments:
  ADDR                   remote target address [username]@[host]:[port]

Options:
  -X, --x11              using X11 opton, default false
  -i, --identification   a private path, default empty for ~/.ssh/id_rsa
  -p                     remote host port (default "22")
```
Copy a file or dierctory just like ssh does

```bash
Usage: sshx copy FROM TO

cpy files or directories to remote host

Arguments:
  FROM                   file or directory path which want to coy
  TO                     des path
```

Proxy

```bash
Usage: sshx proxy COMMAND [arg...]

manage proxy
               
Commands:      
  start        start a proxy
               
Run 'sshx proxy COMMAND --help' for more information on a command.
```

VNC

sshx contained a `noVNC` client which write with Javascript. To use client just access `http://vnc.sshx.wz` (not working with VPN environment) or `http://127.0.0.1` and input device ID in setting menu.

Copy ID

```bash
Usage: sshx copy-id ADDR

copy public key to server
               
Arguments:     
  ADDR         remote target address [username]@[host]:[port]
```

SSHFS

```bash
Usage: sshx fs COMMAND [arg...]

sshfs filesystem
               
Commands:      
  mount        mount a remote filesystem
  unmount      unmount a remote filesystem
               
Run 'sshx fs COMMAND --help' for more information on a command.
```

Status

Show current connections

## Appliction

Using sshx, you can write your own NAT-Traversal applications by implement `Impl` at `github.com/suutaku/sshx/pkg/impl`:

```golang
type Impl interface {
	// set implementation specifiy configure
	Init(ImplParam)

  // return the application code, see pkg/types/types.go
	Code() int32
	// Writer of dialer
	DialerWriter() io.Writer
	// Writer of responser
	ResponserWriter() io.Writer
	// Reader of dialer
	DialerReader() io.Reader
	// Reader of responser
	ResponserReader() io.Reader
	// Response of remote device call
	Response() error
	// Call remote device
	Dial() error
	// Close Impl connection
	Close()
	// Set pairId dynamiclly
	SetPairId(id string)
}
```

basically, `Impl` can acts as a `Dialer` or `Responser`. A `Dialer` send an connection request to local node to tell it which application will used for this connection. 
local node make a P2P connection to target device and `Responser` at target devie response your request. see more `pkg/impl/impl_ssh.go`.





Features

- [x] Connect devices directly like ssh client does
- [x] Private key loggin
- [x] X11 forwarding
- [x] Connect devices behind NAT
- [x] Copy file or directory like scp does
- [x] Custom device ID
- [x] Custom signaling server
- [x] Multiple connection with one remote device
- [x] A simple signaling server implementation
- [ ] Pure go (due the `github.com/go-vgo/robotgo`)
- [x] Lunux system service supporting
- [x] VS Code SSH remote suportting (use proxy way due the VS Code not an open source project)
- [x] VNC supporting (both vnc server and client)
- [x] Ssh-fs supporting


