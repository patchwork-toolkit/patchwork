package service

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/patchwork-toolkit/patchwork/Godeps/_workspace/src/github.com/gorilla/mux"
	utils "github.com/patchwork-toolkit/patchwork/catalog"
)

const (
	apiLocation    = "/cat"
	staticLocation = "/static"
)

func setupRouter() *mux.Router {
	api := NewWritableCatalogAPI(
		NewMemoryStorage(),
		apiLocation,
		staticLocation,
		"Test catalog",
	)

	r := mux.NewRouter().StrictSlash(true)
	r.Methods("GET").Path(apiLocation).HandlerFunc(api.List).Name("list")
	r.Methods("POST").Path(apiLocation + "/").HandlerFunc(api.Add).Name("add")
	r.Methods("GET").Path(apiLocation + "/{type}/{path}/{op}/{value}").HandlerFunc(api.Filter).Name("filter")

	url := apiLocation + "/{hostid}/{regid}"
	r.Methods("GET").Path(url).HandlerFunc(api.Get).Name("get")
	r.Methods("PUT").Path(url).HandlerFunc(api.Update).Name("update")
	r.Methods("DELETE").Path(url).HandlerFunc(api.Delete).Name("delete")

	return r
}

func mockedService(id string) *Service {
	return &Service{
		Id:          "TestHost/TestService" + id,
		Type:        ApiRegistrationType,
		Name:        "TestService" + id,
		Meta:        map[string]interface{}{"test-id": id},
		Description: "Test Service",
		Protocols: []Protocol{Protocol{
			Type:         "REST",
			Endpoint:     map[string]interface{}{"url": "http://localhost:9000/api"},
			Methods:      []string{"GET"},
			ContentTypes: []string{"application/json"},
		}},
		Representation: map[string]interface{}{"application/json": "{}"},
		Ttl:            30,
	}
}

func sameServices(s1, s2 *Service, checkID bool) bool {
	// Compare IDs if specified
	if checkID {
		if s1.Id != s2.Type {
			return false
		}
	}

	// Compare metadata
	for k1, v1 := range s1.Meta {
		v2, ok := s2.Meta[k1]
		if !ok || v1 != v2 {
			return false
		}
	}
	for k2, v2 := range s2.Meta {
		v1, ok := s1.Meta[k2]
		if !ok || v1 != v2 {
			return false
		}
	}

	// Compare number of protocols
	if len(s1.Protocols) != len(s2.Protocols) {
		return false
	}

	// Compare all other attributes
	if s1.Type != s2.Type || s1.Name != s2.Name || s1.Description != s2.Description || s1.Ttl != s2.Ttl {
		return false
	}

	return true
}

func TestList(t *testing.T) {
	ts := httptest.NewServer(setupRouter())
	defer ts.Close()

	url := ts.URL + apiLocation
	t.Log("Calling GET", url)
	res, err := http.Get(url)
	if err != nil {
		t.Fatal(err.Error())
	}

	if res.StatusCode != http.StatusOK {
		t.Fatalf("Server should return %v, got instead: %v (%s)", http.StatusOK, res.StatusCode, res.Status)
	}

	if !strings.HasPrefix(res.Header.Get("Content-Type"), "application/ld+json") {
		t.Fatalf("Response should have Content-Type: application/ld+json, got instead %s", res.Header.Get("Content-Type"))
	}

	var collection *Collection
	decoder := json.NewDecoder(res.Body)
	defer res.Body.Close()

	err = decoder.Decode(&collection)
	if err != nil {
		t.Fatal(err.Error())
	}

	if collection.Total > 0 {
		t.Fatal("Server should return empty collection, but got total", collection.Total)
	}
}

func TestCreate(t *testing.T) {
	ts := httptest.NewServer(setupRouter())
	defer ts.Close()

	service := mockedService("1")
	b, _ := json.Marshal(service)

	// Create
	url := ts.URL + apiLocation + "/"
	t.Log("Calling POST", url)
	res, err := http.Post(url, "application/ld+json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err.Error())
	}

	if res.StatusCode != http.StatusCreated {
		t.Fatalf("Server should return %v, got instead: %v (%s)", http.StatusCreated, res.StatusCode, res.Status)
	}

	if !strings.HasPrefix(res.Header.Get("Content-Type"), "application/ld+json") {
		t.Fatalf("Response should have Content-Type: application/ld+json, got instead %s", res.Header.Get("Content-Type"))
	}

	// Retrieve whole collection
	t.Log("Calling GET", ts.URL+apiLocation)
	res, err = http.Get(ts.URL + apiLocation)
	if err != nil {
		t.Fatal(err.Error())
	}

	var collection *Collection
	decoder := json.NewDecoder(res.Body)
	defer res.Body.Close()

	err = decoder.Decode(&collection)

	if err != nil {
		t.Fatal(err.Error())
	}

	if collection.Total != 1 {
		t.Fatal("Server should return collection with exactly 1 resource, but got total", collection.Total)
	}
}

