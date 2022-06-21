package impl

import (
	"encoding/gob"
	"fmt"
	"os"

	"github.com/jedib0t/go-pretty/v6/list"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/sirupsen/logrus"
	"github.com/suutaku/sshx/pkg/types"
)

const (
	DISPLAY_TABLE = iota
	DISPLAY_TREE
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

func (stat *STAT) Dial() error {
	return nil
}

func (stat *STAT) Response() error {
	return nil
}

func (stat *STAT) ShowStatus(displayType int) {
	logrus.Debug("read from conn")
	var pld []types.Status
	err := gob.NewEncoder(stat.Conn()).Encode(&pld)
	if err != nil {
		logrus.Error(err)
		return
	}
	err = gob.NewDecoder(stat.Conn()).Decode(&pld)
	if err != nil {
		logrus.Error(err)
		return
	}
	switch displayType {
	case DISPLAY_TABLE:
		stat.showTable(pld)
	case DISPLAY_TREE:
		stat.showList(pld)
	}
}

func (stat *STAT) showTable(status []types.Status) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"#", "Pair ID", "Target ID", "Parent Pair ID", "Application", "Start At"})
	t.AppendSeparator()
	for k, v := range status {
		if v.ParentPairId == "" {
			v.ParentPairId = "NULL"
		}
		t.AppendRows([]table.Row{
			{k + 1, v.PairId, v.TargetId, v.ParentPairId, GetImplName(v.ImplType), v.StartTime.Format("2 Jan 2006 15:04:05")},
		})
	}
	t.AppendSeparator()
	t.Render()
}

func (stat *STAT) showList(status []types.Status) {
	l := list.NewWriter()
	l.SetStyle(list.StyleConnectedRounded)
	l.SetOutputMirror(os.Stdout)
	groups := make(map[string][]types.Status, 0)
	names := make(map[string]string, 0)
	for _, v := range status {
		if v.ParentPairId != "" {
			if groups[v.ParentPairId] == nil {
				groups[v.ParentPairId] = make([]types.Status, 0)
				names[v.ParentPairId] = GetImplName(v.ImplType)
			}
			groups[v.ParentPairId] = append(groups[v.ParentPairId], v)
		} else {
			if groups[v.PairId] == nil {
				groups[v.PairId] = make([]types.Status, 0)
			}
			names[v.PairId] = GetImplName(v.ImplType)
		}
	}
	for k, v := range groups {
		l.AppendItem(fmt.Sprintf("%s [%s]", k, names[k]))
		l.Indent()
		for _, c := range v {
			l.AppendItem(fmt.Sprintf("%s [%s]", c.PairId, GetImplName(c.ImplType)))
		}
		l.UnIndent()
	}
	l.Render()
}

func (stat *STAT) Close() {
	stat.BaseImpl.Close()
}
