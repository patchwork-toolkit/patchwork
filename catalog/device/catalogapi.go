package device

import (
	"encoding/json"
	"fmt"
	"github.com/patchwork-toolkit/patchwork/catalog"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
)

const (
	PatternReg      = ":regid"
	PatternRes      = ":resname"
	PatternUuid     = ":uuid"
	PatternFType    = ":type"
	PatternFPath    = ":path"
	PatternFOp      = ":op"
	PatternFValue   = ":value"
	FTypeDevice     = "device"
	FTypeDevices    = "devices"
	FTypeResource   = "resource"
	FTypeResources  = "resources"
	GetParamPage    = "page"
	GetParamPerPage = "per_page"
	CtxRootDir      = "/ctx"
	CtxPathCatalog  = "/catalog.jsonld"
)

type Collection struct {
	Context   string                 `json:"@context,omitempty"`
	Id        string                 `json:"id"`
	Type      string                 `json:"type"`
	Devices   map[string]EmptyDevice `json:"devices"`
	Resources []Resource             `json:"resources"`
	Page      int                    `json:"page"`
	PerPage   int                    `json:"per_page"`
	Total     int                    `json:"total"`
}

// Device object with empty resources
type EmptyDevice struct {
	*Device
	Resources []Resource `json:"resources,omitempty"`
}

// Device object with paginated resources
type PaginatedDevice struct {
	*Device
	Resources []Resource `json:"resources"`
	Page      int        `json:"page"`
	PerPage   int        `json:"per_page"`
	Total     int        `json:"total"`
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

func (self *Device) ldify(apiLocation string) Device {
	rc := self.copy()
	for i, res := range rc.Resources {
		rc.Resources[i] = res.ldify(apiLocation)
	}
	rc.Id = fmt.Sprintf("%v/%v", apiLocation, self.Id)
	return rc
}

func (self *Resource) ldify(apiLocation string) Resource {
	resc := self.copy()
	resc.Id = fmt.Sprintf("%v/%v", apiLocation, self.Id)
	resc.Device = fmt.Sprintf("%v/%v", apiLocation, self.Device)
	return resc
}

func (self *Device) unLdify(apiLocation string) Device {
	rc := self.copy()
	for i, res := range rc.Resources {
		rc.Resources[i] = res.unLdify(apiLocation)
	}
	rc.Id = strings.TrimPrefix(self.Id, apiLocation+"/")
	return rc
}

func (self *Resource) unLdify(apiLocation string) Resource {
	resc := self.copy()
	resc.Id = strings.TrimPrefix(self.Id, apiLocation+"/")
	resc.Device = strings.TrimPrefix(self.Device, apiLocation+"/")
	return resc
}

func (self ReadableCatalogAPI) collectionFromDevices(devices []Device, page, perPage, total int) *Collection {
	respDevices := make(map[string]EmptyDevice)
	respResources := make([]Resource, 0, self.catalogStorage.getResourcesCount())

	for _, d := range devices {
		dld := d.ldify(self.apiLocation)
		for _, res := range dld.Resources {
			respResources = append(respResources, res)
		}

		respDevices[d.Id] = EmptyDevice{
			&dld,
			nil,
		}
	}

	return &Collection{
		Context:   self.ctxPathRoot + CtxPathCatalog,
		Id:        self.apiLocation,
		Type:      ApiCollectionType,
		Devices:   respDevices,
		Resources: respResources,
		Page:      page,
		PerPage:   perPage,
		Total:     total,
	}
}

func (self ReadableCatalogAPI) paginatedDeviceFromDevice(d Device, page, perPage int) *PaginatedDevice {
	pd := &PaginatedDevice{
		&d,
		make([]Resource, 0, len(d.Resources)),
		page,
		perPage,
		len(d.Resources),
	}

	resourceIds := make([]string, 0, len(d.Resources))
	for _, r := range d.Resources {
		resourceIds = append(resourceIds, r.Id)
	}

	pageResourceIds := catalog.GetPageOfSlice(resourceIds, page, perPage, MaxPerPage)
	for _, id := range pageResourceIds {
		for _, r := range d.Resources {
			if r.Id == id {
				pd.Resources = append(pd.Resources, r.ldify(self.apiLocation))
			}
		}
	}

	return pd
}

func (self ReadableCatalogAPI) List(w http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	page, _ := strconv.Atoi(req.Form.Get(GetParamPage))
	perPage, _ := strconv.Atoi(req.Form.Get(GetParamPerPage))
	page, perPage = catalog.ValidatePagingParams(page, perPage, MaxPerPage)

	devices, total, _ := self.catalogStorage.getMany(page, perPage)
	coll := self.collectionFromDevices(devices, page, perPage, total)

	b, _ := json.Marshal(coll)
	w.Header().Set("Content-Type", "application/ld+json;version="+ApiVersion)
	w.Write(b)

	return
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

	var (
		data  interface{}
		err   error
		total int
	)

	switch ftype {
	case FTypeDevice:
		data, err = self.catalogStorage.pathFilterDevice(fpath, fop, fvalue)
		if data.(Device).Id != "" {
			data = self.paginatedDeviceFromDevice(data.(Device), page, perPage)
		}

	case FTypeDevices:
		data, total, err = self.catalogStorage.pathFilterDevices(fpath, fop, fvalue, page, perPage)
		data = self.collectionFromDevices(data.([]Device), page, perPage, total)

	case FTypeResource:
		data, err = self.catalogStorage.pathFilterResource(fpath, fop, fvalue)
		if data.(Resource).Id != "" {
			res := data.(Resource)
			data = res.ldify(self.apiLocation)
		}

	case FTypeResources:
		data, total, err = self.catalogStorage.pathFilterResources(fpath, fop, fvalue, page, perPage)
		devs := self.catalogStorage.devicesFromResources(data.([]Resource))
		data = self.collectionFromDevices(devs, page, perPage, total)
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
	req.ParseForm()
	page, _ := strconv.Atoi(req.Form.Get(GetParamPage))
	perPage, _ := strconv.Atoi(req.Form.Get(GetParamPerPage))
	page, perPage = catalog.ValidatePagingParams(page, perPage, MaxPerPage)

	id := fmt.Sprintf("%v/%v", req.URL.Query().Get(PatternUuid), req.URL.Query().Get(PatternReg))

	d, err := self.catalogStorage.get(id)
	if err == ErrorNotFound {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "Device not found\n")
		return
	} else if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error requesting the device: %s\n", err.Error())
		return
	}

	pd := self.paginatedDeviceFromDevice(d, page, perPage)
	b, _ := json.Marshal(pd)

	w.Header().Set("Content-Type", "application/ld+json;version="+ApiVersion)
	w.Write(b)
	return
}

