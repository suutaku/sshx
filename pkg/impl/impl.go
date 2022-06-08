package impl

import (
	"io"
	"net"
	"reflect"
	"time"
)

const flagLen = 8
const timeout = 30 * time.Second

// Impl represents an application implementation
type Impl interface {
	Init()
	// Return impl code
	Code() int32
	// Set connection for non-detach process
	SetConn(net.Conn)
	// Writer
	Writer() io.Writer
	// Reader of dialer
	Reader() io.Reader
	// Response of remote device call
	Response() error
	// Call remote device
	Dial() error
	// Preper
	Preper() error
	// Close Impl connection
	Close()
	// Get Host Id
	HostId() string
	PairId() string
	SetPairId(string)
	ParentId() string
	SetParentId(string)
}

var registeddApp = []Impl{
	&SSH{},
	&Proxy{},
	&SSHFS{},
	&SCP{},
	&VNCService{},
	&VNC{},
	&STAT{},
}

func GetImpl(code int32) Impl {

	for _, v := range registeddApp {
		if v.Code() == code {
			s := reflect.TypeOf(v).Elem()
			return reflect.New(s).Interface().(Impl)
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
