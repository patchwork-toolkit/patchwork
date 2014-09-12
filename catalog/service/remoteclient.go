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

func serviceFromResponse(res *http.Response) (Service, error) {
	var s Service
	body, err := ioutil.ReadAll(res.Body)
	res.Body.Close()
	if err != nil {
		return s, err
	}

	err = json.Unmarshal(body, &s)
	if err != nil {
		return s, err
	}
	s = s.unLdify()
	return s, nil
}

func NewRemoteCatalogClient(serverEndpoint string) *RemoteCatalogClient {
	return &RemoteCatalogClient{
		serverEndpoint: serverEndpoint,
	}
}

func (self *RemoteCatalogClient) Get(id string) (Service, error) {
	res, err := http.Get(fmt.Sprintf("%v%v/%v", self.serverEndpoint, CatalogBaseUrl, id))
	if err != nil {
		return Service{}, err
	}

	if res.StatusCode == http.StatusNotFound {
		return Service{}, ErrorNotFound
	} else if res.StatusCode != http.StatusOK {
		return Service{}, fmt.Errorf("%v", res.StatusCode)
	}
	return serviceFromResponse(res)
}

func (self *RemoteCatalogClient) Add(s Service) error {
	b, _ := json.Marshal(s)
	_, err := http.Post(self.serverEndpoint+CatalogBaseUrl+"/", "application/ld+json", bytes.NewReader(b))
	if err != nil {
		return err
	}
	return nil
}

func (self *RemoteCatalogClient) Update(id string, s Service) error {
	b, _ := json.Marshal(s)
	req, err := http.NewRequest("PUT", fmt.Sprintf("%v%v/%v", self.serverEndpoint, CatalogBaseUrl, id), bytes.NewReader(b))
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
	req, err := http.NewRequest("DELETE", fmt.Sprintf("%v%v/%v", self.serverEndpoint, CatalogBaseUrl, id), bytes.NewReader([]byte{}))
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

func (self *RemoteCatalogClient) GetMany(page, perPage int) ([]Service, int, error) {
	res, err := http.Get(
		fmt.Sprintf("%s%s?%s=%s&%s=%s",
			self.serverEndpoint, CatalogBaseUrl, GetParamPage, page, GetParamPerPage, perPage))
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

	svcs := make([]Service, 0, len(coll.Services))
	for _, v := range coll.Services {
		svcs = append(svcs, v)
	}

	return svcs, len(svcs), nil
}
