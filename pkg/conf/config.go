package conf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/google/uuid"
	"github.com/pion/webrtc/v3"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"github.com/suutaku/go-vnc/pkg/config"
)

type Configure struct {
	LocalSSHPort        int32
	LocalHTTPPort       int32
	LocalTCPPort        int32
	ID                  string
	SignalingServerAddr string
	RTCConf             webrtc.Configuration
	VNCConf             config.Configure
	VNCStaticPath       string
	ETHAddr             string
}

type ConfManager struct {
	Conf  *Configure
	Viper *viper.Viper
	Path  string
}

var defaultConfig = Configure{
	LocalHTTPPort:       80,
	LocalSSHPort:        22,
	LocalTCPPort:        2224,
	ID:                  uuid.New().String(),
	SignalingServerAddr: "http://140.179.153.231:11095",
	RTCConf: webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{
					"stun:stun.l.google.com:19302",
					"stun:stun1.l.google.com:19302",
					"stun:stun2.l.google.com:19302",
					"stun:stun3.l.google.com:19302",
					"stun:stun4.l.google.com:19302",
				},
			},
		},
	},
	VNCConf: config.DefaultConfigure,
}

func ClearKnownHosts(subStr string) {
	subStr = strings.Replace(subStr, "127.0.0.1", "[127.0.0.1]", 1)
	//[127.0.0.1]:2222
	fileName := os.Getenv("HOME") + "/.ssh/known_hosts"
	input, err := ioutil.ReadFile(fileName)
	if err != nil {
		logrus.Error(err)
		return
	}
	lines := strings.Split(string(input), "\n")
	var newLines []string
	for i, line := range lines {
		if strings.Contains(line, subStr) {
		} else {
			newLines = append(newLines, lines[i])
		}
	}
	output := strings.Join(newLines, "\n")
	err = ioutil.WriteFile(fileName, []byte(output), 0644)
	if err != nil {
		logrus.Error(err)
		return
	}
	//ioutil.WriteFile(fileName, []byte(res), 544)
}

func NewConfManager(homePath string) *ConfManager {
	var tmp Configure
	vp := viper.New()
	vp.SetConfigName(".sshx_config")
	vp.SetConfigType("json")
	vp.AddConfigPath(homePath)
	vp.WatchConfig()
	vp.OnConfigChange(func(e fsnotify.Event) {
		err := vp.Unmarshal(&tmp)
		if err != nil {
			logrus.Error(err)
			return
		}
		logrus.Infof("update configure file\n %#v\n", tmp)
		ClearKnownHosts(fmt.Sprintf("127.0.0.1:%d", tmp.LocalSSHPort))
	})
	err := vp.ReadInConfig() // Find and read the config file
	if err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found; ignore error if desired
			defaultConfig.VNCStaticPath = path.Join(homePath, "noVNC")
			bs, _ := json.MarshalIndent(defaultConfig, "", "  ")
			vp.ReadConfig(bytes.NewBuffer(bs))
			logrus.Print("Write config at 1 ...\n", string(bs))
			err = vp.WriteConfigAs(homePath + "/.sshx_config.json")
			if err != nil {
				logrus.Error(err)
				os.Exit(1)
			}
		} else {
			logrus.Error(err)
			os.Exit(1)
		}
	}

	err = vp.Unmarshal(&tmp)
	if err != nil {
		logrus.Error(err)
		os.Exit(1)
	}

	ClearKnownHosts(fmt.Sprintf("127.0.0.1:%d", tmp.LocalSSHPort))
	return &ConfManager{
		Conf:  &tmp,
		Viper: vp,
		Path:  homePath,
	}
}

func (cm *ConfManager) Set(key, value string) {
	logrus.Info("key/value", key, value)
	cm.Viper.Set(key, value)
	logrus.Infof("%#v\n", cm.Conf)
	err := cm.Viper.Unmarshal(cm.Conf)
	if err != nil {
		logrus.Error(err)
		return
	}
	err = cm.Viper.WriteConfig()
	if err != nil {
		logrus.Error(err)
		return
	}
	logrus.Print("Write config ...")
}

func (cm *ConfManager) Show() {
	bs, _ := json.MarshalIndent(cm.Conf, "", "  ")
	logrus.Info("read configure file at: ", cm.Path+"/.sshx_config.json")
	logrus.Info(string(bs))
}
