package device

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

const (
	PatternReg         = ":regid"
	PatternRes         = ":resname"
	PatternUuid        = ":uuid"
	PatternFType       = ":type"
	PatternFPath       = ":path"
	PatternFOp         = ":op"
	PatternFValue      = ":value"
	FTypeRegistration  = "device"
	FTypeRegistrations = "devices"
	FTypeResource      = "resource"
	FTypeResources     = "resources"
	CurrentApiVersion  = "0.2.1"
)

type Collection struct {
	Context   string                  `json:"@context,omitempty"`
	Id        string                  `json:"id"`
	Type      string                  `json:"type"`
	Devices   map[string]Registration `json:"devices"`
	Resources []Resource              `json:"resources"`
}

// Registration without resources
type Device struct {
}

// Read-only catalog api
type ReadableCatalogAPI struct {
	catalogStorage CatalogStorage
	contextUrl     string
}

// Writable catalog api
type WritableCatalogAPI struct {
	*ReadableCatalogAPI
}

func NewReadableCatalogAPI(storage CatalogStorage, contextUrl string) *ReadableCatalogAPI {
	return &ReadableCatalogAPI{
		catalogStorage: storage,
		contextUrl:     contextUrl,
	}
}

func NewWritableCatalogAPI(storage CatalogStorage, contextUrl string) *WritableCatalogAPI {
	return &WritableCatalogAPI{
		&ReadableCatalogAPI{
			catalogStorage: storage,
			contextUrl:     contextUrl,
		}}
}

func (self *Registration) ldify() Registration {
	rc := self.copy()
	for i, res := range rc.Resources {
		rc.Resources[i] = res.ldify()
	}
	rc.Id = fmt.Sprintf("%v/%v", CatalogBaseUrl, self.Id)
	return rc
}

func (self *Resource) ldify() Resource {
	resc := self.copy()
	resc.Id = fmt.Sprintf("%v/%v", CatalogBaseUrl, self.Id)
	resc.Device = fmt.Sprintf("%v/%v", CatalogBaseUrl, self.Device)
	return resc
}

func (self *Registration) unLdify() Registration {
	rc := self.copy()
	for i, res := range rc.Resources {
		rc.Resources[i] = res.unLdify()
	}
	rc.Id = strings.TrimPrefix(self.Id, CatalogBaseUrl+"/")
	return rc
}

func (self *Resource) unLdify() Resource {
	resc := self.copy()
	resc.Id = strings.TrimPrefix(self.Id, CatalogBaseUrl+"/")
	resc.Device = strings.TrimPrefix(self.Device, CatalogBaseUrl+"/")
	return resc
}

func (self ReadableCatalogAPI) collectionFromRegistrations(registrations []Registration) *Collection {
	devices := make(map[string]Registration)
	resources := make([]Resource, 0, self.catalogStorage.getResourcesCount())

	for _, reg := range registrations {
		regld := reg.ldify()
		for _, res := range regld.Resources {
			resources = append(resources, res)
		}
		regld.Resources = nil
		devices[regld.Id] = regld
	}

	// TODO: create paged collection if len(entries) > x
	return &Collection{
		Context:   self.contextUrl,
		Id:        CatalogBaseUrl,
		Type:      "Collection",
		Devices:   devices,
		Resources: resources,
	}
}

// NOTE: this is inefficient, might need to reconsider how we return resources filter/query
func (self ReadableCatalogAPI) collectionFromResources(resources []Resource) *Collection {
	// registrations := make([]Registration, 0, self.catalogStorage.getRegistrationsCount())
	// added := make(map[string]struct{})
	// for _, res := range resources {
	// 	// skip already encountered devices
	// 	if _, ok := added[res.Device]; !ok {
	// 		added[res.Device] = struct{}{}

	// 		reg, _ := self.catalogStorage.get(res.Device)
	// 		registrations = append(registrations, reg)
	// 	}
	// }
	// return self.collectionFromRegistrations(registrations)
	devices := make(map[string]Registration)
	resourcesld := make([]Resource, 0, len(resources))
	for _, res := range resources {
		resld := res.ldify()
		resourcesld = append(resourcesld, resld)
		// skip already encountered devices
		if _, ok := devices[res.Device]; !ok {
			reg, _ := self.catalogStorage.get(res.Device)
			regld := reg.ldify()

			regld.Resources = nil
			devices[regld.Id] = regld
		}
	}

	// TODO: create paged collection if len(entries) > x
	return &Collection{
		Context:   self.contextUrl,
		Id:        CatalogBaseUrl,
		Type:      "Collection",
		Devices:   devices,
		Resources: resourcesld,
	}
}

func (self ReadableCatalogAPI) List(w http.ResponseWriter, req *http.Request) {
	registrations, _ := self.catalogStorage.getAll()
	coll := self.collectionFromRegistrations(registrations)

	b, _ := json.Marshal(coll)
	w.Header().Set("Content-Type", "application/ld+json;version="+CurrentApiVersion)
	w.Write(b)
}

