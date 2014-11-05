package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/url"
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
	Id           string                       `json:"id"`
	Description  string                       `json:"description"`
	DnssdEnabled bool                         `json:"dnssdEnabled"`
	PublicAddr   string                       `json:"publicAddr"`
	StaticDir    string                       `json:"staticDir`
	Catalog      []Catalog                    `json:"catalog"`
	Http         HttpConfig                   `json:"http"`
	Protocols    map[ProtocolType]interface{} `json:"protocols"`
	Devices      []Device                     `json:"devices"`
}

// Validates the loaded configuration
func (c *Config) Validate() error {
	// Check if HTTP configuration is valid
	err := c.Http.Validate()
	if err != nil {
		return err
	}

	_, ok := c.Protocols[ProtocolTypeREST]
	// Check if REST configuration is valid
	if ok {
		restConf := c.Protocols[ProtocolTypeREST].(RestProtocol)
		err := restConf.Validate()
		if err != nil {
			return err
		}
	}

	_, ok = c.Protocols[ProtocolTypeMQTT]
	// Check if MQTT configuration is valid
	if ok {
		mqttConf := c.Protocols[ProtocolTypeMQTT].(MqttProtocol)
		err := mqttConf.Validate()
		if err != nil {
			return err
		}
	}

	// Check if remote catalogs configs are valid
	for _, cat := range c.Catalog {
		err := cat.Validate()
		if err != nil {
			return err
		}
	}

	return nil
}

// Finds resource record by given resource id
func (c *Config) FindResource(resourceId string) (*Resource, bool) {
	for _, d := range c.Devices {
		for _, r := range d.Resources {
			if resourceId == d.ResourceId(r.Name) {
				return &r, true
			}
		}
	}
	return nil, false
}

//
// Catalog config
//
type Catalog struct {
	Discover bool   `json:"discover"`
	Endpoint string `json:"endpoint"`
}

func (c *Catalog) Validate() error {
	if c.Endpoint == "" && c.Discover == false {
		return fmt.Errorf("Catalog must have either endpoint or discovery flag defined")
	}
	return nil
}

//
// Http config (for protocols using it)
//
type HttpConfig struct {
	BindAddr string `json:"bindAddr"`
	BindPort int    `json:"bindPort"`
}

func (h *HttpConfig) Validate() error {
	if h.BindAddr == "" || h.BindPort == 0 {
		return fmt.Errorf("HTTP bindAddr and bindPort have to be defined")
	}
	return nil
}

//
// Protocol entry and types
//
type RestProtocol struct {
	Location string `json:"location"`
}

func (p *RestProtocol) Validate() error {
	if p.Location == "" {
		return fmt.Errorf("REST location has to be defined")
	}
	return nil
}

type MqttProtocol struct {
	Discover  bool   `json:"discover"`
	ServerUri string `json:"serverUri"`
	Prefix    string `json:"prefix"`
	Username  string `json:"username"`
	Password  string `json:"password"`
	CaFile    string `json:"caFile"`
	CertFile  string `json:"certFile"`
	KeyFile   string `json:"keyFile"`
}

func (p *MqttProtocol) Validate() error {
	if !p.Discover {
		serverUri, err := url.Parse(p.ServerUri)
		if err != nil {
			return fmt.Errorf("MQTT ServerUri must be a URI in the format scheme://host:port")
		}
		if serverUri.Scheme != "tcp" && serverUri.Scheme != "ssl" {
			return fmt.Errorf("MQTT ServerUri scheme must be either 'tcp' or 'ssl'")
		}
	}

	// Check that the CA file exists
	if p.CaFile != "" {
		if _, err := os.Stat(p.CaFile); os.IsNotExist(err) {
			return fmt.Errorf("MQTT CA file %s does not exist", p.CaFile)
		}
	}

	// Check that the client certificate and key files exist
	if p.CertFile != "" || p.KeyFile != "" {
		if _, err := os.Stat(p.CertFile); os.IsNotExist(err) {
			return fmt.Errorf("MQTT client certificate file %s does not exist", p.CertFile)
		}

		if _, err := os.Stat(p.KeyFile); os.IsNotExist(err) {
			return fmt.Errorf("MQTT client key file %s does not exist", p.KeyFile)
		}
	}
	return nil
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

func (d *Device) ResourceId(name string) string {
	return fmt.Sprintf("%s/%s", d.Name, name)
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
