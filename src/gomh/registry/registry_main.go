package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/tidwall/gjson"
	"gomh/registry/handler"
	"gomh/util/logger"
	"io/ioutil"
	"os"
)

var (
	confPath = flag.String("confpath", "raft.cfg", "configuration filepath")
)

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
	flag.Parse()

	//	cfg, err := LoadConfig("config/reg.conf")
	//	if err != nil {
	//		logger.LogErrorf("Program will exit while loading config failed.")
	//		os.Exit(1)
	//	}

	sv, err := handler.NewService(*confPath)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	err = sv.Start()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}