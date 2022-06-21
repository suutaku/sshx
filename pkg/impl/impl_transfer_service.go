package impl

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path"

	"encoding/base64"

	"github.com/andybalholm/brotli"
	"github.com/gorilla/mux"
	"github.com/schollz/progressbar/v3"
	"github.com/sirupsen/logrus"
	"github.com/suutaku/go-qrc/pkg/qrc"
	"github.com/suutaku/sshx/internal/utils"
	"github.com/suutaku/sshx/pkg/res"
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
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		logrus.Debug("ctr+c ", trs.PairId())
		sender := NewSender(trs, types.OPTION_TYPE_DOWN)
		sender.PairId = []byte(trs.PairId())
		sender.SendDetach()
		trs.Close()
	}()

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
		// 	page := `<html>
		// 	<form action="` + upUrl + `" method="post" enctype="multipart/form-data">
		// 	<input type="file" name="xcontent" required>
		// 	<button type="submit">送信する</button>
		// </form></html>`
		// 	w.Write([]byte(page))
		sDec, _ := base64.StdEncoding.DecodeString(res.UploadHeader)
		buf := bytes.NewBuffer(sDec)
		brh := brotli.NewReader(buf)

		sDec2, _ := base64.StdEncoding.DecodeString(res.UploaderFoot)
		buf2 := bytes.NewBuffer(sDec2)
		brf := brotli.NewReader(buf2)
		io.Copy(w, brh)
		w.Write([]byte(upUrl))
		io.Copy(w, brf)

		//w.Write([]byte(res.UploadHeader + upUrl + res.UploaderFoot))
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
		logrus.Debug("new update request come")
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
	defer trs.Close()
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
