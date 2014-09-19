package service

import (
	"testing"
)

func TestNewStorage(t *testing.T) {
	storage := NewMemoryStorage()
	if storage == nil {
		t.Fail()
	}
}

func TestAddService(t *testing.T) {
	r := &Service{}
	uuid := "E9203BE9-D705-42A8-8B12-F28E7EA2FC99"
	r.Id = uuid + "/" + "ServiceName"

	storage := NewMemoryStorage()
	err := storage.add(*r)
	if err != nil {
		t.Errorf("Received unexpected error: %v", err.Error())
	}
}

func TestUpdateService(t *testing.T) {
	r := &Service{}
	uuid := "E9203BE9-D705-42A8-8B12-F28E7EA2FC99"
	r.Id = uuid + "/" + "ServiceName"

	storage := NewMemoryStorage()
	err := storage.add(*r)
	if err != nil {
		t.Errorf("Unexpected error on add: %v", err.Error())
	}
	r.Name = "UpdatedName"

	err = storage.update(r.Id, *r)
	if err != nil {
		t.Errorf("Unexpected error on update: %v", err.Error())
	}

}

func TestGetService(t *testing.T) {
	r := &Service{
		Name: "TestName",
	}
	uuid := "E9203BE9-D705-42A8-8B12-F28E7EA2FC99"
	r.Id = uuid + "/" + "ServiceName"

	storage := NewMemoryStorage()
	err := storage.add(*r)
	if err != nil {
		t.Errorf("Unexpected error on add: %v", err.Error())
	}

	rg, err := storage.get(r.Id)
	if err != nil {
		t.Error("Unexpected error on get: %v", err.Error())
	}

	if rg.Name != "TestName" {
		t.Fail()
	}
}

func TestDeleteService(t *testing.T) {
	r := &Service{}
	uuid := "E9203BE9-D705-42A8-8B12-F28E7EA2FC99"
	r.Id = uuid + "/" + "ServiceName"
	storage := NewMemoryStorage()
	err := storage.add(*r)
	if err != nil {
		t.Errorf("Unexpected error on add: %v", err.Error())
	}

	err = storage.delete(r.Id)
	if err != nil {
		t.Error("Unexpected error on delete: %v", err.Error())
	}

	err = storage.delete(r.Id)
	if err != ErrorNotFound {
		t.Error("The previous call hasn't deleted the Service?")
	}
}

func TestGetManyServices(t *testing.T) {
	r := &Service{}
	storage := NewMemoryStorage()
	// Add 10 entries
	for i := 0; i < 11; i++ {
		r.Id = "TestID" + "/" + string(i)
		err := storage.add(*r)

		if err != nil {
			t.Errorf("Unexpected error on add: %v", err.Error())
		}
	}

	p1pp2, total, _ := storage.getMany(1, 2)
	if total != 11 {
		t.Errorf("Expected total is 11, returned: %v", total)
	}

	if len(p1pp2) != 2 {
		t.Errorf("Wrong number of entries: requested page=1 , perPage=2. Expected: 2, returned: %v", len(p1pp2))
	}

	p2pp2, _, _ := storage.getMany(2, 2)
	if len(p2pp2) != 2 {
		t.Errorf("Wrong number of entries: requested page=2 , perPage=2. Expected: 2, returned: %v", len(p2pp2))
	}

	p2pp5, _, _ := storage.getMany(2, 5)
	if len(p2pp5) != 5 {
		t.Errorf("Wrong number of entries: requested page=2 , perPage=5. Expected: 5, returned: %v", len(p2pp5))
	}

	p4pp3, _, _ := storage.getMany(4, 3)
	if len(p4pp3) != 2 {
		t.Errorf("Wrong number of entries: requested page=4 , perPage=3. Expected: 2, returned: %v", len(p4pp3))
	}
}