func TestRetrieve(t *testing.T) {
	ts := httptest.NewServer(setupRouter())
	defer ts.Close()

	service := mockedService("1")
	b, _ := json.Marshal(service)

	// Create
	url := ts.URL + apiLocation + "/"
	t.Log("Calling POST", url)
	res, err := http.Post(url, "application/ld+json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err.Error())
	}

	// Retrieve: service
	url = url + service.Id
	t.Log("Calling GET", url)
	res, err = http.Get(url)
	if err != nil {
		t.Fatal(err.Error())
	}

	if res.StatusCode != http.StatusOK {
		t.Fatalf("Server should return %v, got instead: %v (%s)", http.StatusOK, res.StatusCode, res.Status)
	}

	if !strings.HasPrefix(res.Header.Get("Content-Type"), "application/ld+json") {
		t.Fatalf("Response should have Content-Type: application/ld+json, got instead %s", res.Header.Get("Content-Type"))
	}

	var service2 *Service
	decoder := json.NewDecoder(res.Body)
	defer res.Body.Close()

	err = decoder.Decode(&service2)
	if err != nil {
		t.Fatal(err.Error())
	}

	if !strings.HasPrefix(service2.Id, apiLocation) {
		t.Fatalf("Service ID should have been prefixed with %v by catalog, retrieved ID: %v", apiLocation, service2.Id)
	}
	if !sameServices(service, service2, false) {
		t.Fatalf("The retrieved service is not the same as the added one:\n Added:\n %v \n Retrieved: \n %v", service, service2)
	}
}

func TestUpdate(t *testing.T) {
	ts := httptest.NewServer(setupRouter())
	defer ts.Close()

	service := mockedService("1")
	b, _ := json.Marshal(service)

	// Create
	url := ts.URL + apiLocation + "/"
	t.Log("Calling POST", url)
	res, err := http.Post(url, "application/ld+json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err.Error())
	}

	// Update
	service2 := mockedService("1")
	service2.Description = "Updated Test Service"
	url = url + service.Id
	b, _ = json.Marshal(service2)

	t.Log("Calling PUT", url)
	req, err := http.NewRequest("PUT", url, bytes.NewReader(b))
	if err != nil {
		t.Fatal(err.Error())
	}
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err.Error())
	}

	if res.StatusCode != http.StatusOK {
		t.Fatalf("Server should return %v, got instead: %v (%s)", http.StatusOK, res.StatusCode, res.Status)
	}

	// Retrieve & compare
	t.Log("Calling GET", url)
	res, err = http.Get(url)
	if err != nil {
		t.Fatal(err.Error())
	}

	var service3 *Service
	decoder := json.NewDecoder(res.Body)
	defer res.Body.Close()

	err = decoder.Decode(&service3)
	if err != nil {
		t.Fatal(err.Error())
	}

	if !sameServices(service2, service3, false) {
		t.Fatalf("The retrieved service is not the same as the added one:\n Added:\n %v \n Retrieved: \n %v", service2, service3)
	}
}

func TestDelete(t *testing.T) {
	ts := httptest.NewServer(setupRouter())
	defer ts.Close()

	service := mockedService("1")
	b, _ := json.Marshal(service)

	// Create
	url := ts.URL + apiLocation + "/"
	t.Log("Calling POST", url)
	res, err := http.Post(url, "application/ld+json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err.Error())
	}

	// Delete
	url = url + service.Id
	t.Log("Calling DELETE", url)
	req, err := http.NewRequest("DELETE", url, bytes.NewReader([]byte{}))
	if err != nil {
		t.Fatal(err.Error())
	}
	res, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err.Error())
	}

	if res.StatusCode != http.StatusOK {
		t.Fatalf("Server should return %v, got instead: %v (%s)", http.StatusOK, res.StatusCode, res.Status)
	}

	// Retrieve whole collection
	t.Log("Calling GET", ts.URL+apiLocation)
	res, err = http.Get(ts.URL + apiLocation)
	if err != nil {
		t.Fatal(err.Error())
	}

	var collection *Collection
	decoder := json.NewDecoder(res.Body)
	defer res.Body.Close()

	err = decoder.Decode(&collection)

	if err != nil {
		t.Fatal(err.Error())
	}

	if collection.Total != 0 {
		t.Fatal("Server should return an empty collection, but got total", collection.Total)
	}

}

func TestFilter(t *testing.T) {
	ts := httptest.NewServer(setupRouter())
	defer ts.Close()

	// create 3 services
	service1 := mockedService("1")
	service2 := mockedService("2")
	service3 := mockedService("3")

	// Add
	url := ts.URL + apiLocation + "/"
	for _, d := range []*Service{service1, service2, service3} {
		b, _ := json.Marshal(d)

		_, err := http.Post(url, "application/ld+json", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err.Error())
		}
	}

	// Services
	// Filter many
	url = ts.URL + apiLocation + "/" + FTypeServices + "/name/" + utils.FOpPrefix + "/" + "Test"
	t.Log("Calling GET", url)
	res, err := http.Get(url)
	if err != nil {
		t.Fatal(err.Error())
	}

	var collection *Collection
	decoder := json.NewDecoder(res.Body)
	defer res.Body.Close()

	err = decoder.Decode(&collection)

	if err != nil {
		t.Fatal(err.Error())
	}

	if collection.Total != 3 {
		t.Fatal("Server should return a collection of *3* resources, but got total", collection.Total)
	}

	// Filter one
	url = ts.URL + apiLocation + "/" + FTypeService + "/name/" + utils.FOpEquals + "/" + service1.Name
	t.Log("Calling GET", url)
	res, err = http.Get(url)
	if err != nil {
		t.Fatal(err.Error())
	}

	var serviceF *Service
	decoder = json.NewDecoder(res.Body)
	defer res.Body.Close()

	err = decoder.Decode(&serviceF)
	if err != nil {
		t.Fatal(err.Error())
	}

	if !sameServices(service1, serviceF, false) {
		t.Fatalf("The retrieved service is not the same as the added one:\n Added:\n %v \n Retrieved: \n %v", service1, serviceF)
	}
}
