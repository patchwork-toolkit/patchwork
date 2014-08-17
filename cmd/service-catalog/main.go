package main

import (
	"flag"
	"fmt"
	"github.com/bmizerany/pat"
	catalog "github.com/patchwork-toolkit/patchwork/catalog/service"
	"github.com/patchwork-toolkit/patchwork/discovery"
	"log"
	"net/http"
	"strconv"
	"strings"
)

var (
	confPath  = flag.String("confPath", "conf/device-catalog.json", "Configuration file path")
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
		log.Fatalf("Error reading config file %v:%v", *confPath, err)
	}
	staticDir = config.StaticDir

	var cat catalog.CatalogStorage
	switch config.Storage {
	case "memory":
		cat = catalog.NewCatalogMemoryStorage()
	}
	api := catalog.NewWritableCatalogAPI(cat, "/static/ctx/catalog.jsonld")

	m := pat.New()
	// writable api
	m.Post(catalog.CatalogBaseUrl+"/", http.HandlerFunc(api.Add))

	m.Get(fmt.Sprintf("%s/%s/%s",
		catalog.CatalogBaseUrl, catalog.PatternHostid, catalog.PatternReg),
		http.HandlerFunc(api.Get))

	m.Get(fmt.Sprintf("%s/%s/%s/%s/%s",
		catalog.CatalogBaseUrl, catalog.PatternFType, catalog.PatternFPath, catalog.PatternFOp, catalog.PatternFValue),
		http.HandlerFunc(api.Filter))

	m.Put(fmt.Sprintf("%s/%s/%s",
		catalog.CatalogBaseUrl, catalog.PatternHostid, catalog.PatternReg),
		http.HandlerFunc(api.Update))

	m.Del(fmt.Sprintf("%s/%s/%s",
		catalog.CatalogBaseUrl, catalog.PatternHostid, catalog.PatternReg),
		http.HandlerFunc(api.Delete))

	m.Get(catalog.CatalogBaseUrl, http.HandlerFunc(api.List))

	// static
	m.Get("/static/", http.HandlerFunc(staticHandler))

	http.Handle("/", m)

	// Announce service using DNS-SD
	if config.DnssdEnabled {
		parts := strings.Split(config.Endpoint, ":")
		port, _ := strconv.Atoi(parts[1])
		_, err := discovery.DnsRegisterService(config.Name, catalog.DnssdServiceType, port)
		if err != nil {
			log.Printf("Failed to perform DNS-SD registration: %v\n", err.Error())
		}
	}

	log.Printf("Started standalone Service Catalog at %s%s", config.Endpoint, catalog.CatalogBaseUrl)
	log.Fatal(http.ListenAndServe(config.Endpoint, nil))
}
