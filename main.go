package main

import (
	"flag"
)

var (
	configPathFlag string
)
func init()  {
	flag.StringVar(&configPathFlag, "c", "", "conf file path")
	flag.Parse()

	if configPathFlag == ""{
		configPathFlag = "config.json"
	}
}

func main()  {
	StartFileServer(configPathFlag)
}
