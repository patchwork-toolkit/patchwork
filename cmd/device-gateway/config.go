package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)

//
// Loads a configuration form a given path
//
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

	dir := filepath.Dir(confPath)
	devicesDir := filepath.Join(dir, "devices")
	if _, err = os.Stat(devicesDir); os.IsNotExist(err) {
		return nil, err
	}

	err = filepath.Walk(devicesDir, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() || !strings.HasSuffix(path, ".json") {
			return nil
		}
		if err != nil {
			return err
		}

		f, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}

		device := new(Device)
		err = json.Unmarshal(f, device)
		if err != nil {
			return err
		}
		config.Devices = append(config.Devices, *device)

		return nil
	})

	if err != nil {
		return nil, err
	}

	if err = config.Validate(); err != nil {
		return nil, err
	}
	return config, nil
}

//
// Main configuration container
//
type Config struct {
	Id         string                    `json:"id"`
	Name       string                    `json:"name"`
	PublicAddr string                    `json:"publicAddr"`
	StaticDir  string                    `json:"staticDir`
	Catalog    []Catalog                 `json:"catalog"`
	Protocols  map[ProtocolType]Protocol `json:"protocols"`
	Devices    []Device                  `json:"devices"`
}

// Validates the loaded configuration
func (self *Config) Validate() error {
	//if _, ok := self.Catalog[CatalogTypeLocal]; !ok {
	//	return fmt.Errorf("Catalog should contain local section")
	//}
	//TODO: add more validation rules here
	return nil
}

// Finds resource record by given resource id
func (self *Config) FindResource(resourceId string) (*Resource, bool) {
	for _, d := range self.Devices {
		for _, r := range d.Resources {
			if resourceId == d.ResourceId(r.Name) {
				return &r, true
			}
		}
	}
	return nil, false
}

//
// Catalog entry and types
//
type Catalog struct {
	Type     string `json:"type"`
	Discover bool   `json:"discover"`
	Endpoint string `json:"endpoint"`
}

type CatalogType string

const (
	CatalogTypeLocal  CatalogType = "local"
	CatalogTypeRemote CatalogType = "remote"
)

//
// Protocol entry and types
//
type Protocol struct {
	BindAddr string
	BindPort int
	Host     string
	Port     int
	Prefix   string
}

type ProtocolType string

const (
	ProtocolTypeUnknown ProtocolType = ""
	ProtocolTypeREST    ProtocolType = "REST"
	ProtocolTypeMQTT    ProtocolType = "MQTT"
)

//
// Device information container (has one or many resources)
//
type Device struct {
	Name        string
	Description string
	Meta        map[string]interface{}
	Ttl         int
	Resources   []Resource
}

func (device *Device) ResourceId(name string) string {
	return fmt.Sprintf("%s/%s", device.Name, name)
}

//
// Resource information container (belongs to device)
//
type Resource struct {
	Name           string
	Meta           map[string]interface{}
	Representation map[string]interface{}
	Protocols      []SupportedProtocol
	Agent          Agent
}

//
// Protocol supported by resource and its supported content-types/methods
//
type SupportedProtocol struct {
	Type         ProtocolType
	Methods      []string
	ContentTypes []string `json:"content-types"`
}

//
// Description of how to run an agent that communicates with hardware
//
type Agent struct {
	Type     ExecType
	Interval time.Duration
	Dir      string
	Exec     string
}

type ExecType string

const (
	// Executes, outputs data, exits
	ExecTypeTask ExecType = "task"
	// Executes periodically (see Interval)
	ExecTypeTimer ExecType = "timer"
	// Constantly running and emitting output
	ExecTypeService ExecType = "service"
)
