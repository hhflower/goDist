package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/moxiaomomo/goDist/gateway/proxy"
	"github.com/moxiaomomo/goDist/util/logger"

	"github.com/tidwall/gjson"
)

// LoadConfig load proxy config
func LoadConfig(confPath string) (map[string]interface{}, error) {
	cfg, err := ioutil.ReadFile(confPath)
	if err != nil {
		fmt.Printf("LoadConfig failed, err:%s\n", err.Error())
		return nil, err
	}
	m, ok := gjson.Parse(string(cfg)).Value().(map[string]interface{})
	if !ok {
		return nil, errors.New("Parse config failed.")
	}
	logger.LogInfof("config:%+v\n", m)
	return m, nil
}

func main() {
	cfg, err := LoadConfig("config/gateway.conf")
	if err != nil {
		logger.LogErrorf("Program will exit while loading config failed.")
		os.Exit(1)
	}

	_ = fmt.Sprintf("%s:%s", cfg["listenhost"], cfg["listenport"])
	// handler.StartGatewayServer(listenHost)

	proxy := proxy.NewProxy("")
	proxy.Start()
}
