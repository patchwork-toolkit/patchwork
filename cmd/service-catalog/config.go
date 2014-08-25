package main

import (
	"encoding/json"
	"errors"
	"io/ioutil"
)

type Config struct {
	Name         string `json:"name"`
	DnssdEnabled bool   `json:"dnssdEnabled"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
	StaticDir    string `json:"staticDir"`
	Storage      string `json:"storage"`
}

var supportedBackends = map[string]bool{
	"memory": true,
}

func (self *Config) Validate() error {
	var err error
	if self.Host == "" || self.Port == 0 {
		err = errors.New("Empty host or port")
	}
	if !supportedBackends[self.Storage] {
		err = errors.New("Unsupported storage backend")
	}
	return err
}

func loadConfig(confPath string) (*Config, error) {
	file, err := ioutil.ReadFile(confPath)
	if err != nil {
		return nil, err
	}

	config := new(Config)
	err = json.Unmarshal(file, config)
	if err != nil {
		return nil, err
	}

	if err = config.Validate(); err != nil {
		return nil, err
	}
	return config, nil
}
