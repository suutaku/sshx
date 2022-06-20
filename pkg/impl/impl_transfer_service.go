package impl

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path"

	"github.com/gorilla/mux"
	"github.com/schollz/progressbar/v3"
	"github.com/sirupsen/logrus"
	"github.com/suutaku/go-qrc/pkg/qrc"
	"github.com/suutaku/sshx/internal/utils"
	"github.com/suutaku/sshx/pkg/types"
)

type TransferService struct {
	BaseImpl
	ServerAddr string
	ServerPort int32
	server     *http.Server
	Upload     bool
	FilePath   string
	ShowQR     bool
	TmpPath    string
}

func NewTransferService(hostId string, filePath string, upload, qr bool) *TransferService {
	ret := &TransferService{
		BaseImpl: *NewBaseImpl(hostId),
		FilePath: filePath,
		Upload:   upload,
	}
	if (upload && filePath == "") || qr {
		ret.ShowQR = true
	}
	if !upload && filePath == "" {
		return nil
	}
	ret.ServerPort = 14567
	f, _ := os.MkdirTemp("", "sshx")
	ret.TmpPath = f
	return ret
}

func (trs *TransferService) Start() error {
	if !trs.ShowQR {
		logrus.Debug("not shown qr code")
		if trs.Upload { // upload case
			transfer := NewTransfer(trs.HostId(), trs.FilePath, true, nil)
			if transfer == nil {
				return fmt.Errorf("cannot create transfer")
			}
			defer transfer.Close()
			err := transfer.Preper()
			if err != nil {
				return err
			}
			sender := NewSender(transfer, types.OPTION_TYPE_UP)
			conn, err := sender.Send()
			if err != nil {
				return err
			}
			transfer.SetConn(conn)
			err = transfer.DoUpload(nil)
			if err != nil {
				return err
			}
			return nil

		} else {
			transfer := NewTransfer(trs.HostId(), trs.FilePath, false, nil)
			if transfer == nil {
				return fmt.Errorf("cannot create transfer")
			}
			defer transfer.Close()
			err := transfer.Preper()
			if err != nil {
				return err
			}
			sender := NewSender(transfer, types.OPTION_TYPE_UP)
			conn, err := sender.Send()
			if err != nil {
				return err
			}
			transfer.SetConn(conn)
			err = transfer.DoDownload(nil)
			return err
		}
	}
	r := mux.NewRouter()
	downUrl, _ := utils.MakeRandomStr(10)
	upUrl, _ := utils.MakeRandomStr(10)
	upUrlEntry, _ := utils.MakeRandomStr(10)
	entryUrl := downUrl
	entryType := TYPE_DOWNLOAD
	if trs.Upload {
		entryUrl = upUrlEntry
		entryType = TYPE_UPLOAD
	}
	trs.ServerAddr = fmt.Sprintf("http://%s:%d/%s", utils.GetLocalIP(), trs.ServerPort, entryUrl)
	logrus.Debug(trs.ServerAddr)

	r.HandleFunc("/"+upUrlEntry, func(w http.ResponseWriter, r *http.Request) {
		page := `<html>
		<form action="` + upUrl + `" method="post" enctype="multipart/form-data">
		<input type="file" name="xcontent" required>
		<button type="submit">送信する</button>
	</form></html>`
		w.Write([]byte(page))
	})
	r.HandleFunc("/"+downUrl, func(w http.ResponseWriter, r *http.Request) {

		tmpFilePath := path.Join(trs.TmpPath, path.Base(trs.FilePath))

		finfo, err := os.Stat(tmpFilePath)
		if os.IsNotExist(err) {
			logrus.Debug("tmp file ", tmpFilePath, " not exist")
			tf, err := os.Create(tmpFilePath)
			if err != nil {
				w.Write([]byte(err.Error()))
				return
			}
			defer tf.Close()

			transfer := NewTransfer(trs.HostId(), trs.FilePath, false, nil)
			if transfer == nil {
				w.Write([]byte("cannot create transfer"))
				return
			}
			defer transfer.Close()
			err = transfer.Preper()
			if err != nil {
				logrus.Error(err)
				return
			}
			sender := NewSender(transfer, types.OPTION_TYPE_UP)
			conn, err := sender.Send()
			if err != nil {
				logrus.Error(err)
				return
			}
			mw := io.MultiWriter(w, tf)
			transfer.SetConn(conn)
			err = transfer.DoDownload(mw)
			if err != nil {
				w.Write([]byte(err.Error()))
				return
			}

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
		transfer := NewTransfer(trs.HostId(), "", true, header)
		if transfer == nil {
			w.Write([]byte("cannot create transfer at /upload "))
			return
		}
		defer transfer.Close()
		err = transfer.Preper()
		if err != nil {
			logrus.Error(err)
			return
		}
		sender := NewSender(transfer, types.OPTION_TYPE_UP)
		conn, err := sender.Send()
		if err != nil {
			logrus.Error("sender: ", err)
			return
		}
		transfer.SetConn(conn)
		err = transfer.DoUpload(file)
		if err != nil {
			w.Write([]byte(err.Error()))
			return
		}

		msg := fmt.Sprintf("upload %s (%d) success!", header.Filename, header.Size)
		w.Write([]byte(msg))
		logrus.Debug("end of gorutine")
	})
	trs.server = &http.Server{Addr: fmt.Sprintf(":%d", trs.ServerPort), Handler: r}
	if entryType == TYPE_DOWNLOAD {
		fmt.Println("\nScan QR code to download file")
	} else {
		fmt.Println("\nScan QR code to upload file")
	}
	fmt.Printf("\n")
	qrc.ShowQR(trs.ServerAddr, false)
	fmt.Printf("\n\n\n")
	trs.server.ListenAndServe()
	return nil
}

func (trs *TransferService) Code() int32 {
	return types.APP_TYPE_TRANSFER_SERVICE
}

func (trs *TransferService) Close() {
	if trs.server != nil {
		trs.server.Close()
	}
	trs.BaseImpl.Close()
	if trs.conn != nil {
		trs.Conn().Close()
	}
}
