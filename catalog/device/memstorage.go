package device

import (
	"errors"
	"github.com/patchwork-toolkit/patchwork/catalog"
	"log"
	"strings"
	"sync"
	"time"
)

// In-memory storage
type MemoryStorage struct {
	data  map[string]Registration
	mutex sync.RWMutex
}

// CRUD
func (self *MemoryStorage) add(r Registration) (Registration, error) {
	if r.Id == "" || len(strings.Split(r.Id, "/")) != 2 {
		return Registration{}, errors.New("Registration ID has to be <uuid>/<name>")
	}

	for i, res := range r.Resources {
		r.Resources[i].Device = r.Id
		if res.Id == "" || len(strings.Split(res.Id, "/")) != 3 {
			return Registration{}, errors.New("Resource ID has to be <uuid>/<name>/<resource>")
		}
	}

	r.Created = time.Now()
	r.Updated = r.Created
	if r.Ttl >= 0 {
		r.Expires = r.Created.Add(time.Duration(r.Ttl) * time.Second)
	}

	self.mutex.Lock()
	self.data[r.Id] = r
	self.mutex.Unlock()

	return r, nil
}

// Empty registration and nil error should be interpreted as "not found"
func (self *MemoryStorage) update(id string, r Registration) (Registration, error) {
	var ru Registration

	self.mutex.Lock()

	ru, ok := self.data[id]
	if !ok {
		self.mutex.Unlock()
		return ru, nil
	}

	ru.Type = r.Type
	ru.Name = r.Name
	ru.Description = r.Description
	ru.Ttl = r.Ttl
	ru.Updated = time.Now()
	if r.Ttl >= 0 {
		ru.Expires = ru.Updated.Add(time.Duration(r.Ttl) * time.Second)
	}
	ru.Resources = r.Resources
	self.data[id] = ru
	self.mutex.Unlock()

	return ru, nil
}

// Empty registration and nil error should be interpreted as "not found"
func (self *MemoryStorage) delete(id string) (Registration, error) {
	self.mutex.Lock()

	rd, ok := self.data[id]
	if !ok {
		self.mutex.Unlock()
		return rd, nil
	}
	delete(self.data, id)
	self.mutex.Unlock()

	return rd, nil
}

// Empty registration and nil error should be interpreted as "not found"
func (self *MemoryStorage) get(id string) (Registration, error) {
	self.mutex.RLock()
	r, ok := self.data[id]
	if !ok {
		self.mutex.RUnlock()
		return r, nil
	}
	self.mutex.RUnlock()
	return r, nil
}

// Utility
func (self *MemoryStorage) getAll() ([]Registration, error) {
	self.mutex.RLock()
	regs := make([]Registration, 0, len(self.data))
	for _, r := range self.data {
		regs = append(regs, r)
	}
	self.mutex.RUnlock()
	return regs, nil
}

func (self *MemoryStorage) getRegistrationsCount() int {
	self.mutex.RLock()
	l := len(self.data)
	self.mutex.RUnlock()
	return l
}

// Returns the total number of resources (from all devices)
func (self *MemoryStorage) getResourcesCount() int {
	var count int
	self.mutex.RLock()
	for _, reg := range self.data {
		count += len(reg.Resources)
	}
	self.mutex.RUnlock()
	return count
}

// Clean all remote registrations which expire time is larger than the given timestamp
func (self *MemoryStorage) cleanExpired(timestamp time.Time) {
	// log.Printf("Storage cleaner: will clean up all entries expired after %v", timestamp)
	self.mutex.Lock()
	for id, reg := range self.data {
		if reg.Ttl >= 0 && !reg.Expires.After(timestamp) {
			log.Printf("Storage cleaner: registration %v has expired\n", id)
			delete(self.data, id)
		}
	}
	self.mutex.Unlock()
}

// Path filtering
func (self *MemoryStorage) pathFilterRegistration(path string, op string, value string) (Registration, error) {
	var r Registration
	pathTknz := strings.Split(path, ".")

	self.mutex.RLock()
	// return the first one found
	for _, reg := range self.data {
		matched, err := catalog.MatchObject(reg, pathTknz, op, value)
		if err != nil {
			self.mutex.RUnlock()
			return r, err
		}
		if matched {
			self.mutex.RUnlock()
			return reg, nil
		}
	}
	self.mutex.RUnlock()
	return r, nil
}

func (self *MemoryStorage) pathFilterRegistrations(path string, op string, value string) ([]Registration, error) {
	regs := make([]Registration, 0, len(self.data))
	pathTknz := strings.Split(path, ".")

	self.mutex.RLock()
	for _, reg := range self.data {
		matched, err := catalog.MatchObject(reg, pathTknz, op, value)
		if err != nil {
			self.mutex.RUnlock()
			return regs, err
		}
		if matched {
			regs = append(regs, reg)
		}
	}
	self.mutex.RUnlock()
	return regs, nil
}

func (self *MemoryStorage) pathFilterResource(path string, op string, value string) (Resource, error) {
	var r Resource
	pathTknz := strings.Split(path, ".")

	self.mutex.RLock()
	// return the first one found
	for _, reg := range self.data {
		for _, res := range reg.Resources {
			matched, err := catalog.MatchObject(res, pathTknz, op, value)
			if err != nil {
				self.mutex.RUnlock()
				return r, err
			}
			if matched {
				self.mutex.RUnlock()
				return res, nil
			}
		}
	}
	return r, nil
}

func (self *MemoryStorage) pathFilterResources(path string, op string, value string) ([]Resource, error) {
	ress := make([]Resource, 0, self.getResourcesCount())
	pathTknz := strings.Split(path, ".")

	self.mutex.RLock()
	for _, reg := range self.data {
		for _, res := range reg.Resources {
			matched, err := catalog.MatchObject(res, pathTknz, op, value)
			if err != nil {
				self.mutex.RUnlock()
				return ress, err
			}
			if matched {
				ress = append(ress, res)
			}
		}
	}
	self.mutex.RUnlock()
	return ress, nil
}

func NewCatalogMemoryStorage() *MemoryStorage {
	storage := &MemoryStorage{
		data:  make(map[string]Registration),
		mutex: sync.RWMutex{},
	}

	// schedule cleaner
	t := time.Tick(time.Duration(5) * time.Second)
	go func() {
		for now := range t {
			storage.cleanExpired(now)
		}
	}()

	return storage
}
