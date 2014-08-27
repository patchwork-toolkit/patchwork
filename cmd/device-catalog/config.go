package main

import (
	"encoding/json"
	"errors"
	"io/ioutil"
)

type Config struct {
	Description    string           `json:"description"`
	Address        string           `json:"address"`
	Host           string           `json:"host"`
	Port           int              `json:"port"`
	DnssdEnabled   bool             `json:"dnssdEnabled"`
	StaticDir      string           `json:"staticDir"`
	Storage        string           `json:"storage"`
	ServiceCatalog []ServiceCatalog `json:"serviceCatalog"`
}

type ServiceCatalog struct {
	Discover bool
	Endpoint string
	Ttl      int
}

func (self *Config) Validate() error {
	if self.Host != "" && self.Port > 0 {
		return nil
	}
	return errors.New("Invalid config")
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
