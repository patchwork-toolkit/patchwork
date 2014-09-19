package device

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
)

type RemoteCatalogClient struct {
	serverEndpoint *url.URL
}

func deviceFromResponse(res *http.Response, apiLocation string) (Device, error) {
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
	d = d.unLdify(apiLocation)
	return d, nil
}

func NewRemoteCatalogClient(serverEndpoint string) *RemoteCatalogClient {
	// Check if serverEndpoint is a correct URL
	endpointUrl, err := url.Parse(serverEndpoint)
	if err != nil {
		return &RemoteCatalogClient{}
	}

	return &RemoteCatalogClient{
		serverEndpoint: endpointUrl,
	}
}

func (self *RemoteCatalogClient) Get(id string) (Device, error) {
	res, err := http.Get(fmt.Sprintf("%v/%v", self.serverEndpoint, id))
	if err != nil {
		return Device{}, err
	}

	if res.StatusCode == http.StatusNotFound {
		return Device{}, ErrorNotFound
	} else if res.StatusCode != http.StatusOK {
		return Device{}, fmt.Errorf("%v", res.StatusCode)
	}
	return deviceFromResponse(res, self.serverEndpoint.Path)
}

func (self *RemoteCatalogClient) Add(d Device) error {
	b, _ := json.Marshal(d)
	_, err := http.Post(self.serverEndpoint.String()+"/", "application/ld+json", bytes.NewReader(b))
	if err != nil {
		return err
	}
	return nil
}

func (self *RemoteCatalogClient) Update(id string, d Device) error {
	b, _ := json.Marshal(d)
	req, err := http.NewRequest("PUT", fmt.Sprintf("%v/%v", self.serverEndpoint, id), bytes.NewReader(b))
	if err != nil {
		return err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	if res.StatusCode == http.StatusNotFound {
		return ErrorNotFound
	} else if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%v", res.StatusCode)
	}
	return nil
}

func (self *RemoteCatalogClient) Delete(id string) error {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%v/%v", self.serverEndpoint, id), bytes.NewReader([]byte{}))
	if err != nil {
		return err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	if res.StatusCode == http.StatusNotFound {
		return ErrorNotFound
	} else if res.StatusCode != http.StatusOK {
		return fmt.Errorf("%v", res.StatusCode)
	}

	return nil
}

func (self *RemoteCatalogClient) GetMany(page int, perPage int) ([]Device, int, error) {
	res, err := http.Get(
		fmt.Sprintf("%s?%s=%s&%s=%s",
			self.serverEndpoint, GetParamPage, page, GetParamPerPage, perPage))
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
		devs = append(devs, d.unLdify(self.serverEndpoint.Path))
	}
	return devs, len(coll.Devices), nil
}
