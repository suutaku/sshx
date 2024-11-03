# SSHX

[![Build Status](https://travis-ci.com/suutaku/sshx.svg?branch=master)](https://travis-ci.com/suutaku/sshx)
[![Go Report Card](https://goreportcard.com/badge/github.com/suutaku/sshx)](https://goreportcard.com/report/github.com/suutaku/sshx)

SSH P2P tunneling service. An enhanced version of 
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

The server is not stable and just for testing. **Please use your own signaling server on production**.

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
I don't have a Windows device so I don't know how to create and test install scripts, maybe someone can write a script for Windows users.


## Configuration
Configure file will created for the first time at the path: `$HOME/.sshx_config.json`. You can also set the root path of SSHX with `SSHX_HOME` environment value.
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

* `locallistenaddr`: sshx listening address.
* `localsshaddr`: server sshd listening to address.
* `rtcconf`: STUN server configure.
* `signalingserveraddr`: signaling server address.

## Usage

### Signaling server

Specify server listening port by environment variable **PORT**, default **8080**.
```bash
export SSHX_SIGNALING_PORT=[port you want] #default port is 8080
signaling
```

### SSHX

<ul>
<li>Start sshx:
<pre><code>Usage: sshx COMMAND [arg...]

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
</code></pre></li>

<li>Daemon

<pre><code>sshx daemon
</code></pre>
<strong>Note:</strong> Before you run any command of sshx, you must run sshx as a daemon first.</li>

<li>List configure informations

<pre><code>sshx list
</code></pre></li>

<li>Connect a remote device with ID or IP(domain)

<pre><code>Usage: sshx connect [ -X ] [ -i ] [ -p ] ADDR

connect to remote host

Arguments:
  ADDR                   remote target address [username]@[host]:[port]

Options:
  -X, --x11              using X11 opton, default false
  -i, --identification   a private path, default empty for ~/.ssh/id_rsa
  -p                     remote host port (default "22")
</code></pre></li>

<li>Copy a file or directory just like ssh does

<pre><code>Usage: sshx copy FROM TO

copy files or directories to remote host

Arguments:
  FROM                   file or directory path which want to coy
  TO                     des path
</code></pre></li>

<li>Proxy

<pre><code>Usage: sshx proxy COMMAND [arg...]

manage proxy
               
Commands:      
  start        start a proxy
               
Run 'sshx proxy COMMAND --help' for more information on a command.
</code></pre>

<li>VNC

<p>sshx contained a <code>noVNC</code> client which write with Javascript. To use client just access <code>http://vnc.sshx.wz</code> (not working with VPN environment) or <code>http://127.0.0.1</code> and input device ID in setting menu.</p></li>

<li>Copy ID

<pre><code>Usage: sshx copy-id ADDR

copy public key to server
               
Arguments:     
  ADDR         remote target address [username]@[host]:[port]
</code></pre></li>

<li>SSHFS

<pre><code>Usage: sshx fs COMMAND [arg...]

sshfs filesystem
               
Commands:      
  mount        mount a remote filesystem
  unmount      unmount a remote filesystem
               
Run 'sshx fs COMMAND --help' for more information on a command.
</code></pre></li>

<li>Status

<p>Show current connections</p></li>
</ul>

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

Basically, `Impl` can acts as a `Dialer` or `Responser`. A `Dialer` sends a connection request to the local node to tell it which application will used for this connection. 

The local node makes a P2P connection to the target device and the `Responser` at the target device responds to your request. See more at `pkg/impl/impl_ssh.go`.

## Features

- [x] Connect devices directly like the SSH client does
- [x] Private key login
- [x] X11 forwarding
- [x] Connect devices behind NAT
- [x] Copy file or directory like scp does
- [x] Custom device ID
- [x] Custom signaling server
- [x] Multiple connection with one remote device
- [x] A simple signaling server implementation
- [ ] Pure go (due the `github.com/go-vgo/robotgo`)
- [x] Lunux system service supporting
- [x] VS Code SSH remote supporting (use proxy way due the VS Code not being an open source project)
- [x] VNC supporting (both vnc server and client)
- [x] SSH-FS supporting
