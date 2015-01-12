package device

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

	url := apiLocation + "/{dgwid}/{regid}"
	r.Methods("GET").Path(url).HandlerFunc(api.Get).Name("get")
	r.Methods("PUT").Path(url).HandlerFunc(api.Update).Name("update")
	r.Methods("DELETE").Path(url).HandlerFunc(api.Delete).Name("delete")
	r.Methods("GET").Path(url + "/{resname}").HandlerFunc(api.GetResource).Name("details")

	return r
}

func mockedDevice(id string) *Device {
	return &Device{
		Id:          "TestHost/TestDevice" + id,
		Type:        ApiDeviceType,
		Name:        "TestDevice" + id,
		Meta:        map[string]interface{}{"test-id": id},
		Description: "Test Device",
		Ttl:         30,
		Resources: []Resource{
			Resource{
				Id:   "TestHost/TestDevice" + id + "/TestResource",
				Type: ApiResourceType,
				Name: "TestResource",
				Meta: map[string]interface{}{"test-id-resource": id},
				Protocols: []Protocol{Protocol{
					Type:         "REST",
					Endpoint:     map[string]interface{}{"url": "http://localhost:9000/rest/device/resource"},
					Methods:      []string{"GET"},
					ContentTypes: []string{"application/senml+json"},
				}},
				Representation: map[string]interface{}{"application/senml+json": ""},
			},
		},
	}
}

func sameDevices(d1, d2 *Device, checkID bool) bool {
	// Compare IDs if specified
	if checkID {
		if d1.Id != d2.Type {
			return false
		}
	}

	// Compare metadata
	for k1, v1 := range d1.Meta {
		v2, ok := d2.Meta[k1]
		if !ok || v1 != v2 {
			return false
		}
	}
	for k2, v2 := range d2.Meta {
		v1, ok := d1.Meta[k2]
		if !ok || v1 != v2 {
			return false
		}
	}

	// Compare number of resources
	if len(d1.Resources) != len(d2.Resources) {
		return false
	}

	// Compare all other attributes
	if d1.Type != d2.Type || d1.Name != d2.Name || d1.Description != d2.Description || d1.Ttl != d2.Ttl {
		return false
	}

	return true
}

func sameResources(r1, r2 *Resource, checkID bool) bool {
	// Compare IDs if specified
	if checkID {
		if r1.Id != r2.Type {
			return false
		}
	}

	// Compare metadata
	for k1, v1 := range r1.Meta {
		v2, ok := r2.Meta[k1]
		if !ok || v1 != v2 {
			return false
		}
	}
	for k2, v2 := range r2.Meta {
		v1, ok := r1.Meta[k2]
		if !ok || v1 != v2 {
			return false
		}
	}

	// Compare all other attributes
	if r1.Type != r2.Type || r1.Name != r2.Name || len(r1.Protocols) != len(r2.Protocols) {
		return false
	}

	return true
}

func TestCreate(t *testing.T) {
	ts := httptest.NewServer(setupRouter())
	defer ts.Close()

	device := mockedDevice("1")
	b, _ := json.Marshal(device)

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

	device := mockedDevice("1")
	resource := &device.Resources[0]
	b, _ := json.Marshal(device)

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

	// Retrieve: device
	url = url + device.Id
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

	var device2 *Device
	decoder := json.NewDecoder(res.Body)
	defer res.Body.Close()

	err = decoder.Decode(&device2)
	if err != nil {
		t.Fatal(err.Error())
	}

	if !strings.HasPrefix(device2.Id, apiLocation) {
		t.Fatalf("Device ID should have been prefixed with %v by catalog, retrieved ID: %v", apiLocation, device2.Id)
	}
	if !sameDevices(device, device2, false) {
		t.Fatalf("The retrieved device is not the same as the added one:\n Added:\n %v \n Retrieved: \n %v", device, device2)
	}
	if !sameResources(&device.Resources[0], resource, false) {
		t.Fatalf("The retrieved device has not the same resource as the added one:\n Added:\n %v \n Retrieved: \n %v", device, device2)
	}

	// Retrieve: resource
	url = ts.URL + apiLocation + "/" + resource.Id
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

	var resource2 *Resource
	decoder = json.NewDecoder(res.Body)
	defer res.Body.Close()

	err = decoder.Decode(&resource2)
	if err != nil {
		t.Fatal(err.Error())
	}

	if !strings.HasPrefix(resource2.Id, apiLocation) {
		t.Fatalf("Resource ID should have been prefixed with %v by catalog, retrieved ID: %v", apiLocation, resource2.Id)
	}
	if !sameResources(resource, resource2, false) {
		t.Fatalf("The retrieved resource is not the same as the added one:\n Added:\n %v \n Retrieved: \n %v", resource, resource2)
	}
}