func (self ReadableCatalogAPI) Filter(w http.ResponseWriter, req *http.Request) {
	ftype := req.URL.Query().Get(PatternFType)
	fpath := req.URL.Query().Get(PatternFPath)
	fop := req.URL.Query().Get(PatternFOp)
	fvalue := req.URL.Query().Get(PatternFValue)

	var data interface{}
	var err error
	matched := false

	switch ftype {
	case FTypeRegistration:
		data, err = self.catalogStorage.pathFilterRegistration(fpath, fop, fvalue)
		if data.(Registration).Id != "" {
			reg := data.(Registration)
			data = reg.ldify()
			matched = true
		}

	case FTypeRegistrations:
		data, err = self.catalogStorage.pathFilterRegistrations(fpath, fop, fvalue)
		if len(data.([]Registration)) > 0 {
			data = self.collectionFromRegistrations(data.([]Registration))
			matched = true
		}

	case FTypeResource:
		data, err = self.catalogStorage.pathFilterResource(fpath, fop, fvalue)
		if data.(Resource).Id != "" {
			res := data.(Resource)
			data = res.ldify()
			matched = true
		}

	case FTypeResources:
		data, err = self.catalogStorage.pathFilterResources(fpath, fop, fvalue)
		if len(data.([]Resource)) > 0 {
			data = self.collectionFromResources(data.([]Resource))
			matched = true
		}
	}

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Error processing the request: %s\n", err.Error())
	}

	if matched == false {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "Not found\n")
		return
	}

	b, _ := json.Marshal(data)
	w.Header().Set("Content-Type", "application/ld+json;version="+CurrentApiVersion)
	w.Write(b)
}

func (self ReadableCatalogAPI) Get(w http.ResponseWriter, req *http.Request) {
	id := fmt.Sprintf("%v/%v", req.URL.Query().Get(PatternUuid), req.URL.Query().Get(PatternReg))

	r, err := self.catalogStorage.get(id)
	if err != nil || r.Id == "" {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "Registration not found\n")
		return
	}

	b, _ := json.Marshal(r.ldify())

	w.Header().Set("Content-Type", "application/ld+json;version="+CurrentApiVersion)
	w.Write(b)
	return
}

func (self ReadableCatalogAPI) GetResource(w http.ResponseWriter, req *http.Request) {
	regid := fmt.Sprintf("%v/%v", req.URL.Query().Get(PatternUuid), req.URL.Query().Get(PatternReg))
	resname := req.URL.Query().Get(PatternRes)

	// check if registration regid exists
	r, err := self.catalogStorage.get(regid)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "Registration not found\n")
		return
	}

	// check if it has a resource regid
	rs, err := r.getResourceByName(resname)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "Resource not found\n")
		return
	}

	b, _ := json.Marshal(rs.ldify())
	w.Header().Set("Content-Type", "application/ld+json;version="+CurrentApiVersion)
	w.Write(b)
	return
}

func (self WritableCatalogAPI) Add(w http.ResponseWriter, req *http.Request) {
	body, err := ioutil.ReadAll(req.Body)
	req.Body.Close()

	var r Registration
	err = json.Unmarshal(body, &r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Error processing the request: %s\n", err.Error())
		return
	}

	ra, err := self.catalogStorage.add(r)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error creating the registration: %s\n", err.Error())
		return
	}

	b, _ := json.Marshal(ra.ldify())
	w.Header().Set("Content-Type", "application/ld+json;version="+CurrentApiVersion)
	w.WriteHeader(http.StatusCreated)
	w.Write(b)
	return
}

func (self WritableCatalogAPI) Update(w http.ResponseWriter, req *http.Request) {
	id := fmt.Sprintf("%v/%v", req.URL.Query().Get(PatternUuid), req.URL.Query().Get(PatternReg))

	body, err := ioutil.ReadAll(req.Body)
	req.Body.Close()

	var r Registration
	err = json.Unmarshal(body, &r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Error processing the request:: %s\n", err.Error())
		return
	}

	ru, err := self.catalogStorage.update(id, r)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error updating the registration: %s\n", err.Error())
		return
	}

	if ru.Id == "" {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "Not found\n")
		return
	}

	b, _ := json.Marshal(ru.ldify())
	w.Header().Set("Content-Type", "application/ld+json;version="+CurrentApiVersion)
	w.WriteHeader(http.StatusOK)
	w.Write(b)

	return
}

func (self WritableCatalogAPI) Delete(w http.ResponseWriter, req *http.Request) {
	id := fmt.Sprintf("%v/%v", req.URL.Query().Get(PatternUuid), req.URL.Query().Get(PatternReg))

	rd, err := self.catalogStorage.delete(id)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error deleting the registration: %s\n", err.Error())
		return
	}

	if rd.Id == "" {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "Not found\n")
		return
	}

	b, _ := json.Marshal(rd.ldify())
	w.Header().Set("Content-Type", "application/ld+json;version="+CurrentApiVersion)
	w.WriteHeader(http.StatusOK)
	w.Write(b)
	return
}
