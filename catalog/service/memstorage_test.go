package service

import (
	"strings"
	"testing"
)

func TestNewCatalogStorage(t *testing.T) {
	storage := NewCatalogMemoryStorage()
	if storage == nil {
		t.Fail()
	}
}

func TestAddRegistration(t *testing.T) {
	r := &Registration{}
	uuid := "E9203BE9-D705-42A8-8B12-F28E7EA2FC99"
	r.Id = uuid + "/" + "ServiceName"

	storage := NewCatalogMemoryStorage()
	ra, err := storage.add(*r)
	if err != nil {
		t.Errorf("Received unexpected error: %v", err.Error())
	}
	if ra.Id == "" {
		t.Error("Registration's Id should not be empty")
	}

	// id should be uuid/<id>
	spid := strings.Split(ra.Id, "/")
	if len(spid) != 2 {
		t.Fail()
	}
}

func TestUpdateRegistration(t *testing.T) {
	r := &Registration{}
	uuid := "E9203BE9-D705-42A8-8B12-F28E7EA2FC99"
	r.Id = uuid + "/" + "ServiceName"

	storage := NewCatalogMemoryStorage()
	ra, err := storage.add(*r)
	if err != nil {
		t.Errorf("Unexpected error on add: %v", err.Error())
	}
	ra.Name = "UpdatedName"
	spid := strings.Split(ra.Id, "/")

	if len(spid) != 2 {
		t.Fail()
	}

	ru, err := storage.update(ra.Id, ra)
	if err != nil {
		t.Error("Unexpected error on update: %v", err.Error())
	}

	if ru.Name != "UpdatedName" {
		t.Fail()
	}
}

func TestGetRegistration(t *testing.T) {
	r := &Registration{
		Name: "TestName",
	}
	uuid := "E9203BE9-D705-42A8-8B12-F28E7EA2FC99"
	r.Id = uuid + "/" + "ServiceName"

	storage := NewCatalogMemoryStorage()
	ra, err := storage.add(*r)
	if err != nil {
		t.Errorf("Unexpected error on add: %v", err.Error())
	}
	spid := strings.Split(ra.Id, "/")

	if len(spid) != 2 {
		t.Fail()
	}

	rg, err := storage.get(ra.Id)
	if err != nil {
		t.Error("Unexpected error on get: %v", err.Error())
	}

	if rg.Name != "TestName" {
		t.Fail()
	}
}

func TestDeleteRegistration(t *testing.T) {
	r := &Registration{}
	uuid := "E9203BE9-D705-42A8-8B12-F28E7EA2FC99"
	r.Id = uuid + "/" + "ServiceName"
	storage := NewCatalogMemoryStorage()
	ra, err := storage.add(*r)
	if err != nil {
		t.Errorf("Unexpected error on add: %v", err.Error())
	}
	spid := strings.Split(ra.Id, "/")

	if len(spid) != 2 {
		t.Fail()
	}

	_, err = storage.delete(ra.Id)
	if err != nil {
		t.Error("Unexpected error on delete: %v", err.Error())
	}

	rd, err := storage.delete(ra.Id)
	if err != nil {
		t.Error("Unexpected error on delete: %v", err.Error())
	}
	if rd.Id != "" {
		t.Error("The previous call hasn't deleted the registration?")
	}
}
