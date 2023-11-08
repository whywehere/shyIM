package main

import (
	"flag"
	"shyIM/config"
	"shyIM/pkg/db"
	"shyIM/pkg/etcd"
	"shyIM/pkg/mq"
	"shyIM/router"
)

func main() {
	var configPath string
	flag.StringVar(&configPath, "c", "./im.yaml", "config path")
	flag.Parse()
	if err := config.Init(configPath); err != nil {
		panic(err)
	}
	if err := db.InitDB(); err != nil {
		panic(err)
	}
	mq.InitMessageMQ(config.GlobalConfig.RabbitMQ.URL)

	go etcd.Start()

	go router.HTTPRouter()

	go router.WSRouter()
	select {}
}
