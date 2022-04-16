package impl

import (
	"net"

	"github.com/suutaku/sshx/pkg/conf"
)

// Impl represents an application implementation
type Impl interface {
	// set implementation specifiy configure
	Init(ImplParam)
	Code() int32
	// Get connnection
	Conn() *net.Conn
	// Response of remote device call
	Response() error
	// Call remote device
	Dial() error
	// Close Impl connection
	Close()
	// Set pairId dynamiclly
	SetPairId(id string)
}

// initial parameters
type ImplParam struct {
	Config conf.Configure
	HostId string    // Remote host id
	PairId string    // Connection pair ID using on close
	Conn   *net.Conn // Local connection handler
}

// Request struct which send to Local TCP listenner
type CoreRequest struct {
	Type    int32 // Request type defined on types
	PairId  []byte
	Payload []byte // Application specify payload
}

func NewCoreRequest(appCode, optCode int32) *CoreRequest {
	return &CoreRequest{
		Type: (appCode << 1) | optCode,
	}
}

func (cr *CoreRequest) GetAppCode() int32 {
	return cr.Type >> 1
}

func (cr *CoreRequest) GetOptionCode() int32 {
	return cr.Type & 0x01
}

// Response struct which recive from Local TCP listenner
type CoreResponse struct {
	Type   int32 // Request type defined on types
	PairId []byte
	Status int32
}

func NewCoreResponse(appCode, optCode int32) *CoreResponse {
	return &CoreResponse{
		Type: (appCode << 1) | optCode,
	}
}

func (cr *CoreResponse) GetAppCode() int32 {
	return cr.Type >> 1
}

func (cr *CoreResponse) GetOptionCode() int32 {
	return cr.Type & 0x01
}

var enabledApp = []Impl{
	&SshImpl{},
	&VNCImpl{},
	&ScpImpl{},
	&SfsImpl{},
	&ProxyImpl{},
}

func GetImpl(code int32) Impl {
	return enabledApp[code]
}
