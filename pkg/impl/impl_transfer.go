package impl

import (
	"encoding/gob"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path"
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
	Ready      bool
}

type TransferStatus struct {
	Status int32
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
	ShowQR     bool
	TmpPath    string
}

func NewTransfer(hostId string, filePath string, upload, qr bool) *Transfer {
	ret := &Transfer{
		BaseImpl: BaseImpl{
			HId: hostId,
		},
		FilePath: filePath,
		Upload:   upload,
	}
	if (upload && filePath == "") || qr {
		ret.ShowQR = true
	}
	if !upload && filePath == "" {
		return nil
	}
	return ret
}

func (tr *Transfer) Init() {
	tr.Progress = make(chan float32)
	tr.Exit = make(chan error)
	tr.ServerPort = 14567
	f, _ := os.MkdirTemp("", "sshx")
	tr.TmpPath = f
}

func (tr *Transfer) Code() int32 {
	return types.APP_TYPE_TRANSFER
}

func (tr *Transfer) sendHeader() (FileInfo, error) {
	logrus.Warn("send header ", tr.FilePath)
	info := FileInfo{
		Name:       tr.FilePath,
		OptionType: TYPE_DOWNLOAD,
	}
	if tr.Upload {
		fInfo, err := os.Stat(tr.FilePath)
		if err != nil {
			logrus.Error(err)
			return info, err
		}
		info.OptionType = TYPE_UPLOAD
		info.Size = fInfo.Size()
		info.Name = filepath.Base(tr.FilePath)
		info.Ready = true
	}
	err := gob.NewEncoder(tr.Conn()).Encode(info)
	if err != nil {
		return info, err
	}
	logrus.Warn("send ping")
	err = gob.NewDecoder(tr.Conn()).Decode(&info)
	if err != nil {
		return info, err
	}
	logrus.Warn("send pong")
	return info, nil
}

func (tr *Transfer) recvHeader(conn net.Conn) (FileInfo, error) {
	info := FileInfo{}
	err := gob.NewDecoder(conn).Decode(&info)
	if err != nil {
		logrus.Error(err)
		return info, err
	}
	logrus.Warn("recv ping ", info.Name)
	if info.OptionType == TYPE_DOWNLOAD {
		tr.FilePath = info.Name
		fInfo, err := os.Stat(tr.FilePath)
		if err != nil {
			logrus.Error(err, tr.FilePath)
			return info, err
		}
		info.Size = fInfo.Size()
		info.Ready = true
	}
	err = gob.NewEncoder(conn).Encode(&info)
	if err != nil {
		logrus.Error(err)
		return info, err
	}
	logrus.Warn("recv pong")
	return info, nil
}
func (tr *Transfer) doResponse(s net.Conn) error {
	// get file header
	logrus.Debug("start transfer response get file header")
	info, err := tr.recvHeader(s)
	if err != nil {
		return err
	}
	if !info.Ready {
		return fmt.Errorf("remote not ready")
	}
	logrus.Debug("start transfer response get file header ok")
	switch info.OptionType {
	case TYPE_DOWNLOAD:
		logrus.Debug("response download")
		file, err := os.Open(tr.FilePath)
		if err != nil {
			logrus.Error(err)
			return err
		}
		defer file.Close()
		bar := progressbar.DefaultBytes(
			info.Size,
			"update",
		)
		_, err = io.Copy(io.MultiWriter(s, bar), file)
		s.Close()
		return err
	case TYPE_UPLOAD:
		logrus.Debug("response upload")
		file, err := os.Create(filepath.Join(os.Getenv("HOME"), "Downloads", info.Name))
		if err != nil {
			logrus.Error(err)
			return err
		}
		logrus.Debug("file created")
		defer file.Close()
		bar := progressbar.DefaultBytes(
			info.Size,
			"download",
		)
		logrus.Debug("start copy io")
		n, err := io.Copy(io.MultiWriter(file, bar), s)

		logrus.Debug("stop response upload ", err, n)
		return err
	default:
		logrus.Error("invalid file option type for ", info.OptionType)
	}
	// progress
	return nil

}

func (tr *Transfer) Response() error {
	s, c := net.Pipe()
	tr.lock.Lock()
	tr.BaseImpl.conn = &c
	tr.lock.Unlock()
	go func() {
		err := tr.doResponse(s)
		if err != nil {
			logrus.Error("do response ", err)
		}
	}()
	return nil
}

func (tr *Transfer) Dial() error {

	return nil
}

