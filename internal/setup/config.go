package setup

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
	"github.com/tahardi/bearclave/tee"
)

type Config struct {
	Platform tee.Platform `mapstructure:"platform"`
	Enclave  Enclave      `mapstructure:"enclave"`
	Nonclave Nonclave     `mapstructure:"nonclave"`
	Proxy    Proxy        `mapstructure:"proxy"`
}

type Enclave struct {
	Addr    string `mapstructure:"addr"`
}

type Proxy struct {
	InAddr  string `mapstructure:"in_addr"`
	OutAddr string `mapstructure:"out_addr"`
}

type Nonclave struct {
	Measurement string `mapstructure:"measurement"`
}

func LoadConfig(configFile string) (*Config, error) {
	config := &Config{}
	if _, err := os.Stat(configFile); err != nil {
		return nil, fmt.Errorf("finding config %s: %w", configFile, err)
	}

	v := viper.New()
	v.SetConfigFile(configFile)

	ext := filepath.Ext(configFile)
	if ext != "" {
		v.SetConfigType(ext[1:]) // Remove the dot from the extension
	}

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	if err := v.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}
	return config, nil
}
