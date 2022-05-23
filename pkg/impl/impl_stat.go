package impl

import (
	"os"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/suutaku/sshx/pkg/types"
)

type STAT struct {
	BaseImpl
}

func NewSTAT() *STAT {
	return &STAT{}
}

func (stat *STAT) Code() int32 {
	return types.APP_TYPE_STAT
}

func (stat *STAT) Preper() error {
	return nil
}

func (stat *STAT) Dial() error {
	return nil
}

func (stat *STAT) Response() error {
	return nil
}

func (stat *STAT) ShowStatus(status []types.Status) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"#", "Pair ID", "Target ID", "Application", "Start At"})
	t.AppendSeparator()
	for k, v := range status {
		t.AppendRows([]table.Row{
			{k, v.PairId, v.TargetId, GetImplName(v.ImplType), v.StartTime.String()},
		})
	}
	t.AppendSeparator()
	t.Render()
}

func (stat *STAT) Close() {
	stat.BaseImpl.Close()
}