func (tr *Transfer) Wait() error {
	if !tr.ShowQR {
		info, err := tr.sendHeader()
		if err != nil {
			return err
		}
		if !info.Ready {
			return fmt.Errorf("remote not ready")
		}
		if tr.Upload { // upload case
			logrus.Warn("uploading ", tr.FilePath, " ...")
			file, err := os.Open(tr.FilePath)
			if err != nil {
				return err
			}
			defer file.Close()
			bar := progressbar.DefaultBytes(
				info.Size,
				"upload",
			)
			n, err := io.Copy(io.MultiWriter(tr.Conn(), bar), file)
			logrus.Debug("stop process upload ", err, n)
			// time.Sleep(5 * time.Second)

			return err
		} else {
			logrus.Warn("downloading ", tr.FilePath, " ...")
			file, err := os.Create(filepath.Join(os.Getenv("HOME"), "Downloads", filepath.Base(info.Name)))
			if err != nil {
				logrus.Error(err)
				return err
			}
			defer file.Close()
			bar := progressbar.DefaultBytes(
				info.Size,
				"download",
			)
			io.Copy(io.MultiWriter(file, bar), tr.Conn())
			return nil
		}
	}
	downUrl, _ := utils.MakeRandomStr(10)
	upUrl, _ := utils.MakeRandomStr(10)
	entryUrl := downUrl
	entryType := TYPE_DOWNLOAD
	if tr.Upload {
		entryUrl, _ = utils.MakeRandomStr(10)
		entryType = TYPE_UPLOAD
	}
	entryUrl = "/" + entryUrl
	tr.ServerAddr = fmt.Sprintf("http://%s:%d%s", utils.GetLocalIP(), tr.ServerPort, entryUrl)
	r := mux.NewRouter()

	// logrus.Warn(downUrl, " ", upUrl, " ", entryUrl)

	r.HandleFunc("/"+entryUrl, func(w http.ResponseWriter, r *http.Request) {
		page := `<html>
		<form action="` + upUrl + `" method="post" enctype="multipart/form-data">
		<input type="file" name="xcontent" required>
		<button type="submit">送信する</button>
	</form></html>`
		w.Write([]byte(page))
	})
	r.HandleFunc("/"+downUrl, func(w http.ResponseWriter, r *http.Request) {
		logrus.Warn("download request come")
		tmpFilePath := path.Join(tr.TmpPath, path.Base(tr.FilePath))

		finfo, err := os.Stat(tmpFilePath)
		if os.IsNotExist(err) {
			logrus.Debug("tmp file ", tmpFilePath, " not exist")
			tf, err := os.Create(tmpFilePath)
			if err != nil {
				w.Write([]byte(err.Error()))
				return
			}
			defer tf.Close()
			info := FileInfo{
				Name:       tr.FilePath,
				OptionType: TYPE_DOWNLOAD,
			}
			err = gob.NewEncoder(tr.Conn()).Encode(info)
			if err != nil {
				w.Write([]byte(err.Error()))
				return
			}
			err = gob.NewDecoder(tr.Conn()).Decode(&info)
			if err != nil {
				w.Write([]byte(err.Error()))
				return
			}
			if !info.Ready {
				w.Write([]byte("remote is not ready"))
				return
			}
			bar := progressbar.DefaultBytes(
				info.Size,
				"download",
			)
			mw := io.MultiWriter(tf, w, bar)
			io.Copy(mw, tr.Conn())
		} else {
			logrus.Debug("tmp file ", tmpFilePath, " already exist")
			tf, err := os.Open(tmpFilePath)
			if err != nil {
				w.Write([]byte(err.Error()))
				return
			}
			defer tf.Close()
			bar := progressbar.DefaultBytes(
				finfo.Size(),
				"download",
			)
			mw := io.MultiWriter(w, bar)
			io.Copy(mw, tf)
		}
		logrus.Debug("end of gorutine ", err)
	})
	r.HandleFunc("/"+upUrl, func(w http.ResponseWriter, r *http.Request) {
		file, header, err := r.FormFile("xcontent")
		if err != nil {
			w.Write([]byte(err.Error()))
			return
		}
		tr.FilePath = header.Filename

		info := FileInfo{
			Name:       header.Filename,
			Size:       header.Size,
			OptionType: TYPE_UPLOAD,
			Ready:      true,
		}
		err = gob.NewEncoder(tr.Conn()).Encode(info)
		if err != nil {
			w.Write([]byte(err.Error()))
			return
		}
		err = gob.NewDecoder(tr.Conn()).Decode(&info)
		if err != nil {
			w.Write([]byte(err.Error()))
			return
		}
		if !info.Ready {
			w.Write([]byte("remote is not ready"))
			return
		}

		bar := progressbar.DefaultBytes(
			info.Size,
			"upload",
		)

		mw := io.MultiWriter(tr.Conn(), bar)
		n, err := io.Copy(mw, file)

		if err != nil {
			w.Write([]byte(err.Error()))
			return
		}
		msg := fmt.Sprintf("upload %s (%d:%d) success!", info.Name, info.Size, n)
		w.Write([]byte(msg))
		logrus.Debug("end of gorutine")
	})
	tr.server = &http.Server{Addr: fmt.Sprintf(":%d", tr.ServerPort), Handler: r}
	if entryType == TYPE_DOWNLOAD {
		fmt.Println("\nScan QR code to download file")
	} else {
		fmt.Println("\nScan QR code to upload file")
	}
	fmt.Printf("\n")
	qrc.ShowQR(tr.ServerAddr, false)
	fmt.Printf("\n\n\n")
	tr.server.ListenAndServe()
	return nil
}

func (tr *Transfer) Close() {
	tr.BaseImpl.Close()
	if tr.server != nil {
		tr.server.Close()
	}
}
