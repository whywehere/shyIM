package config

import (
	"github.com/spf13/viper"
	"time"
)

var GlobalConfig *Config

type Config struct {
	MySQL struct {
		DNS string `mapstructure:"dns"`
	} `mapstructure:"mysql"`

	Redis struct {
		Address  string `mapstructure:"address"`  // Redis 地址
		Password string `mapstructure:"password"` // Redis 认证密码
	} `mapstructure:"redis"`

	Etcd struct {
		Endpoints []string      `mapstructure:"endpoints"`
		Timeout   time.Duration `mapstructure:"timeout"`
	} `mapstructure:"etcd"`

	APP struct {
		IP             string `mapstructure:"ip"`
		HttpServerPort int    `mapstructure:"http_server_port"`
		RPCPort        int    `mapstructure:"rpc_port"`
		Salt           string `mapstructure:"salt"`
	} `mapstructure:"app"`
	JWT struct {
		SignKey    string        `mapstructure:"sign_key"`
		ExpireTime time.Duration `mapstructure:"expire_time"`
	} `mapstructure:"jwt"`
}

func Init(configPath string) error {
	viper.SetConfigFile(configPath)
	if err := viper.ReadInConfig(); err != nil {
		return err
	}
	GlobalConfig = new(Config)
	if err := viper.Unmarshal(GlobalConfig); err != nil {
		return err
	}
	return nil
}
