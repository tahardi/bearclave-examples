package setup

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/viper"
	"github.com/tahardi/bearclave"
)

type Config struct {
	Platform bearclave.Platform `mapstructure:"platform"`
	Enclave  Enclave            `mapstructure:"enclave"`
	Nonclave Nonclave           `mapstructure:"nonclave"`
	Proxy    Proxy              `mapstructure:"proxy"`
}

type Enclave struct {
	Network string `mapstructure:"network"`
	Addr    string `mapstructure:"addr"`
}

type Proxy struct {
	Network string `mapstructure:"network"`
	Addr    string `mapstructure:"addr"`
	Route   string `mapstructure:"route"`
}

type Nonclave struct {
	Measurement string `mapstructure:"measurement"`
	Route       string `mapstructure:"route"`
}

func LoadConfig(configFile string) (*Config, error) {
	config := &Config{}
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return nil, fmt.Errorf("config file %s does not exist", configFile)
	}

	v := viper.New()
	v.SetConfigFile(configFile)

	ext := filepath.Ext(configFile)
	if ext != "" {
		v.SetConfigType(ext[1:]) // Remove the dot from the extension
	}

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("error reading config file: %w", err)
	}

	if err := v.Unmarshal(config); err != nil {
		return nil, fmt.Errorf("error unmarshaling config: %w", err)
	}
	return config, nil
}
