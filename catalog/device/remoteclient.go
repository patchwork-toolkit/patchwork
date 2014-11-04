package device

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type RemoteCatalogClient struct {
	serverEndpoint *url.URL
}

func deviceFromResponse(res *http.Response, apiLocation string) (*Device, error) {
	decoder := json.NewDecoder(res.Body)
	defer res.Body.Close()

	var d Device
	err := decoder.Decode(&d)
	if err != nil {
		return nil, err
	}
	d = d.unLdify(apiLocation)
	return &d, nil
}

func devicesFromResponse(res *http.Response, apiLocation string) ([]Device, int, error) {
	decoder := json.NewDecoder(res.Body)
	defer res.Body.Close()

	var coll Collection
	err := decoder.Decode(&coll)
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
		devs = append(devs, d.unLdify(apiLocation))
	}

	return devs, len(coll.Devices), nil
}

func resourceFromResponse(res *http.Response, apiLocation string) (*Resource, error) {
	decoder := json.NewDecoder(res.Body)
	defer res.Body.Close()

	var r Resource
	err := decoder.Decode(&r)
	if err != nil {
		return nil, err
	}
	r = r.unLdify(apiLocation)
	return &r, nil
}

func resourcesFromResponse(res *http.Response, apiLocation string) ([]Resource, int, error) {
	decoder := json.NewDecoder(res.Body)
	defer res.Body.Close()

	var coll Collection
	err := decoder.Decode(&coll)
	if err != nil {
		return nil, 0, err
	}

	ress := make([]Resource, 0, len(coll.Resources))
	for _, r := range coll.Resources {
		ress = append(ress, r.unLdify(apiLocation))
	}

	return ress, len(coll.Resources), nil
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

func (self *RemoteCatalogClient) Get(id string) (*Device, error) {
	res, err := http.Get(fmt.Sprintf("%v/%v", self.serverEndpoint, id))
	if err != nil {
		return nil, err
	}

	if res.StatusCode == http.StatusNotFound {
		return nil, ErrorNotFound
	} else if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%v", res.StatusCode)
	}
	return deviceFromResponse(res, self.serverEndpoint.Path)
}

func (self *RemoteCatalogClient) Add(d *Device) error {
	b, _ := json.Marshal(d)
	_, err := http.Post(self.serverEndpoint.String()+"/", "application/ld+json", bytes.NewReader(b))
	if err != nil {
		return err
	}
	return nil
}

func (self *RemoteCatalogClient) Update(id string, d *Device) error {
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

func (self *RemoteCatalogClient) GetDevices(page int, perPage int) ([]Device, int, error) {
	res, err := http.Get(
		fmt.Sprintf("%v?%v=%v&%v=%v",
			self.serverEndpoint, GetParamPage, page, GetParamPerPage, perPage))
	if err != nil {
		return nil, 0, err
	}

	return devicesFromResponse(res, self.serverEndpoint.Path)
}

func (self *RemoteCatalogClient) FindDevice(path, op, value string) (*Device, error) {
	res, err := http.Get(fmt.Sprintf("%v/%v/%v/%v/%v", self.serverEndpoint, FTypeDevice, path, op, value))
	if err != nil {
		return nil, err
	}

	if res.StatusCode == http.StatusNotFound {
		return nil, ErrorNotFound
	} else if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%v", res.StatusCode)
	}
	return deviceFromResponse(res, self.serverEndpoint.Path)
}

func (self *RemoteCatalogClient) FindDevices(path, op, value string, page, perPage int) ([]Device, int, error) {
	res, err := http.Get(
		fmt.Sprintf("%v/%v/%v/%v/%v?%v=%v&%v=%v",
			self.serverEndpoint, FTypeDevices, path, op, value, GetParamPage, page, GetParamPerPage, perPage))
	if err != nil {
		return nil, 0, err
	}

	return devicesFromResponse(res, self.serverEndpoint.Path)
}

func (self *RemoteCatalogClient) FindResource(path, op, value string) (*Resource, error) {
	res, err := http.Get(fmt.Sprintf("%v/%v/%v/%v/%v", self.serverEndpoint, FTypeResource, path, op, value))
	if err != nil {
		return nil, err
	}

	if res.StatusCode == http.StatusNotFound {
		return nil, ErrorNotFound
	} else if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%v", res.StatusCode)
	}
	return resourceFromResponse(res, self.serverEndpoint.Path)
}

func (self *RemoteCatalogClient) FindResources(path, op, value string, page, perPage int) ([]Resource, int, error) {
	res, err := http.Get(
		fmt.Sprintf("%v/%v/%v/%v/%v?%v=%v&%v=%v",
			self.serverEndpoint, FTypeResources, path, op, value, GetParamPage, page, GetParamPerPage, perPage))
	if err != nil {
		return nil, 0, err
	}

	return resourcesFromResponse(res, self.serverEndpoint.Path)
}
