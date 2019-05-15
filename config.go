package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"path"
	"strconv"
	"strings"
)

type Config struct {
	Port int `json:"port"`
	Dir string `json:"dir"`
	Peers []string `json:"peers"`
	Index int
}

func NewConfig(p string) *Config {
	ips := GetIp()
	bs,err := ioutil.ReadFile(p)
	if err != nil {
		log.Fatal("cannot read config file")
	}
	config := new(Config)
	err = json.Unmarshal(bs, config)
	if err != nil {
		log.Fatal("cannot parse config file")
	}

	//check dir
	if len(config.Dir) == 0{
		log.Fatal("cannot get dir")
	}
	if config.Dir[len(config.Dir) - 1] == '/' || config.Dir[len(config.Dir) - 1] == '\\'{
		config.Dir = config.Dir[:len(config.Dir) - 1]
	}
	//没有就创建该目录
	if err = EnsureDir(config.Dir); err != nil {
		log.Fatal("cannot create dir " + config.Dir)
	}

	//创建DB目录
	dbdir := path.Join(config.Dir, "db")
	if err = EnsureDir(dbdir); err != nil {
		log.Fatal("cannot create dir " + dbdir)
	}

	config.Index = -1
	//查询本机
	selfPort := strconv.Itoa(config.Port)
	if len(config.Peers) > 0{
		for i,v := range config.Peers {
			ip := strings.Split(v, ":")
			if ip[0] == "127.0.0.1" && ip[1] == selfPort{
				config.Index = i
				break
			} else if _, ok := ips[ip[0]]; ok && ip[1] == selfPort{
				config.Index = i
				break
			}
		}
		if config.Index == -1{
			log.Fatal("cannot find host")
		}
	} else {
		config.Index = 0
	}

	return config
}



