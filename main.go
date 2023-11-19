package main

import (
	"flag"
	"fmt"
	"shyIM/config"
	"shyIM/pkg/db"
	"shyIM/pkg/etcd"
	"shyIM/pkg/kafka"
	"shyIM/router"
)

func main() {
	var configPath string
	// 自定义配置文件路径
	flag.StringVar(&configPath, "c", "./im.yaml", "config path")
	flag.Parse()
	if err := config.Init(configPath); err != nil {
		panic(err)
	}
	if err := db.InitDB(); err != nil {
		panic(err)
	}
	kafka.InitMQ([]string{config.GlobalConfig.Kafka.URL})
	fmt.Println("kafka initialized successfully")
	go etcd.Start()

	go router.HTTPRouter()

	go router.WSRouter()

	select {}
}
