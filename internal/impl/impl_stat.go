package impl

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"net"
	"os"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/sirupsen/logrus"
	"github.com/suutaku/sshx/pkg/types"
)

type StatImpl struct {
	conn           *net.Conn
	localEntryAddr string
	pairId         string
}

func NewStatImpl() *StatImpl {
	return &StatImpl{}
}

func (stat *StatImpl) Init(param ImplParam) {
	stat.conn = param.Conn
	stat.localEntryAddr = fmt.Sprintf("127.0.0.1:%d", param.Config.LocalTCPPort)
}

func (stat *StatImpl) Close() {}

func (stat *StatImpl) Code() int32 {
	return types.APP_TYPE_STAT
}

func (stat *StatImpl) DialerWriter() io.Writer {
	return *stat.conn
}

func (stat *StatImpl) DialerReader() io.Reader {
	return *stat.conn
}

func (stat *StatImpl) ResponserWriter() io.Writer {
	return *stat.conn
}

func (stat *StatImpl) ResponserReader() io.Reader {
	return *stat.conn
}
func (stat *StatImpl) Response() error {
	return nil
}

func (stat *StatImpl) Dial() error {
	conn, err := net.Dial("tcp", stat.localEntryAddr)
	if err != nil {
		return err
	}
	req := NewCoreRequest(stat.Code(), types.OPTION_TYPE_STAT)

	logrus.Infof("%#v\n", req)

	if err := gob.NewEncoder(conn).Encode(req); err != nil {
		return err
	}
	logrus.Debug("StatImpl waitting TCP Response")

	resp := CoreResponse{}
	if err := gob.NewDecoder(conn).Decode(&resp); err != nil {
		return err
	}
	logrus.Debug("StatImpl TCP Response comming")
	pld := make([]types.Status, 0)
	gob.NewDecoder(bytes.NewBuffer(resp.Payload)).Decode(&pld)
	stat.ShowStatus(pld)
	return nil
}

func (stat *StatImpl) SetPairId(id string) {
	stat.pairId = id
}

func (stat *StatImpl) ShowStatus(status []types.Status) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"#", "Pair ID", "Target ID", "Application", "Start At"})
	t.AppendSeparator()
	for k, v := range status {
		if v.PairId == stat.pairId {
			continue
		}
		t.AppendRows([]table.Row{
			{k, v.PairId, v.TargetId, GetImplName(v.ImplType), v.StartTime.String()},
		})
	}
	t.AppendSeparator()
	t.Render()
}
