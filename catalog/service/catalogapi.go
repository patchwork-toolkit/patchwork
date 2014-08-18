package service

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
)

const (
	PatternReg         = ":regid"
	PatternHostid      = ":hostid"
	PatternFType       = ":type"
	PatternFPath       = ":path"
	PatternFOp         = ":op"
	PatternFValue      = ":value"
	FTypeRegistration  = "service"
	FTypeRegistrations = "services"
	CurrentApiVersion  = "0.1.0"
)

type Collection struct {
	Context  string         `json:"@context,omitempty"`
	Id       string         `json:"id"`
	Type     string         `json:"type"`
	Services []Registration `json:"services"`
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
	rc.Id = fmt.Sprintf("%v/%v", CatalogBaseUrl, self.Id)
	return rc
}

func (self *Registration) unLdify() Registration {
	rc := self.copy()
	rc.Id = strings.TrimPrefix(self.Id, CatalogBaseUrl+"/")
	return rc
}

func (self ReadableCatalogAPI) collectionFromRegistrations(registrations []Registration) *Collection {
	services := make([]Registration, 0, self.catalogStorage.getCount())
	for _, reg := range registrations {
		regld := reg.ldify()
		services = append(services, regld)
	}

	// TODO: create paged collection if len(entries) > x
	return &Collection{
		Context:  self.contextUrl,
		Id:       CatalogBaseUrl,
		Type:     "Collection",
		Services: services,
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
		data, err = self.catalogStorage.pathFilterOne(fpath, fop, fvalue)
		if data.(Registration).Id != "" {
			reg := data.(Registration)
			data = reg.ldify()
			matched = true
		}

	case FTypeRegistrations:
		data, err = self.catalogStorage.pathFilter(fpath, fop, fvalue)
		if len(data.([]Registration)) > 0 {
			data = self.collectionFromRegistrations(data.([]Registration))
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
	id := fmt.Sprintf("%v/%v", req.URL.Query().Get(PatternHostid), req.URL.Query().Get(PatternReg))

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
	id := fmt.Sprintf("%v/%v", req.URL.Query().Get(PatternHostid), req.URL.Query().Get(PatternReg))

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
	id := fmt.Sprintf("%v/%v", req.URL.Query().Get(PatternHostid), req.URL.Query().Get(PatternReg))

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