func (self ReadableCatalogAPI) GetResource(w http.ResponseWriter, req *http.Request) {
	devid := fmt.Sprintf("%v/%v", req.URL.Query().Get(PatternUuid), req.URL.Query().Get(PatternReg))
	resid := fmt.Sprintf("%v/%v", devid, req.URL.Query().Get(PatternRes))

	// check if device devid exists
	_, err := self.catalogStorage.get(devid)
	if err == ErrorNotFound {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "Registration not found\n")
		return
	} else if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error requesting the device: %s\n", err.Error())
		return
	}

	// check if it has a resource resid
	res, err := self.catalogStorage.getResourceById(resid)
	if err == ErrorNotFound {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "Registration not found\n")
		return
	} else if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error requesting the resource: %s\n", err.Error())
		return
	}

	b, _ := json.Marshal(res.ldify(self.apiLocation))
	w.Header().Set("Content-Type", "application/ld+json;version="+ApiVersion)
	w.Write(b)
	return
}

func (self WritableCatalogAPI) Add(w http.ResponseWriter, req *http.Request) {
	body, err := ioutil.ReadAll(req.Body)
	req.Body.Close()

	var d Device
	err = json.Unmarshal(body, &d)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Error processing the request: %s\n", err.Error())
		return
	}

	err = self.catalogStorage.add(d)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error creating the registration: %s\n", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/ld+json;version="+ApiVersion)
	w.Header().Set("Location", fmt.Sprintf("%s/%s", self.apiLocation, d.Id))
	w.WriteHeader(http.StatusCreated)
	return
}

func (self WritableCatalogAPI) Update(w http.ResponseWriter, req *http.Request) {
	id := fmt.Sprintf("%v/%v", req.URL.Query().Get(PatternUuid), req.URL.Query().Get(PatternReg))

	body, err := ioutil.ReadAll(req.Body)
	req.Body.Close()

	var d Device
	err = json.Unmarshal(body, &d)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Error processing the request:: %s\n", err.Error())
		return
	}

	err = self.catalogStorage.update(id, d)
	if err == ErrorNotFound {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "Not found\n")
		return
	} else if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error updating the device: %s\n", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/ld+json;version="+ApiVersion)
	w.WriteHeader(http.StatusOK)
	return
}

func (self WritableCatalogAPI) Delete(w http.ResponseWriter, req *http.Request) {
	id := fmt.Sprintf("%v/%v", req.URL.Query().Get(PatternUuid), req.URL.Query().Get(PatternReg))

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
