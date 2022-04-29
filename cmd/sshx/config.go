package main

import (
	"fmt"

	cli "github.com/jawher/mow.cli"
	"github.com/suutaku/sshx/pkg/conf"
)

func cmdGetConfig(cmd *cli.Cmd) {
	cmd.Spec = "[KEYS...]"
	keys := cmd.StringsArg("KEYS", nil, "get cofigure by key [[key1] [key2]],[key1.key2]. if key is empty, list all configure info")
	cmd.Action = func() {
		cm := conf.NewConfManager(getRootPath())
		if keys == nil || len(*keys) == 0 {
			cm.Show()
			return
		}
		for _, v := range *keys {
			res := cm.Viper.Get(v)
			fmt.Printf("%s:\t%#v\n", v, res)
		}
	}
}

func cmdSetConfig(cmd *cli.Cmd) {
	cmd.Spec = "KEY VALUE"
	key := cmd.StringArg("KEY", "", "configure key, [key] ]value], [key1.key2] [value]")
	value := cmd.StringArg("VALUE", "", "configure value")
	cmd.Action = func() {
		cm := conf.NewConfManager(getRootPath())
		if key == nil || *key == "" {
			return
		}
		if value == nil || *value == "" {
			return
		}
		cm.Set(*key, *value)
	}
}

func cmdConfig(cmd *cli.Cmd) {
	cmd.Command("set", "set configure with key value", cmdSetConfig)
	cmd.Command("get", "get configure value with key", cmdGetConfig)
}
