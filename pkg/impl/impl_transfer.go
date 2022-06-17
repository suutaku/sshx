package impl

import (
	"encoding/gob"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gorilla/mux"
	"github.com/schollz/progressbar/v3"
	"github.com/sirupsen/logrus"
	"github.com/suutaku/go-qrc/pkg/qrc"
	"github.com/suutaku/sshx/internal/utils"
	"github.com/suutaku/sshx/pkg/types"
)

const (
	TYPE_UPLOAD = iota
	TYPE_DOWNLOAD
)

type FileInfo struct {
	Name       string
	Size       int64
	OptionType int32
}

type Transfer struct {
	BaseImpl
	FilePath   string
	Upload     bool
	ServerAddr string
	ServerPort int32
	server     *http.Server
	Progress   chan float32
	Exit       chan error
}

func NewTransfer(hostId string, filePath string, upload bool) *Transfer {
	return &Transfer{
		BaseImpl: BaseImpl{
			HId: hostId,
		},
		FilePath: filePath,
		Upload:   upload,
	}
}

func (tr *Transfer) Init() {
	tr.Progress = make(chan float32)
	tr.Exit = make(chan error)
	tr.ServerPort = 14567
}

func (tr *Transfer) Code() int32 {
	return types.APP_TYPE_TRANSFER
}

func (tr *Transfer) Response() error {
	tr.lock.Lock()
	s, c := net.Pipe()
	tr.BaseImpl.conn = &c
	tr.lock.Unlock()
	go func() {
		// get file header
		var info FileInfo
		err := gob.NewDecoder(s).Decode(&info)
		if err != nil {
			logrus.Error(err)
			return
		}
		switch info.OptionType {
		case TYPE_DOWNLOAD:
			fInfo, err := os.Stat(tr.FilePath)
			if err != nil {
				logrus.Error(err)
				return
			}
			info.Size = fInfo.Size()
			err = gob.NewEncoder(s).Encode(&info)
			if err != nil {
				logrus.Error(err)
				return
			}
			file, err := os.Open(tr.FilePath)
			if err != nil {
				logrus.Error(err)
				return
			}
			defer file.Close()

			bar := progressbar.DefaultBytes(
				fInfo.Size(),
				"update",
			)
			io.Copy(io.MultiWriter(s, bar), file)
		case TYPE_UPLOAD:
			file, err := os.Create(filepath.Join(os.Getenv("HOME"), "Downloads", info.Name))
			if err != nil {
				logrus.Error(err)
				return
			}
			defer file.Close()
			bar := progressbar.DefaultBytes(
				info.Size,
				"download",
			)
			io.Copy(io.MultiWriter(file, bar), s)
		default:
			logrus.Error("invalid file option type for ", info.OptionType)
		}
		// progress

	}()
	return nil
}

func (tr *Transfer) Dial() error {

	return nil
}

func (tr *Transfer) Wait() error {
	logrus.Warn("waitting process")
	if tr.FilePath != "" {
		if tr.Upload { // upload case
			logrus.Warn("uploading ", tr.FilePath, " ...")
			fInfo, err := os.Stat(tr.FilePath)
			if err != nil {
				logrus.Error(err)
				return err
			}
			file, err := os.Open(tr.FilePath)
			if err != nil {
				return err
			}
			defer file.Close()
			info := FileInfo{
				Name:       filepath.Base(tr.FilePath),
				Size:       fInfo.Size(),
				OptionType: TYPE_UPLOAD,
			}

			err = gob.NewEncoder(tr.Conn()).Encode(info)
			if err != nil {
				return err
			}

			bar := progressbar.DefaultBytes(
				fInfo.Size(),
				"upload",
			)
			_, err = io.Copy(io.MultiWriter(tr.Conn(), bar), file)
			return err
		} else {
			logrus.Warn("downloading ", tr.FilePath, " ...")
			info := FileInfo{
				Name:       filepath.Base(tr.FilePath),
				OptionType: TYPE_UPLOAD,
			}
			err := gob.NewEncoder(tr.Conn()).Encode(info)
			if err != nil {
				return err
			}
			err = gob.NewDecoder(tr.Conn()).Decode(&info)
			if err != nil {
				return err
			}

			file, err := os.Create(filepath.Join(os.Getenv("HOME"), "Downloads", info.Name))
			if err != nil {
				logrus.Error(err)
				return err
			}
			defer file.Close()
			bar := progressbar.DefaultBytes(
				info.Size,
				"download",
			)
			_, err = io.Copy(io.MultiWriter(file, bar), tr.Conn())
			return err
		}
	}
	downUrl, _ := utils.MakeRandomStr(10)
	downUrl = "/" + downUrl
	upUrl, _ := utils.MakeRandomStr(10)
	upUrl = "/" + upUrl
	entryUrl := downUrl
	if tr.Upload {
		entryUrl, _ = utils.MakeRandomStr(10)
	}
	entryUrl = "/" + entryUrl
	tr.ServerAddr = fmt.Sprintf("http://%s:%d%s", utils.GetLocalIP(), tr.ServerPort, entryUrl)
	r := mux.NewRouter()

	logrus.Warn(downUrl, " ", upUrl, " ", entryUrl)

	r.HandleFunc(entryUrl, func(w http.ResponseWriter, r *http.Request) {
		page := `<html><form action="` + upUrl + `">
		<input type="file" name="xcontent" required>
		<button type="submit">送信する</button>
	</form></html>`
		w.Write([]byte(page))
	})

	r.HandleFunc(downUrl, func(w http.ResponseWriter, r *http.Request) {

		utils.PipeWR(*tr.conn, r.Body, *tr.conn, w)
		logrus.Debug("end of gorutine")
	})
	r.HandleFunc(upUrl, func(w http.ResponseWriter, r *http.Request) {
		logrus.Warn("start to upload ", r.FormValue("xcontent"))

		utils.PipeWR(*tr.conn, r.Body, *tr.conn, w)

		logrus.Debug("end of gorutine")
	})
	tr.server = &http.Server{Addr: fmt.Sprintf(":%d", tr.ServerPort), Handler: r}
	qrc.ShowQR(tr.ServerAddr, false)
	fmt.Println("")
	tr.server.ListenAndServe()
	return nil
}

func (tr *Transfer) Close() {
	if tr.server != nil {
		tr.server.Close()
	}
}
