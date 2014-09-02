package service

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

func (self *MemoryStorage) getCount() int {
	self.mutex.RLock()
	l := len(self.data)
	self.mutex.RUnlock()
	return l
}

// Clean all remote registrations which expire time is larger than the given timestamp
func (self *MemoryStorage) cleanExpired(timestamp time.Time) {
	self.mutex.Lock()
	for id, reg := range self.data {
		if reg.Ttl >= 0 && !reg.Expires.After(timestamp) {
			log.Printf("In-memory storage cleaner: registration %v has expired\n", id)
			delete(self.data, id)
		}
	}
	self.mutex.Unlock()
}

// Path filtering
// Filter one registration
func (self *MemoryStorage) pathFilterOne(path string, op string, value string) (Registration, error) {
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

// Filter multiple registrations
func (self *MemoryStorage) pathFilter(path string, op string, value string) ([]Registration, error) {
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
