package service

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/patchwork-toolkit/patchwork/catalog"
)

// In-memory storage
type MemoryStorage struct {
	data  map[string]Service
	index []string
	mutex sync.RWMutex
}

// CRUD
func (self *MemoryStorage) add(s Service) error {
	if !s.validate() {
		return fmt.Errorf("Invalid Service registration")
	}

	s.Created = time.Now()
	s.Updated = s.Created
	if s.Ttl >= 0 {
		s.Expires = s.Created.Add(time.Duration(s.Ttl) * time.Second)
	}

	self.mutex.Lock()
	self.data[s.Id] = s
	self.reindexEntries()
	self.mutex.Unlock()

	return nil
}

func (self *MemoryStorage) update(id string, s Service) error {
	self.mutex.Lock()

	su, ok := self.data[id]
	if !ok {
		self.mutex.Unlock()
		return ErrorNotFound
	}

	su.Type = s.Type
	su.Name = s.Name
	su.Description = s.Description
	su.Ttl = s.Ttl
	su.Updated = time.Now()
	if s.Ttl >= 0 {
		su.Expires = su.Updated.Add(time.Duration(s.Ttl) * time.Second)
	}
	self.data[id] = su
	self.mutex.Unlock()

	return nil
}

func (self *MemoryStorage) delete(id string) error {
	self.mutex.Lock()

	_, ok := self.data[id]
	if !ok {
		self.mutex.Unlock()
		return ErrorNotFound
	}
	delete(self.data, id)
	self.reindexEntries()
	self.mutex.Unlock()

	return nil
}

func (self *MemoryStorage) get(id string) (Service, error) {
	self.mutex.RLock()
	s, ok := self.data[id]
	if !ok {
		self.mutex.RUnlock()
		return s, ErrorNotFound
	}
	self.mutex.RUnlock()
	return s, nil
}

// Utility

func (self *MemoryStorage) getMany(page int, perPage int) ([]Service, int, error) {
	self.mutex.RLock()
	total := len(self.data)
	keys := catalog.GetPageOfSlice(self.index, page, perPage, MaxPerPage)

	if len(keys) == 0 {
		self.mutex.RUnlock()
		return []Service{}, total, nil
	}

	svcs := make([]Service, 0, len(keys))
	for _, k := range keys {
		svcs = append(svcs, self.data[k])
	}
	self.mutex.RUnlock()
	return svcs, total, nil
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
	for id, svc := range self.data {
		if svc.Ttl >= 0 && !svc.Expires.After(timestamp) {
			logger.Printf("MemoryStorage.cleanExpired() Registration %v has expired\n", id)
			delete(self.data, id)
		}
	}
	self.mutex.Unlock()
}

// Path filtering
// Filter one registration
func (self *MemoryStorage) pathFilterOne(path string, op string, value string) (Service, error) {
	pathTknz := strings.Split(path, ".")

	self.mutex.RLock()
	// return the first one found
	for _, svc := range self.data {
		matched, err := catalog.MatchObject(svc, pathTknz, op, value)
		if err != nil {
			self.mutex.RUnlock()
			return Service{}, err
		}
		if matched {
			self.mutex.RUnlock()
			return svc, nil
		}
	}
	self.mutex.RUnlock()
	return Service{}, nil
}

// Filter multiple registrations
func (self *MemoryStorage) pathFilter(path, op, value string, page, perPage int) ([]Service, int, error) {
	matchedIds := []string{}
	pathTknz := strings.Split(path, ".")

	self.mutex.RLock()
	for _, svc := range self.data {
		matched, err := catalog.MatchObject(svc, pathTknz, op, value)
		if err != nil {
			self.mutex.RUnlock()
			return []Service{}, 0, err
		}
		if matched {
			matchedIds = append(matchedIds, svc.Id)
		}
	}

	keys := catalog.GetPageOfSlice(matchedIds, page, perPage, MaxPerPage)
	if len(keys) == 0 {
		self.mutex.RUnlock()
		return []Service{}, len(matchedIds), nil
	}

	svcs := make([]Service, 0, len(keys))
	for _, k := range keys {
		svcs = append(svcs, self.data[k])
	}
	self.mutex.RUnlock()
	return svcs, len(matchedIds), nil
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

func NewMemoryStorage() *MemoryStorage {
	storage := &MemoryStorage{
		data:  make(map[string]Service),
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