func TestUpdate(t *testing.T) {
	ts := httptest.NewServer(setupRouter())
	defer ts.Close()

	device := mockedDevice("1")
	b, _ := json.Marshal(device)

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

	// Update
	device2 := mockedDevice("1")
	device2.Description = "Updated Test Device"
	url = url + device.Id
	b, _ = json.Marshal(device2)

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
		t.Fatalf("Server should return %v, got instead: %v (%s)", http.StatusCreated, res.StatusCode, res.Status)
	}

	// Retrieve & compare
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

	var device3 *Device
	decoder := json.NewDecoder(res.Body)
	defer res.Body.Close()

	err = decoder.Decode(&device3)
	if err != nil {
		t.Fatal(err.Error())
	}

	if !strings.HasPrefix(device3.Id, apiLocation) {
		t.Fatalf("Device ID should have been prefixed with %v by catalog, retrieved ID: %v", apiLocation, device3.Id)
	}
	if !sameDevices(device2, device3, false) {
		t.Fatalf("The retrieved device is not the same as the added one:\n Added:\n %v \n Retrieved: \n %v", device2, device3)
	}
	if !sameResources(&device2.Resources[0], &device3.Resources[0], false) {
		t.Fatalf("The retrieved device has not the same resource as the added one:\n Added:\n %v \n Retrieved: \n %v", device2, device3)
	}
}

func TestDelete(t *testing.T) {
	ts := httptest.NewServer(setupRouter())
	defer ts.Close()

	device := mockedDevice("1")
	b, _ := json.Marshal(device)

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

	// Delete
	url = url + device.Id
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
		t.Fatalf("Server should return %v, got instead: %v (%s)", http.StatusCreated, res.StatusCode, res.Status)
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

func TestFilter(t *testing.T) {
	ts := httptest.NewServer(setupRouter())
	defer ts.Close()

	// create 3 devices
	device1 := mockedDevice("1")
	device2 := mockedDevice("2")
	device3 := mockedDevice("3")

	// Add
	url := ts.URL + apiLocation + "/"
	for _, d := range []*Device{device1, device2, device3} {
		b, _ := json.Marshal(d)

		_, err := http.Post(url, "application/ld+json", bytes.NewReader(b))
		if err != nil {
			t.Fatal(err.Error())
		}
	}

	// Devices
	// Filter many
	url = ts.URL + apiLocation + "/" + FTypeDevices + "/name/" + utils.FOpPrefix + "/" + "Test"
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
	url = ts.URL + apiLocation + "/" + FTypeDevice + "/name/" + utils.FOpEquals + "/" + device1.Name
	t.Log("Calling GET", url)
	res, err = http.Get(url)
	if err != nil {
		t.Fatal(err.Error())
	}

	var deviceF *Device
	decoder = json.NewDecoder(res.Body)
	defer res.Body.Close()

	err = decoder.Decode(&deviceF)
	if err != nil {
		t.Fatal(err.Error())
	}

	if !sameDevices(device1, deviceF, false) {
		t.Fatalf("The retrieved device is not the same as the added one:\n Added:\n %v \n Retrieved: \n %v", device1, deviceF)
	}

	// Resources
	// Filter many
	url = ts.URL + apiLocation + "/" + FTypeResources + "/name/" + utils.FOpPrefix + "/" + "Test"
	t.Log("Calling GET", url)
	res, err = http.Get(url)
	if err != nil {
		t.Fatal(err.Error())
	}

	decoder = json.NewDecoder(res.Body)
	defer res.Body.Close()

	err = decoder.Decode(&collection)

	if err != nil {
		t.Fatal(err.Error())
	}

	if collection.Total != 3 {
		t.Fatal("Server should return a collection of *3* resources, but got total", collection.Total)
	}

	// Filter one
	url = ts.URL + apiLocation + "/" + FTypeResource + "/name/" + utils.FOpEquals + "/" + device1.Resources[0].Name
	t.Log("Calling GET", url)
	res, err = http.Get(url)
	if err != nil {
		t.Fatal(err.Error())
	}

	var resource *Resource
	decoder = json.NewDecoder(res.Body)
	defer res.Body.Close()

	err = decoder.Decode(&resource)
	if err != nil {
		t.Fatal(err.Error())
	}

	if !sameResources(&device1.Resources[0], resource, false) {
		t.Fatalf("The retrieved device is not the same as the added one:\n Added:\n %v \n Retrieved: \n %v", device1.Resources[0], resource)
	}
}
