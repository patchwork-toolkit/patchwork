package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strings"
)

type Config struct {
	Description    string           `json:"description"`
	PublicAddr     string           `json:"publicAddr"`
	BindAddr       string           `json:"bindAddr"`
	BindPort       int              `json:"bindPort"`
	DnssdEnabled   bool             `json:"dnssdEnabled"`
	StaticDir      string           `json:"staticDir"`
	ApiLocation    string           `json:"apiLocation"`
	Storage        StorageConfig    `json:"storage"`
	ServiceCatalog []ServiceCatalog `json:"serviceCatalog"`
}

type ServiceCatalog struct {
	Discover bool
	Endpoint string
	Ttl      int
}

type StorageConfig struct {
	Type string `json:"type"`
}

var supportedBackends = map[string]bool{
	"memory": true,
}

func (c *Config) Validate() error {
	var err error
	if c.BindAddr == "" && c.BindPort == 0 {
		err = fmt.Errorf("Empty host or port")
	}
	if !supportedBackends[c.Storage.Type] {
		err = fmt.Errorf("Unsupported storage backend")
	}
	if c.ApiLocation == "" {
		err = fmt.Errorf("apiLocation must be defined")
	}
	if c.StaticDir == "" {
		err = fmt.Errorf("staticDir must be defined")
	}
	if strings.HasSuffix(c.ApiLocation, "/") {
		err = fmt.Errorf("apiLocation must not have a training slash")
	}
	if strings.HasSuffix(c.StaticDir, "/") {
		err = fmt.Errorf("staticDir must not have a training slash")
	}
	for _, cat := range c.ServiceCatalog {
		if cat.Ttl <= 0 {
			err = fmt.Errorf("All ServiceCatalog entries should have TTL >= 0")
		}
	}
	return err
}

func loadConfig(path string) (*Config, error) {
	file, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	c := new(Config)
	err = json.Unmarshal(file, c)
	if err != nil {
		return nil, err
	}

	if err = c.Validate(); err != nil {
		return nil, err
	}
	return c, nil
}
