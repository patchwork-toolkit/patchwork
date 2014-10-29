package main

import (
	"flag"
	"fmt"
	"log"
	"strconv"

	"github.com/patchwork-toolkit/patchwork/Godeps/_workspace/src/github.com/codegangsta/negroni"
	"github.com/patchwork-toolkit/patchwork/Godeps/_workspace/src/github.com/gorilla/mux"
	"github.com/patchwork-toolkit/patchwork/Godeps/_workspace/src/github.com/oleksandr/bonjour"
	utils "github.com/patchwork-toolkit/patchwork/catalog"
	catalog "github.com/patchwork-toolkit/patchwork/catalog/service"
)

var (
	confPath = flag.String("conf", "conf/service-catalog.json", "Service catalog configuration file path")
)

func main() {
	flag.Parse()

	config, err := loadConfig(*confPath)
	if err != nil {
		log.Fatalf("Error reading config file %v: %v", *confPath, err)
	}

	r, err := setupRouter(config)
	if err != nil {
		log.Fatal(err.Error())
	}

	// Announce service using DNS-SD
	var bonjourCh chan<- bool
	if config.DnssdEnabled {
		bonjourCh, err = bonjour.Register(config.Description,
			catalog.DnssdServiceType,
			"",
			config.BindPort,
			[]string{fmt.Sprintf("uri=%s", config.ApiLocation)},
			nil)
		if err != nil {
			log.Printf("Failed to register DNS-SD service: %s", err.Error())
		} else {
			log.Println("Registered service via DNS-SD using type", catalog.DnssdServiceType)
			defer func(ch chan<- bool) {
				ch <- true
			}(bonjourCh)
		}
	}

	// Configure the middleware
	n := negroni.New(
		negroni.NewRecovery(),
		negroni.NewLogger(),
	)
	// Mount router
	n.UseHandler(r)

	// Start listener
	endpoint := fmt.Sprintf("%s:%s", config.BindAddr, strconv.Itoa(config.BindPort))
	log.Printf("Starting standalone Service Catalog at %v%v", endpoint, config.ApiLocation)
	n.Run(endpoint)
}

func setupRouter(config *Config) (*mux.Router, error) {
	// Create catalog API object
	var api *catalog.WritableCatalogAPI
	if config.Storage.Type == utils.CatalogBackendMemory {
		api = catalog.NewWritableCatalogAPI(
			catalog.NewMemoryStorage(),
			config.ApiLocation,
			utils.StaticLocation,
			config.Description,
		)
	}
	if api == nil {
		return nil, fmt.Errorf("Could not create catalog API structure. Unsupported storage type: %v", config.Storage.Type)
	}

	// Configure routers
	r := mux.NewRouter().StrictSlash(true)

	// NB: For now the order of routes IS IMPORTANT!!!

	scr := r.PathPrefix(config.ApiLocation).Subrouter()

	r.Methods("GET").PathPrefix(utils.StaticLocation).HandlerFunc(utils.NewStaticHandler(config.StaticDir))

	scr.Methods("GET").Path("/{type}/{path}/{op}/{value}").HandlerFunc(api.Filter)

	scr.Methods("GET").Path("/").HandlerFunc(api.List)
	scr.Methods("POST").Path("/").HandlerFunc(api.Add)

	regr := scr.PathPrefix("/{hostid}/{regid}").Subrouter()
	regr.Methods("GET").HandlerFunc(api.Get)
	regr.Methods("PUT").HandlerFunc(api.Update)
	regr.Methods("DELETE").HandlerFunc(api.Delete)

	return r, nil
}
