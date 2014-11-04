package service

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

func serviceFromResponse(res *http.Response, apiLocation string) (*Service, error) {
	decoder := json.NewDecoder(res.Body)
	defer res.Body.Close()

	var s *Service
	err := decoder.Decode(&s)
	if err != nil {
		return nil, err
	}
	svc := s.unLdify(apiLocation)
	return &svc, nil
}

func servicesFromResponse(res *http.Response, apiLocation string) ([]Service, int, error) {
	decoder := json.NewDecoder(res.Body)
	defer res.Body.Close()

	var coll Collection
	err := decoder.Decode(&coll)
	if err != nil {
		return nil, 0, err
	}

	svcs := make([]Service, 0, len(coll.Services))
	for _, v := range coll.Services {
		svcs = append(svcs, v.unLdify(apiLocation))
	}

	return svcs, len(svcs), nil
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

func (self *RemoteCatalogClient) Get(id string) (*Service, error) {
	res, err := http.Get(fmt.Sprintf("%v/%v", self.serverEndpoint, id))
	if err != nil {
		return nil, err
	}

	if res.StatusCode == http.StatusNotFound {
		return nil, ErrorNotFound
	} else if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%v", res.StatusCode)
	}
	return serviceFromResponse(res, self.serverEndpoint.Path)
}

func (self *RemoteCatalogClient) Add(s *Service) error {
	b, _ := json.Marshal(s)
	_, err := http.Post(self.serverEndpoint.String()+"/", "application/ld+json", bytes.NewReader(b))
	if err != nil {
		return err
	}
	return nil
}

func (self *RemoteCatalogClient) Update(id string, s *Service) error {
	b, _ := json.Marshal(s)
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

func (self *RemoteCatalogClient) GetServices(page, perPage int) ([]Service, int, error) {
	res, err := http.Get(
		fmt.Sprintf("%v?%v=%v&%v=%v",
			self.serverEndpoint, GetParamPage, page, GetParamPerPage, perPage))
	if err != nil {
		return nil, 0, err
	}

	return servicesFromResponse(res, self.serverEndpoint.Path)
}

func (self *RemoteCatalogClient) FindService(path, op, value string) (*Service, error) {
	res, err := http.Get(fmt.Sprintf("%v/%v/%v/%v/%v", self.serverEndpoint, FTypeService, path, op, value))
	if err != nil {
		return nil, err
	}

	if res.StatusCode == http.StatusNotFound {
		return nil, ErrorNotFound
	} else if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%v", res.StatusCode)
	}

	return serviceFromResponse(res, self.serverEndpoint.Path)
}

func (self *RemoteCatalogClient) FindServices(path, op, value string, page, perPage int) ([]Service, int, error) {
	res, err := http.Get(
		fmt.Sprintf("%v/%v/%v/%v/%v?%v=%v&%v=%v",
			self.serverEndpoint, FTypeServices, path, op, value, GetParamPage, page, GetParamPerPage, perPage))
	if err != nil {
		return nil, 0, err
	}

	return servicesFromResponse(res, self.serverEndpoint.Path)
}
