package service

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"

	"github.com/patchwork-toolkit/patchwork/catalog"
)

const (
	PatternReg      = ":regid"
	PatternHostid   = ":hostid"
	PatternFType    = ":type"
	PatternFPath    = ":path"
	PatternFOp      = ":op"
	PatternFValue   = ":value"
	GetParamPage    = "page"
	GetParamPerPage = "per_page"
	FTypeService    = "service"
	FTypeServices   = "services"
	CtxRootDir      = "/ctx"
	CtxPathCatalog  = "/catalog.jsonld"
)

type Collection struct {
	Context  string    `json:"@context,omitempty"`
	Id       string    `json:"id"`
	Type     string    `json:"type"`
	Services []Service `json:"services"`
	Page     int       `json:"page"`
	PerPage  int       `json:"per_page"`
	Total    int       `json:"total"`
}

// Read-only catalog api
type ReadableCatalogAPI struct {
	catalogStorage CatalogStorage
	apiLocation    string
	ctxPathRoot    string
}

// Writable catalog api
type WritableCatalogAPI struct {
	*ReadableCatalogAPI
}

func NewReadableCatalogAPI(storage CatalogStorage, apiLocation, staticLocation string) *ReadableCatalogAPI {
	return &ReadableCatalogAPI{
		catalogStorage: storage,
		apiLocation:    apiLocation,
		ctxPathRoot:    staticLocation + CtxRootDir,
	}
}

func NewWritableCatalogAPI(storage CatalogStorage, apiLocation, staticLocation string) *WritableCatalogAPI {
	return &WritableCatalogAPI{
		&ReadableCatalogAPI{
			catalogStorage: storage,
			apiLocation:    apiLocation,
			ctxPathRoot:    staticLocation + CtxRootDir,
		}}
}

func (self *Service) ldify(apiLocation string) Service {
	sc := self.copy()
	sc.Id = fmt.Sprintf("%v/%v", apiLocation, self.Id)
	return sc
}

func (self *Service) unLdify(apiLocation string) Service {
	sc := self.copy()
	sc.Id = strings.TrimPrefix(self.Id, apiLocation+"/")
	return sc
}

func (self ReadableCatalogAPI) collectionFromServices(services []Service, page, perPage, total int) *Collection {
	respServices := make([]Service, 0, len(services))
	for _, svc := range services {
		svcld := svc.ldify(self.apiLocation)
		respServices = append(respServices, svcld)
	}

	return &Collection{
		Context:  self.ctxPathRoot + CtxPathCatalog,
		Id:       self.apiLocation,
		Type:     ApiCollectionType,
		Services: respServices,
		Page:     page,
		PerPage:  perPage,
		Total:    total,
	}
}

func (self ReadableCatalogAPI) List(w http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	page, _ := strconv.Atoi(req.Form.Get(GetParamPage))
	perPage, _ := strconv.Atoi(req.Form.Get(GetParamPerPage))
	page, perPage = catalog.ValidatePagingParams(page, perPage, MaxPerPage)

	services, total, _ := self.catalogStorage.getMany(page, perPage)
	coll := self.collectionFromServices(services, page, perPage, total)

	b, _ := json.Marshal(coll)
	w.Header().Set("Content-Type", "application/ld+json;version="+ApiVersion)
	w.Write(b)
}

func (self ReadableCatalogAPI) Filter(w http.ResponseWriter, req *http.Request) {
	ftype := req.URL.Query().Get(PatternFType)
	fpath := req.URL.Query().Get(PatternFPath)
	fop := req.URL.Query().Get(PatternFOp)
	fvalue := req.URL.Query().Get(PatternFValue)

	req.ParseForm()
	page, _ := strconv.Atoi(req.Form.Get(GetParamPage))
	perPage, _ := strconv.Atoi(req.Form.Get(GetParamPerPage))
	page, perPage = catalog.ValidatePagingParams(page, perPage, MaxPerPage)

	var data interface{}
	var err error

	switch ftype {
	case FTypeService:
		data, err = self.catalogStorage.pathFilterOne(fpath, fop, fvalue)
		if data.(Service).Id != "" {
			svc := data.(Service)
			data = svc.ldify(self.apiLocation)
		}

	case FTypeServices:
		var total int
		data, total, err = self.catalogStorage.pathFilter(fpath, fop, fvalue, page, perPage)
		data = self.collectionFromServices(data.([]Service), page, perPage, total)
	}

	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Error processing the request: %s\n", err.Error())
	}

	b, _ := json.Marshal(data)
	w.Header().Set("Content-Type", "application/ld+json;version="+ApiVersion)
	w.Write(b)
}

func (self ReadableCatalogAPI) Get(w http.ResponseWriter, req *http.Request) {
	id := fmt.Sprintf("%v/%v", req.URL.Query().Get(PatternHostid), req.URL.Query().Get(PatternReg))

	r, err := self.catalogStorage.get(id)
	if err == ErrorNotFound {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "Service not found\n")
		return
	} else if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error requesting the service: %s\n", err.Error())
		return
	}

	b, _ := json.Marshal(r.ldify(self.apiLocation))

	w.Header().Set("Content-Type", "application/ld+json;version="+ApiVersion)
	w.Write(b)
	return
}

func (self WritableCatalogAPI) Add(w http.ResponseWriter, req *http.Request) {
	body, err := ioutil.ReadAll(req.Body)
	req.Body.Close()

	var s Service
	err = json.Unmarshal(body, &s)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Error processing the request: %s\n", err.Error())
		return
	}

	err = self.catalogStorage.add(s)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error creating the service: %s\n", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/ld+json;version="+ApiVersion)
	w.Header().Set("Location", fmt.Sprintf("%s/%s", self.apiLocation, s.Id))
	w.WriteHeader(http.StatusCreated)
	return
}

func (self WritableCatalogAPI) Update(w http.ResponseWriter, req *http.Request) {
	id := fmt.Sprintf("%v/%v", req.URL.Query().Get(PatternHostid), req.URL.Query().Get(PatternReg))

	body, err := ioutil.ReadAll(req.Body)
	req.Body.Close()

	var s Service
	err = json.Unmarshal(body, &s)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Error processing the request:: %s\n", err.Error())
		return
	}

	err = self.catalogStorage.update(id, s)
	if err == ErrorNotFound {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "Service not found\n")
		return
	} else if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error requesting the service: %s\n", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/ld+json;version="+ApiVersion)
	w.WriteHeader(http.StatusOK)
	return
}

func (self WritableCatalogAPI) Delete(w http.ResponseWriter, req *http.Request) {
	id := fmt.Sprintf("%v/%v", req.URL.Query().Get(PatternHostid), req.URL.Query().Get(PatternReg))

	err := self.catalogStorage.delete(id)
	if err == ErrorNotFound {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "Not found\n")
		return
	} else if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error deleting the device: %s\n", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/ld+json;version="+ApiVersion)
	w.WriteHeader(http.StatusOK)
	return
}
