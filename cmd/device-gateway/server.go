package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"mime"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/patchwork-toolkit/patchwork/Godeps/_workspace/src/github.com/codegangsta/negroni"
	"github.com/patchwork-toolkit/patchwork/Godeps/_workspace/src/github.com/gorilla/mux"
	catalog "github.com/patchwork-toolkit/patchwork/catalog/device"
)

// errorResponse used to serialize errors into JSON for RESTful responses
type errorResponse struct {
	Error string `json:"error"`
}

// RESTfulAPI contains all required configuration for running a RESTful API
// for device gateway
type RESTfulAPI struct {
	config     *Config
	restConfig *RestProtocol
	router     *mux.Router
	dataCh     chan<- DataRequest
}

// Constructs a RESTfulAPI data structure
func newRESTfulAPI(conf *Config, dataCh chan<- DataRequest) *RESTfulAPI {
	restConfig, _ := conf.Protocols[ProtocolTypeREST].(RestProtocol)

	api := &RESTfulAPI{
		config:     conf,
		restConfig: &restConfig,
		router:     mux.NewRouter().StrictSlash(true),
		dataCh:     dataCh,
	}
	return api
}

// Setup all routers, handlers and start a HTTP server (blocking call)
func (api *RESTfulAPI) start(catalogStorage catalog.CatalogStorage) {
	api.mountCatalog(catalogStorage)
	api.mountResources()

	api.router.Methods("GET").PathPrefix(StaticLocation).HandlerFunc(api.staticHandler())
	api.router.Methods("GET", "POST").Path("/dashboard").HandlerFunc(api.dashboardHandler(*confPath))
	api.router.Methods("GET").Path(api.restConfig.Location).HandlerFunc(api.indexHandler())

	// Configure the middleware
	n := negroni.New(
		negroni.NewRecovery(),
		negroni.NewLogger(),
	)
	// Mount router
	n.UseHandler(api.router)

	// Start the listener
	addr := fmt.Sprintf("%v:%v", api.config.Http.BindAddr, api.config.Http.BindPort)
	logger.Printf("RESTfulAPI.start() Starting server at http://%v%v", addr, api.restConfig.Location)
	n.Run(addr)
}

// Create a HTTP handler to serve and update dashboard configuration
func (api *RESTfulAPI) dashboardHandler(confPath string) http.HandlerFunc {
	dashboardConfPath := filepath.Join(filepath.Dir(confPath), "dashboard.json")

	return func(rw http.ResponseWriter, req *http.Request) {
		rw.Header().Set("Content-Type", "application/json")

		if req.Method == "POST" {
			body, err := ioutil.ReadAll(req.Body)
			req.Body.Close()
			if err != nil {
				api.respondWithBadRequest(rw, err.Error())
				return
			}

			err = ioutil.WriteFile(dashboardConfPath, body, 0755)
			if err != nil {
				rw.WriteHeader(http.StatusInternalServerError)
				errData := map[string]string{"error": err.Error()}
				b, _ := json.Marshal(errData)
				rw.Write(b)
				return
			}

			rw.WriteHeader(http.StatusCreated)
			rw.Write([]byte("{}"))

		} else if req.Method == "GET" {
			data, err := ioutil.ReadFile(dashboardConfPath)
			if err != nil {
				rw.WriteHeader(http.StatusInternalServerError)
				errData := map[string]string{"error": err.Error()}
				b, _ := json.Marshal(errData)
				rw.Write(b)
				return
			}
			rw.WriteHeader(http.StatusOK)
			rw.Write(data)
		} else {
			rw.WriteHeader(http.StatusMethodNotAllowed)
		}
	}
}

func (api *RESTfulAPI) indexHandler() http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		b, _ := json.Marshal("Welcome to Device Gateway RESTful API")
		rw.Header().Set("Content-Type", "application/json")
		rw.Write(b)
	}
}

func (api *RESTfulAPI) staticHandler() http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {

		// serve all /static/ctx files as ld+json
		if strings.HasPrefix(req.URL.Path, "/static/ctx") {
			rw.Header().Set("Content-Type", "application/ld+json")
		}
		filePath := strings.Join(strings.Split(req.URL.Path, "/")[2:], "/")
		http.ServeFile(rw, req, api.config.StaticDir+"/"+filePath)
	}
}

func (api *RESTfulAPI) mountResources() {
	for _, device := range api.config.Devices {
		for _, resource := range device.Resources {
			for _, protocol := range resource.Protocols {
				if protocol.Type != ProtocolTypeREST {
					continue
				}
				uri := api.restConfig.Location + "/" + device.Name + "/" + resource.Name
				logger.Println("RESTfulAPI.mountResources() Mounting resource:", uri)
				rid := device.ResourceId(resource.Name)
				for _, method := range protocol.Methods {
					switch method {
					case "GET":
						api.router.Methods("GET").Path(uri).HandlerFunc(api.createResourceGetHandler(rid))
					case "PUT":
						api.router.Methods("PUT").Path(uri).HandlerFunc(api.createResourcePutHandler(rid))
					}
				}
			}
		}
	}
}

