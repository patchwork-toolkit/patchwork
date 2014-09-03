package service

import (
	"errors"
	"github.com/patchwork-toolkit/patchwork/catalog"
	"log"
	"sort"
	"strings"
	"sync"
	"time"
)

// In-memory storage
type MemoryStorage struct {
	data  map[string]Registration
	index []string
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
	self.reindexEntries()
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
	self.reindexEntries()
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

func (self *MemoryStorage) getMany(page int, perPage int) ([]Registration, int, error) {
	keys := []string{}

	// Never return more than the defined maximum
	if perPage > MaxPerPage || perPage == 0 {
		perPage = MaxPerPage
	}

	self.mutex.RLock()
	total := len(self.data)

	// if 1, not specified or negative - return the first page
	if page < 2 {
		// first page
		if perPage > total {
			keys = self.index
		} else {
			keys = self.index[:perPage]
		}
	} else if page == int(total/perPage)+1 {
		// last page
		keys = self.index[perPage*(page-1):]

	} else if page <= total/perPage && page*perPage <= total {
		// slice
		r := page * perPage
		l := r - perPage
		keys = self.index[l:r]
	} else {
		self.mutex.RUnlock()
		return []Registration{}, total, nil
	}

	regs := make([]Registration, 0, len(keys))
	for _, k := range keys {
		regs = append(regs, self.data[k])
	}
	self.mutex.RUnlock()
	return regs, total, nil
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
	self.mutex.RLock()
	regs := make([]Registration, 0, len(self.data))
	pathTknz := strings.Split(path, ".")

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

// Re-index the map entries.
// WARNING: the caller must obtain the lock before calling
func (self *MemoryStorage) reindexEntries() {
	self.index = make([]string, 0, len(self.data))
	for _, reg := range self.data {
		self.index = append(self.index, reg.Id)
	}
	// sort
	sort.Strings(self.index)
}

func NewCatalogMemoryStorage() *MemoryStorage {
	storage := &MemoryStorage{
		data:  make(map[string]Registration),
		index: []string{},
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
