package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jinzhu/copier"
	"github.com/spf13/viper"
)

type Config struct {
	Server struct {
		Address string `mapstructure:"address"`
		Prefix  string `mapstructure:"prefix"`
	} `mapstructure:"server"`

	Vrchat struct {
		Src string `mapstructure:"src"`
		Dst string `mapstructure:"dst"`
	} `mapstructure:"vrchat" copier:"-"`
}

const (
	CONFIG_ARG = "config"
)

var config *Config

func GetGlobalConfig() *Config {
	return config
}

func GetConfigFile() string {
	return viper.ConfigFileUsed()
}

func NewConfig(path, name, file string) *Config {
	if value, ok := os.LookupEnv(CONFIG_ARG); ok {
		viper.AddConfigPath(filepath.Join(value, path))
	} else {
		viper.AddConfigPath(filepath.Join(".", path))
	}

	viper.SetConfigName(name)
	viper.SetConfigType(file)

	viper.AutomaticEnv()
	if err := viper.ReadInConfig(); err != nil {
		panic(fmt.Errorf("failed on reading config file: %w", err))
	}

	cfg := &Config{}
	if err := viper.Unmarshal(cfg); err != nil {
		panic(fmt.Errorf("failed on reading config file: %w", err))
	}

	config = &Config{}
	copier.Copy(config, cfg)
	return cfg
}
