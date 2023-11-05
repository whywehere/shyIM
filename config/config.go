package config

import "github.com/spf13/viper"

var GlobalConfig *Config

type Config struct {
	MySQL struct {
		DNS string `mapstructure:"dns"`
	} `mapstructure:"mysql"`

	Redis struct {
		Address  string `mapstructure:"address"`  // Redis 地址
		Password string `mapstructure:"password"` // Redis 认证密码
	} `mapstructure:"redis"`
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
