package impl

import (
	"io"
	"net"
	"reflect"
	"time"

	"github.com/suutaku/sshx/pkg/conf"
)

const flagLen = 8
const timeout = 30 * time.Second

// Impl represents an application implementation
type Impl interface {
	// Set implementation specifiy configure
	Init(ImplParam)
	// Return impl code
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
		Type: (appCode << flagLen) | optCode,
	}
}

func (cr *CoreRequest) GetAppCode() int32 {
	return cr.Type >> flagLen
}

func (cr *CoreRequest) GetOptionCode() int32 {
	return cr.Type & 0xff
}

// Response struct which recive from Local TCP listenner
type CoreResponse struct {
	Type    int32 // Request type defined on types
	PairId  []byte
	Status  int32
	Payload []byte
}

func NewCoreResponse(appCode, optCode int32) *CoreResponse {
	return &CoreResponse{
		Type: (appCode << flagLen) | optCode,
	}
}

func (cr *CoreResponse) GetAppCode() int32 {
	return cr.Type >> flagLen
}

func (cr *CoreResponse) GetOptionCode() int32 {
	return cr.Type & 0xff
}

var registeddApp = []Impl{
	&SshImpl{},
	&VNCImpl{},
	&ScpImpl{},
	&SfsImpl{},
	&ProxyImpl{},
	&StatImpl{},
}

func GetImpl(code int32) Impl {
	for _, v := range registeddApp {
		if v.Code() == code {
			return v
		}
	}
	return nil
}

func GetImplName(code int32) string {
	if t := reflect.TypeOf(GetImpl(code)); t.Kind() == reflect.Ptr {
		return "*" + t.Elem().Name()
	} else {
		return t.Name()
	}
}
