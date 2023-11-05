package main

import (
	"flag"
	"shyIM/config"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "c", "./im.yaml", "config path")
	flag.Parse()
	if err := config.Init(configPath); err != nil {
		panic(err)
	}
}
