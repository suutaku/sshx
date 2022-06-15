package impl

import (
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"
	"github.com/suutaku/go-qrc/pkg/qrc"
	"github.com/suutaku/sshx/internal/utils"
	"github.com/suutaku/sshx/pkg/types"
)

type Transfer struct {
	BaseImpl
	filePath   string
	upload     bool
	serverAddr string
	serverPort int32
	server     *http.Server
}

func NewTransfer(hostId string, filePath string, upload bool) *Transfer {
	return &Transfer{
		BaseImpl: BaseImpl{
			HId: hostId,
		},
		filePath:   filePath,
		upload:     upload,
		serverPort: 14567,
	}
}

func (tr *Transfer) Code() int32 {
	return types.APP_TYPE_TRANSFER
}

func (tr *Transfer) Response() error {

	return nil
}

func (tr *Transfer) Dial() error {
	return nil
}

func (tr *Transfer) ShowQR() error {
	downUrl, _ := utils.MakeRandomStr(10)
	downUrl = "/" + downUrl
	upUrl, _ := utils.MakeRandomStr(10)
	upUrl = "/" + upUrl
	entryUrl := downUrl
	if tr.upload {
		entryUrl, _ = utils.MakeRandomStr(10)
	}
	entryUrl = "/" + entryUrl
	tr.serverAddr = fmt.Sprintf("http://%s:%d%s", utils.GetLocalIP(), tr.serverPort, entryUrl)
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
	tr.server = &http.Server{Addr: fmt.Sprintf(":%d", tr.serverPort), Handler: r}
	qrc.ShowQR(tr.serverAddr, false)
	fmt.Println("")
	tr.server.ListenAndServe()
	return nil
}

func (tr *Transfer) Close() {
	if tr.server != nil {
		tr.server.Close()
	}
}
