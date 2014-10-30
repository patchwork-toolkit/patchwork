package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	catalog "github.com/patchwork-toolkit/patchwork/catalog/device"
)

func TestDemo(t *testing.T) {
	config, err := loadConfig(*confPath)
	if err != nil {
		t.Fatal(err.Error())
	}

	router, err := setupRouter(config)
	if err != nil {
		t.Fatal(err.Error())
	}

	ts := httptest.NewServer(router)
	defer ts.Close()

	url := ts.URL + "/dc"
	t.Log("Calling", url)
	res, err := http.Get(url)
	if err != nil {
		t.Fatal(err.Error())
	}

	if res.StatusCode != http.StatusOK {
		t.Fatalf("/dc should return OK 200, got instead: %v (%s)", res.StatusCode, res.Status)
	}

	if !strings.HasPrefix(res.Header.Get("Content-Type"), "application/ld+json") {
		t.Fatalf("/dc should have Content-Type: application/ld+json, got instead %s", res.Header.Get("Content-Type"))
	}

	var collection *catalog.Collection
	decoder := json.NewDecoder(res.Body)
	defer res.Body.Close()

	err = decoder.Decode(&collection)
	if err != nil {
		t.Fatal(err.Error())
	}

	if collection.Total > 0 {
		t.Fatal("/dc should return empty collection, but got total", collection.Total)
	}
}
