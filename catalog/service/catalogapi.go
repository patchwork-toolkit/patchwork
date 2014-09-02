package service

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
)

const (
	PatternReg         = ":regid"
	PatternHostid      = ":hostid"
	PatternFType       = ":type"
	PatternFPath       = ":path"
	PatternFOp         = ":op"
	PatternFValue      = ":value"
	GetParamPage       = "page"
	GetParamPerPage    = "per_page"
	FTypeRegistration  = "service"
	FTypeRegistrations = "services"
	CurrentApiVersion  = "0.1.0"
)

type Collection struct {
	Context  string         `json:"@context,omitempty"`
	Id       string         `json:"id"`
	Type     string         `json:"type"`
	Services []Registration `json:"services"`
	Page     int            `json:"page"`
	PerPage  int            `json:"per_page"`
	Total    int            `json:"total"`
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

func (self ReadableCatalogAPI) collectionFromRegistrations(registrations []Registration, page int, perPage int, total int) *Collection {
	services := make([]Registration, 0, len(registrations))
	for _, reg := range registrations {
		regld := reg.ldify()
		services = append(services, regld)
	}

	return &Collection{
		Context:  self.contextUrl,
		Id:       CatalogBaseUrl,
		Type:     "Collection",
		Services: services,
		Page:     page,
		PerPage:  perPage,
		Total:    total,
	}
}

func (self ReadableCatalogAPI) List(w http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	page, _ := strconv.Atoi(req.Form.Get(GetParamPage))
	perPage, _ := strconv.Atoi(req.Form.Get(GetParamPerPage))

	// use defaults if not specified
	if page == 0 {
		page = 1
	}
	if perPage == 0 {
		perPage = MaxPerPage
	}

	registrations, total, _ := self.catalogStorage.getMany(page, perPage)
	coll := self.collectionFromRegistrations(registrations, page, perPage, total)

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
			// FIXME (affects both dc and sc)
			data = self.collectionFromRegistrations(data.([]Registration), 0, 0, 0)
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
