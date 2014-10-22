package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"mime"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/bmizerany/pat"
	catalog "github.com/patchwork-toolkit/patchwork/catalog/device"
)

const (
	StaticLocation  = "/static"
	CatalogLocation = "/dc"
)

type errorResponse struct {
	Error string `json:"error"`
}

type RESTfulAPI struct {
	config     *Config
	restConfig *RestProtocol
	router     *pat.PatternServeMux
	dataCh     chan<- DataRequest
}

func newRESTfulAPI(conf *Config, dataCh chan<- DataRequest) *RESTfulAPI {
	restConfig, _ := conf.Protocols[ProtocolTypeREST].(RestProtocol)

	api := &RESTfulAPI{
		config:     conf,
		restConfig: &restConfig,
		router:     pat.New(),
		dataCh:     dataCh,
	}
	return api
}

func (self *RESTfulAPI) start(catalogStorage catalog.CatalogStorage) {
	self.mountCatalog(catalogStorage)
	self.mountResources()
	self.router.Get(self.restConfig.Location, self.indexHandler())
	self.router.Get(StaticLocation+"/", self.staticHandler())

	// Mount router to server
	serverMux := http.NewServeMux()
	serverMux.Handle("/", self.router)

	s := &http.Server{
		Handler:        serverMux,
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   30 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	addr := fmt.Sprintf("%v:%v", self.config.Http.BindAddr, self.config.Http.BindPort)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Println(err.Error())
		return
	}

	log.Printf("Starting server at http://%v%v", addr, self.restConfig.Location)

	err = s.Serve(ln)
	if err != nil {
		log.Println(err.Error())
	}
}

func (self *RESTfulAPI) indexHandler() http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		b, _ := json.Marshal("Welcome to Device Gateway RESTful API")
		rw.Header().Set("Content-Type", "application/json")
		rw.Write(b)
	}
}

func (self *RESTfulAPI) staticHandler() http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {

		// serve all /static/ctx files as ld+json
		if strings.HasPrefix(req.URL.Path, "/static/ctx") {
			rw.Header().Set("Content-Type", "application/ld+json")
		}
		filePath := strings.Join(strings.Split(req.URL.Path, "/")[2:], "/")
		http.ServeFile(rw, req, self.config.StaticDir+"/"+filePath)
	}
}

func (self *RESTfulAPI) mountResources() {
	for _, device := range self.config.Devices {
		for _, resource := range device.Resources {
			for _, protocol := range resource.Protocols {
				if protocol.Type != ProtocolTypeREST {
					continue
				}
				uri := self.restConfig.Location + "/" + device.Name + "/" + resource.Name
				log.Println("RESTfulAPI: Mounting resource:", uri)
				rid := device.ResourceId(resource.Name)
				for _, method := range protocol.Methods {
					switch method {
					case "GET":
						self.router.Get(uri, self.createResourceGetHandler(rid))
					case "PUT":
						self.router.Put(uri, self.createResourcePutHandler(rid))
					}
				}
			}
		}
	}
}

func (self *RESTfulAPI) mountCatalog(catalogStorage catalog.CatalogStorage) {
	catalogAPI := catalog.NewReadableCatalogAPI(catalogStorage, CatalogLocation, StaticLocation)

	self.router.Get(fmt.Sprintf("%s/%s/%s/%s/%s",
		CatalogLocation, catalog.PatternFType, catalog.PatternFPath, catalog.PatternFOp, catalog.PatternFValue),
		http.HandlerFunc(catalogAPI.Filter))

	self.router.Get(fmt.Sprintf("%s/%s/%s/%s",
		CatalogLocation, catalog.PatternUuid, catalog.PatternReg, catalog.PatternRes),
		http.HandlerFunc(catalogAPI.GetResource))

	self.router.Get(fmt.Sprintf("%s/%s/%s",
		CatalogLocation, catalog.PatternUuid, catalog.PatternReg),
		http.HandlerFunc(catalogAPI.Get))

	self.router.Get(CatalogLocation, http.HandlerFunc(catalogAPI.List))
	log.Printf("Mounted local catalog at %v", CatalogLocation)
}

