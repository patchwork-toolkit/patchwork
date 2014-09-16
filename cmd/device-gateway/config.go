package main

import (
	"encoding/json"
	"errors"
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

	rawConfig := new(struct {
		*Config
		Protocols map[ProtocolType]json.RawMessage `json:"protocols"`
	})
	err = json.Unmarshal(file, rawConfig)
	if err != nil {
		return nil, err
	}
	config := rawConfig.Config
	config.Protocols = make(map[ProtocolType]interface{})

	// Parse config protocols
	for k, v := range rawConfig.Protocols {
		switch k {
		case ProtocolTypeREST:
			protoConf := RestProtocol{}
			err := json.Unmarshal(v, &protoConf)
			if err != nil {
				return nil, errors.New("Invalid config of REST protocol")
			}
			config.Protocols[ProtocolTypeREST] = protoConf

		case ProtocolTypeMQTT:
			protoConf := MqttProtocol{}
			err := json.Unmarshal(v, &protoConf)
			if err != nil {
				return nil, errors.New("Invalid config of MQTT protocol")
			}
			config.Protocols[ProtocolTypeMQTT] = protoConf
		}
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
	Id         string                       `json:"id"`
	Name       string                       `json:"name"`
	PublicAddr string                       `json:"publicAddr"`
	StaticDir  string                       `json:"staticDir`
	Catalog    []Catalog                    `json:"catalog"`
	Http       HttpConfig                   `json:"http"`
	Protocols  map[ProtocolType]interface{} `json:"protocols"`
	Devices    []Device                     `json:"devices"`
}

// Validates the loaded configuration
func (self *Config) Validate() error {
	// Check if HTTP is configured
	if self.Http.BindAddr == "" || self.Http.BindPort == 0 {
		return errors.New("Invalid config: HTTP has to be properly configured")
	}

	// Check if REST protocol is configured
	_, ok := self.Protocols[ProtocolTypeREST]
	if !ok {
		return errors.New("Invalid config: REST protocol has to be configured")
	}

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
	Discover bool   `json:"discover"`
	Endpoint string `json:"endpoint"`
}

//
// Http config (for protocols using it)
//
type HttpConfig struct {
	BindAddr string `json:"bindAddr"`
	BindPort int    `json:"bindPort"`
}

//
// Protocol entry and types
//
type RestProtocol struct {
	Location string `json:"location"`
}

type MqttProtocol struct {
	Host   string `json:"host"`
	Port   int    `json:"port"`
	Prefix string `json:"prefix"`
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
