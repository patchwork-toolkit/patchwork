package main

import (
	"flag"
	"fmt"

	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/patchwork-toolkit/patchwork/Godeps/_workspace/src/github.com/oleksandr/bonjour"
	catalog "github.com/patchwork-toolkit/patchwork/catalog/service"
)

const (
	CatalogBackendMemory = "memory"
	StaticLocation       = "/static"
)

var (
	confPath  = flag.String("conf", "conf/service-catalog.json", "Service catalog configuration file path")
	staticDir = ""
)

func staticHandler(w http.ResponseWriter, req *http.Request) {
	if req.Method != "GET" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// serve all /static/ctx files as ld+json
	if strings.HasPrefix(req.URL.Path, "/static/ctx") {
		w.Header().Set("Content-Type", "application/ld+json")
	}
	urlParts := strings.Split(req.URL.Path, "/")
	http.ServeFile(w, req, staticDir+"/"+strings.Join(urlParts[2:], "/"))
}

func main() {
	flag.Parse()

	config, err := loadConfig(*confPath)
	if err != nil {
		log.Fatalf("Error reading config file %v: %v", *confPath, err)
	}
	staticDir = config.StaticDir

	var cat catalog.CatalogStorage

	switch config.Storage.Type {
	case CatalogBackendMemory:
		cat = catalog.NewMemoryStorage()
	}

	api := catalog.NewWritableCatalogAPI(cat, config.ApiLocation, StaticLocation, config.Description)

	// writable api
	http.HandleFunc(config.ApiLocation, api.Add)
	http.HandleFunc(fmt.Sprintf("%s/%s/%s",
		config.ApiLocation, catalog.PatternHostid, catalog.PatternReg),
		api.Get)
	http.HandleFunc(fmt.Sprintf("%s/%s/%s/%s/%s",
		config.ApiLocation, catalog.PatternFType, catalog.PatternFPath, catalog.PatternFOp, catalog.PatternFValue),
		api.Filter)
	http.HandleFunc(fmt.Sprintf("%s/%s/%s",
		config.ApiLocation, catalog.PatternHostid, catalog.PatternReg),
		api.Update)
	http.HandleFunc(fmt.Sprintf("%s/%s/%s",
		config.ApiLocation, catalog.PatternHostid, catalog.PatternReg),
		api.Delete)
	http.HandleFunc(config.ApiLocation, api.List)

	// static
	http.HandleFunc("/static", staticHandler)

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

	log.Printf("Starting standalone Service Catalog at %v:%v%v", config.BindAddr, config.BindPort, config.ApiLocation)

	// Listen and Serve
	endpoint := fmt.Sprintf("%v:%v", config.BindAddr, strconv.Itoa(config.BindPort))
	log.Fatal(http.ListenAndServe(endpoint, nil))
}