func (self *RESTfulAPI) createResourceGetHandler(resourceId string) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		log.Printf("RESTfulAPI: %s %s", req.Method, req.RequestURI)

		// Resolve mediaType
		v := req.Header.Get("Content-Type")
		mediaType, _, err := mime.ParseMediaType(v)
		if err != nil {
			self.respondWithBadRequest(rw, err.Error())
			return
		}

		// Check if mediaType is supported by resource
		isSupported := false
		resource, found := self.config.FindResource(resourceId)
		if !found {
			self.respondWithNotFound(rw, "Resource does not exist")
			return
		}
		for _, p := range resource.Protocols {
			if p.Type == ProtocolTypeREST {
				isSupported = true
			}
		}
		if !isSupported {
			self.respondWithUnsupportedMediaType(rw, "Media type is not supported by this resource")
			return
		}

		// Retrieve data
		dr := DataRequest{
			ResourceId: resourceId,
			Type:       DataRequestTypeRead,
			Arguments:  nil,
			Reply:      make(chan AgentResponse),
		}
		self.dataCh <- dr

		// Wait for the response
		repl := <-dr.Reply

		// Response to client
		rw.Header().Set("Content-Type", mediaType)
		if repl.IsError {
			self.respondWithInternalServerError(rw, string(repl.Payload))
			return
		}
		rw.Write(repl.Payload)
	}
}

func (self *RESTfulAPI) createResourcePutHandler(resourceId string) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		log.Printf("RESTfulAPI: %s %s", req.Method, req.RequestURI)

		// Resolve mediaType
		v := req.Header.Get("Content-Type")
		mediaType, _, err := mime.ParseMediaType(v)
		if err != nil {
			self.respondWithBadRequest(rw, err.Error())
			return
		}

		// Check if mediaType is supported by resource
		isSupported := false
		resource, found := self.config.FindResource(resourceId)
		if !found {
			self.respondWithNotFound(rw, "Resource does not exist")
			return
		}
		for _, p := range resource.Protocols {
			if p.Type == ProtocolTypeREST {
				isSupported = true
			}
		}
		if !isSupported {
			self.respondWithUnsupportedMediaType(rw, "Media type is not supported by this resource")
			return
		}

		// Extract PUT's body
		body, err := ioutil.ReadAll(req.Body)
		req.Body.Close()
		if err != nil {
			self.respondWithBadRequest(rw, err.Error())
			return
		}

		// Submit data request
		dr := DataRequest{
			ResourceId: resourceId,
			Type:       DataRequestTypeWrite,
			Arguments:  body,
			Reply:      make(chan AgentResponse),
		}
		log.Printf("RESTfulAPI: Submitting data request %#v", dr)
		self.dataCh <- dr

		// Wait for the response
		repl := <-dr.Reply

		// Respond to client
		rw.Header().Set("Content-Type", mediaType)
		if repl.IsError {
			self.respondWithInternalServerError(rw, string(repl.Payload))
			return
		}
		rw.WriteHeader(http.StatusNoContent)
	}
}

func (self *RESTfulAPI) respondWithNotFound(rw http.ResponseWriter, msg string) {
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(http.StatusNotFound)
	err := &errorResponse{Error: msg}
	b, _ := json.Marshal(err)
	rw.Write(b)
}

func (self *RESTfulAPI) respondWithBadRequest(rw http.ResponseWriter, msg string) {
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(http.StatusBadRequest)
	err := &errorResponse{Error: msg}
	b, _ := json.Marshal(err)
	rw.Write(b)
}

func (self *RESTfulAPI) respondWithUnsupportedMediaType(rw http.ResponseWriter, msg string) {
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(http.StatusUnsupportedMediaType)
	err := &errorResponse{Error: msg}
	b, _ := json.Marshal(err)
	rw.Write(b)
}

func (self *RESTfulAPI) respondWithInternalServerError(rw http.ResponseWriter, msg string) {
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(http.StatusInternalServerError)
	err := &errorResponse{Error: msg}
	b, _ := json.Marshal(err)
	rw.Write(b)
}
