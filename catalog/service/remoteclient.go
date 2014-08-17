package service

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

func registrationFromResponse(res *http.Response) (Registration, error) {
	var r Registration
	body, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		return r, err
	}

	err = json.Unmarshal(body, &r)
	if err != nil {
		return r, err
	}
	r = r.unLdify()
	return r, nil
}

func NewRemoteCatalogClient(serverEndpoint string) *RemoteCatalogClient {
	return &RemoteCatalogClient{
		serverEndpoint: serverEndpoint,
	}
}

// Empty registration and nil error should be interpreted as "not found"
func (self *RemoteCatalogClient) Get(id string) (Registration, error) {
	res, err := http.Get(fmt.Sprintf("%v%v/%v", self.serverEndpoint, CatalogBaseUrl, id))
	if err != nil {
		return Registration{}, err
	}

	if res.StatusCode != http.StatusOK {
		return Registration{}, nil
	}
	return registrationFromResponse(res)
}

func (self *RemoteCatalogClient) Add(r Registration) (Registration, error) {
	b, _ := json.Marshal(r)
	res, err := http.Post(self.serverEndpoint+CatalogBaseUrl+"/", "application/ld+json", bytes.NewReader(b))
	if err != nil {
		return Registration{}, err
	}
	return registrationFromResponse(res)
}

// Empty registration and nil error should be interpreted as "not found"
func (self *RemoteCatalogClient) Update(id string, r Registration) (Registration, error) {
	b, _ := json.Marshal(r)
	req, err := http.NewRequest("PUT", fmt.Sprintf("%v%v/%v", self.serverEndpoint, CatalogBaseUrl, id), bytes.NewReader(b))
	if err != nil {
		return Registration{}, err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return Registration{}, err
	}

	if res.StatusCode != http.StatusOK {
		return Registration{}, nil
	}
	return registrationFromResponse(res)
}

// Empty registration and nil error should be interpreted as "not found"
func (self *RemoteCatalogClient) Delete(id string) (Registration, error) {
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%v%v/%v", self.serverEndpoint, CatalogBaseUrl, id), bytes.NewReader([]byte{}))
	if err != nil {
		return Registration{}, err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return Registration{}, err
	}

	if res.StatusCode != http.StatusOK {
		return Registration{}, nil
	}

	return registrationFromResponse(res)
}

func (self *RemoteCatalogClient) GetAll() ([]Registration, error) {
	res, err := http.Get(self.serverEndpoint + CatalogBaseUrl)
	if err != nil {
		return nil, err
	}

	body, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		return nil, err
	}

	var coll Collection
	err = json.Unmarshal(body, &coll)
	if err != nil {
		return nil, err
	}

	regs := make([]Registration, 0, len(coll.Services))
	for _, v := range coll.Services {
		regs = append(regs, v)
	}
	return regs, nil
}
