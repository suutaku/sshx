package conn

import (
	"encoding/gob"
	"fmt"
	"net"

	"github.com/sirupsen/logrus"
	"github.com/suutaku/sshx/pkg/impl"
	"github.com/suutaku/sshx/pkg/types"
)

const directPort = 8099

type DirectInfo struct {
	Id       int64
	ImplCode int32
	HostId   string
}

type DirectService struct {
	BaseConnectionService
}

func NewDirectService(id string) *DirectService {
	return &DirectService{
		BaseConnectionService: *NewBaseConnectionService(id),
	}
}

func (ds *DirectService) Start() error {
	ds.BaseConnectionService.Start()
	listenner, err := net.Listen("tcp", fmt.Sprintf(":%d", directPort))
	if err != nil {
		logrus.Error(err)
		panic(err)
	}

	go func() {
		logrus.Debug("runing status ", ds.running)
		for ds.running {
			sock, err := listenner.Accept()
			if err != nil {
				logrus.Error(err)
				continue
			}
			var info DirectInfo
			err = gob.NewDecoder(sock).Decode(&info)
			if err != nil {
				logrus.Error(err)
				continue
			}
			logrus.Debug("new direct info com ", info)
			imp := impl.GetImpl(info.ImplCode)
			poolId := types.NewPoolId(info.Id)
			// server reset direction
			conn := NewDirectConnection(imp, ds.Id(), info.HostId, *poolId, CONNECTION_DRECT_IN, &ds.CleanChan)
			conn.Conn = sock
			ds.AddPair(conn)
			err = conn.Response()
			if err != nil {
				logrus.Error(err)
				continue
			}
		}
	}()
	return nil
}

func (ds *DirectService) CreateConnection(sender impl.Sender, sock net.Conn, poolId types.PoolId) error {
	// client reset direction
	err := ds.BaseConnectionService.CreateConnection(sender, sock, poolId)
	if err != nil {
		return err
	}
	iface := sender.GetImpl()
	if iface == nil {
		return fmt.Errorf("unknown impl")

	}
	if !sender.Detach {
		iface.SetConn(sock)
	}
	pair := NewDirectConnection(iface, ds.Id(), iface.HostId(), poolId, CONNECTION_DRECT_OUT, &ds.CleanChan)
	err = pair.Dial()
	if err != nil {
		return err
	}
	logrus.Warn("direct add pair")
	err = ds.AddPair(pair)
	if err != nil {
		return err
	}
	if !sender.Detach {
		// fill pair id and send back the 'sender'
		sender.PairId = []byte(pair.poolId.String(pair.Direction()))
		go pair.ResponseTCP(sender)
	}
	return nil
}
