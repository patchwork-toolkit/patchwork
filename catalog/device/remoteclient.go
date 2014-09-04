package device

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

type RemoteCatalogClient struct {
	serverEndpoint string // http://addr:port
}

func deviceFromResponse(res *http.Response) (Device, error) {
	var d Device
	body, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		return d, err
	}

	err = json.Unmarshal(body, &d)
	if err != nil {
		return d, err
	}
	d = d.unLdify()
	return d, nil
}

func NewRemoteCatalogClient(serverEndpoint string) *RemoteCatalogClient {
	return &RemoteCatalogClient{
		serverEndpoint: serverEndpoint,
	}
}

// Empty registration and nil error should be interpreted as "not found"
func (self *RemoteCatalogClient) Get(id string) (Device, error) {
	res, err := http.Get(fmt.Sprintf("%v%v/%v", self.serverEndpoint, CatalogBaseUrl, id))
	if err != nil {
		return Device{}, err
	}

	if res.StatusCode != http.StatusOK {
		return Device{}, nil
	}
	return deviceFromResponse(res)
}

func (self *RemoteCatalogClient) Add(d Device) (Device, error) {
	b, _ := json.Marshal(d)
	res, err := http.Post(self.serverEndpoint+CatalogBaseUrl+"/", "application/ld+json", bytes.NewReader(b))
	if err != nil {
		return Device{}, err
	}
	return deviceFromResponse(res)
}

// Empty registration and nil error should be interpreted as "not found"
func (self *RemoteCatalogClient) Update(id string, d Device) (Device, error) {
	b, _ := json.Marshal(d)
	req, err := http.NewRequest("PUT", fmt.Sprintf("%v%v/%v", self.serverEndpoint, CatalogBaseUrl, id), bytes.NewReader(b))
	if err != nil {
		return Device{}, err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return Device{}, err
	}

	if res.StatusCode != http.StatusOK {
		return Device{}, nil
	}
	return deviceFromResponse(res)
}

// Empty registration and nil error should be interpreted as "not found"
func (self *RemoteCatalogClient) Delete(id string) (Device, error) {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%v%v/%v", self.serverEndpoint, CatalogBaseUrl, id), bytes.NewReader([]byte{}))
	if err != nil {
		return Device{}, err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return Device{}, err
	}

	if res.StatusCode != http.StatusOK {
		return Device{}, nil
	}

	return deviceFromResponse(res)
}

func (self *RemoteCatalogClient) GetMany(page int, perPage int) ([]Device, int, error) {
	res, err := http.Get(self.serverEndpoint + CatalogBaseUrl)
	if err != nil {
		return nil, 0, err
	}

	body, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		return nil, 0, err
	}

	var coll Collection
	err = json.Unmarshal(body, &coll)
	if err != nil {
		return nil, 0, err
	}

	devs := make([]Device, 0, len(coll.Devices))
	for k, v := range coll.Devices {
		d := *v.Device
		for _, res := range coll.Resources {
			if res.Device == k {
				d.Resources = append(d.Resources, res)
			}
		}
		devs = append(devs, d)
	}
	return devs, len(coll.Devices), nil
}
