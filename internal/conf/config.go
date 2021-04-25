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

func NewConfigure(path string) *Configure {
	var tmp Configure
	viper.SetConfigName(".sshx_config")
	viper.SetConfigType("json")
	viper.AddConfigPath(path)
	viper.WatchConfig()
	viper.OnConfigChange(func(e fsnotify.Event) {
		fmt.Println("Config file changed:", e.Name)
	})
	err := viper.ReadInConfig() // Find and read the config file
	if err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			// Config file not found; ignore error if desired
			bs, _ := json.Marshal(defaultConfig)
			viper.ReadConfig(bytes.NewBuffer(bs))
			log.Println(err)
			log.Print("Write config ...")
			viper.SafeWriteConfig()
		} else {
			panic(err)
		}
	}

	err = viper.Unmarshal(&tmp)
	if err != nil {
		panic(err)
	}
	//log.Println(tmp)
	return &tmp
}
