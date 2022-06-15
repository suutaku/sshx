package conn

import (
	"net"

	"github.com/suutaku/sshx/pkg/impl"
)

// a connection service interface
type ConnectionService interface {
	Start() error
	SetStateManager(*StatManager) error
	CreateConnection(impl.Sender, net.Conn) error
	DestroyConnection(impl.Sender) error
	AttachConnection(impl.Sender, net.Conn) error
	IsReady() bool
	Stop()
}