func (api *RESTfulAPI) mountCatalog(catalogStorage catalog.CatalogStorage) {
	catalogAPI := catalog.NewReadableCatalogAPI(
		catalogStorage,
		CatalogLocation,
		StaticLocation,
		fmt.Sprintf("RESTfulAPI.mountCatalog() Local catalog at %s", api.config.Description),
	)

	api.router.Methods("GET").Path(CatalogLocation + "/{type}/{path}/{op}/{value}").HandlerFunc(catalogAPI.Filter).Name("filter")
	api.router.Methods("GET").Path(CatalogLocation + "/{dgwid}/{regid}/{resname}").HandlerFunc(catalogAPI.GetResource).Name("details")
	api.router.Methods("GET").Path(CatalogLocation + "/{dgwid}/{regid}").HandlerFunc(catalogAPI.Get).Name("get")
	api.router.Methods("GET").Path(CatalogLocation).HandlerFunc(catalogAPI.List).Name("list")

	logger.Printf("RESTfulAPI.mountCatalog() Mounted local catalog at %v", CatalogLocation)
}

func (api *RESTfulAPI) createResourceGetHandler(resourceId string) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		logger.Printf("RESTfulAPI.createResourceGetHandler() %s %s", req.Method, req.RequestURI)

		// Resolve mediaType
		v := req.Header.Get("Content-Type")
		mediaType, _, err := mime.ParseMediaType(v)
		if err != nil {
			api.respondWithBadRequest(rw, err.Error())
			return
		}

		// Check if mediaType is supported by resource
		isSupported := false
		resource, found := api.config.FindResource(resourceId)
		if !found {
			api.respondWithNotFound(rw, "Resource does not exist")
			return
		}
		for _, p := range resource.Protocols {
			if p.Type == ProtocolTypeREST {
				isSupported = true
			}
		}
		if !isSupported {
			api.respondWithUnsupportedMediaType(rw, "Media type is not supported by this resource")
			return
		}

		// Retrieve data
		dr := DataRequest{
			ResourceId: resourceId,
			Type:       DataRequestTypeRead,
			Arguments:  nil,
			Reply:      make(chan AgentResponse),
		}
		api.dataCh <- dr

		// Wait for the response
		repl := <-dr.Reply

		// Response to client
		rw.Header().Set("Content-Type", mediaType)
		if repl.IsError {
			api.respondWithInternalServerError(rw, string(repl.Payload))
			return
		}
		rw.Write(repl.Payload)
	}
}

func (api *RESTfulAPI) createResourcePutHandler(resourceId string) http.HandlerFunc {
	return func(rw http.ResponseWriter, req *http.Request) {
		logger.Printf("RESTfulAPI.createResourcePutHandler() %s %s", req.Method, req.RequestURI)

		// Resolve mediaType
		v := req.Header.Get("Content-Type")
		mediaType, _, err := mime.ParseMediaType(v)
		if err != nil {
			api.respondWithBadRequest(rw, err.Error())
			return
		}

		// Check if mediaType is supported by resource
		isSupported := false
		resource, found := api.config.FindResource(resourceId)
		if !found {
			api.respondWithNotFound(rw, "Resource does not exist")
			return
		}
		for _, p := range resource.Protocols {
			if p.Type == ProtocolTypeREST {
				isSupported = true
			}
		}
		if !isSupported {
			api.respondWithUnsupportedMediaType(rw, "Media type is not supported by this resource")
			return
		}

		// Extract PUT's body
		body, err := ioutil.ReadAll(req.Body)
		req.Body.Close()
		if err != nil {
			api.respondWithBadRequest(rw, err.Error())
			return
		}

		// Submit data request
		dr := DataRequest{
			ResourceId: resourceId,
			Type:       DataRequestTypeWrite,
			Arguments:  body,
			Reply:      make(chan AgentResponse),
		}
		logger.Printf("RESTfulAPI.createResourcePutHandler() Submitting data request %#v", dr)
		api.dataCh <- dr

		// Wait for the response
		repl := <-dr.Reply

		// Respond to client
		rw.Header().Set("Content-Type", mediaType)
		if repl.IsError {
			api.respondWithInternalServerError(rw, string(repl.Payload))
			return
		}
		rw.WriteHeader(http.StatusNoContent)
	}
}

func (api *RESTfulAPI) respondWithNotFound(rw http.ResponseWriter, msg string) {
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(http.StatusNotFound)
	err := &errorResponse{Error: msg}
	b, _ := json.Marshal(err)
	rw.Write(b)
}

func (api *RESTfulAPI) respondWithBadRequest(rw http.ResponseWriter, msg string) {
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(http.StatusBadRequest)
	err := &errorResponse{Error: msg}
	b, _ := json.Marshal(err)
	rw.Write(b)
}

func (api *RESTfulAPI) respondWithUnsupportedMediaType(rw http.ResponseWriter, msg string) {
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(http.StatusUnsupportedMediaType)
	err := &errorResponse{Error: msg}
	b, _ := json.Marshal(err)
	rw.Write(b)
}

func (api *RESTfulAPI) respondWithInternalServerError(rw http.ResponseWriter, msg string) {
	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(http.StatusInternalServerError)
	err := &errorResponse{Error: msg}
	b, _ := json.Marshal(err)
	rw.Write(b)
}
