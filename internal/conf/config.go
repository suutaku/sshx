package conf

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/google/uuid"
	"github.com/pion/webrtc/v3"
	"github.com/spf13/viper"
	"log"
)

type Configure struct {
	Key                 string
	LocalSSHAddr        string
	LocalListenAddr     string
	FullNode            bool
	ID                  string
	SignalingServerAddr string
	RTCConf             webrtc.Configuration
}

type ConfManager struct {
	Conf  *Configure
	Viper *viper.Viper
}

var defaultConfig = Configure{
	LocalListenAddr:     "127.0.0.1:2222",
	LocalSSHAddr:        "127.0.0.1:22",
	FullNode:            false,
	ID:                  uuid.New().String(),
	Key:                 uuid.New().String(),
	SignalingServerAddr: "http://peer1.cotnetwork.com:8990",
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
}

func NewConfManager(path string) *ConfManager {
	var tmp Configure
	vp := viper.New()
	vp.SetConfigName(".sshx_config")
	vp.SetConfigType("json")
	vp.AddConfigPath(path)
	vp.WatchConfig()
	vp.OnConfigChange(func(e fsnotify.Event) {
		fmt.Println("Config file changed:", e.Name)
		err := vp.Unmarshal(&tmp)
		if err != nil {
			panic(err)
		}
	})
	err := vp.ReadInConfig() // Find and read the config file
	if err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found; ignore error if desired
			bs, _ := json.Marshal(defaultConfig)
			viper.ReadConfig(bytes.NewBuffer(bs))
			log.Print("Write config ...")
			vp.WriteConfig()
		} else {
			panic(err)
		}
	}

	err = vp.Unmarshal(&tmp)
	if err != nil {
		panic(err)
	}
	//log.Println(tmp)
	return &ConfManager{
		Conf:  &tmp,
		Viper: vp,
	}
}

func (cm *ConfManager) Set(key, value string) {
	cm.Viper.Set(key, value)
	log.Print("Write config ...")
	err := cm.Viper.Unmarshal(cm.Conf)
	if err != nil {
		panic(err)
	}
	cm.Viper.WriteConfig()
}
