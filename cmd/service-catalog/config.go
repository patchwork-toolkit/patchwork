package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"
)

type Config struct {
	Name         string        `json:"name"`
	DnssdEnabled bool          `json:"dnssdEnabled"`
	BindAddr     string        `json:"bindAddr"`
	BindPort     int           `json:"bindPort"`
	ApiLocation  string        `json:"apiLocation"`
	StaticDir    string        `json:"staticDir"`
	Storage      StorageConfig `json:"storage"`
}

type StorageConfig struct {
	Type string `json:"type"`
}

var supportedBackends = map[string]bool{
	"memory": true,
}

func (self *Config) Validate() error {
	var err error
	if self.BindAddr == "" || self.BindPort == 0 {
		err = fmt.Errorf("Empty host or port")
	}
	if !supportedBackends[self.Storage.Type] {
		err = fmt.Errorf("Unsupported storage backend")
	}
	if self.ApiLocation == "" {
		err = fmt.Errorf("apiLocation must be defined")
	}
	if self.StaticDir == "" {
		err = fmt.Errorf("staticDir must be defined")
	}
	if strings.HasSuffix(self.ApiLocation, "/") {
		err = fmt.Errorf("apiLocation must not have a training slash")
	}
	if strings.HasSuffix(self.StaticDir, "/") {
		err = fmt.Errorf("staticDir must not have a training slash")
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
