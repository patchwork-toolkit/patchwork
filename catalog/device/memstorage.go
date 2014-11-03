package device

import (
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/patchwork-toolkit/patchwork/catalog"
)

// In-memory storage
type MemoryStorage struct {
	devices   map[string]StoredDevice
	resources map[string]Resource
	index     []string // index of resources
	mutex     sync.RWMutex
}

// Device object without embedded Resources
type StoredDevice struct {
	*Device
	Resources []string
}

// CRUD
func (self *MemoryStorage) add(d Device) error {
	if !d.validate() {
		return errors.New("Invalid Device registration")
	}

	sd := StoredDevice{
		&Device{
			Id:          d.Id,
			Type:        d.Type,
			Name:        d.Name,
			Meta:        d.Meta,
			Description: d.Description,
			Ttl:         d.Ttl,
		},
		[]string{},
	}
	sd.Created = time.Now()
	sd.Updated = sd.Created
	if d.Ttl >= 0 {
		sd.Expires = sd.Created.Add(time.Duration(sd.Ttl) * time.Second)
	}

	self.mutex.Lock()
	for _, res := range d.Resources {
		res.Device = sd.Id
		sd.Resources = append(sd.Resources, res.Id)
		self.resources[res.Id] = res
	}

	self.devices[sd.Id] = sd
	self.reindexResources()
	self.mutex.Unlock()

	return nil
}

func (self *MemoryStorage) update(id string, d Device) error {
	self.mutex.Lock()

	sd, ok := self.devices[id]
	if !ok {
		self.mutex.Unlock()
		return ErrorNotFound
	}

	sd.Type = d.Type
	sd.Name = d.Name
	sd.Description = d.Description
	sd.Ttl = d.Ttl
	sd.Updated = time.Now()
	if sd.Ttl >= 0 {
		sd.Expires = sd.Updated.Add(time.Duration(sd.Ttl) * time.Second)
	}

	sd.Resources = nil
	for _, res := range d.Resources {
		res.Device = sd.Id
		sd.Resources = append(sd.Resources, res.Id)
		self.resources[res.Id] = res
	}
	self.devices[sd.Id] = sd
	self.reindexResources() // device resources may change on update
	self.mutex.Unlock()

	return nil
}

func (self *MemoryStorage) delete(id string) error {
	self.mutex.Lock()
	sd, ok := self.devices[id]
	if !ok {
		self.mutex.Unlock()
		return ErrorNotFound
	}

	for _, res := range sd.Resources {
		delete(self.resources, res)
	}
	delete(self.devices, id)
	self.reindexResources()
	self.mutex.Unlock()

	return nil
}

func (self *MemoryStorage) get(id string) (Device, error) {
	self.mutex.RLock()
	sd, ok := self.devices[id]
	if !ok {
		self.mutex.RUnlock()
		return Device{}, ErrorNotFound
	}
	d := Device{
		Id:          sd.Id,
		Type:        sd.Type,
		Name:        sd.Name,
		Meta:        sd.Meta,
		Description: sd.Description,
		Ttl:         sd.Ttl,
		Created:     sd.Created,
		Updated:     sd.Updated,
		Expires:     sd.Expires,
		Resources:   []Resource{},
	}

	for _, rid := range sd.Resources {
		res, ok := self.resources[rid]
		if !ok {
			return d, fmt.Errorf("Device %s resource %s not found", id, rid)
		}
		d.Resources = append(d.Resources, res)
	}
	self.mutex.RUnlock()
	return d, nil
}

// Utility
func (self *MemoryStorage) getMany(page int, perPage int) ([]Device, int, error) {
	self.mutex.RLock()
	keys := catalog.GetPageOfSlice(self.index, page, perPage, MaxPerPage)
	total := len(self.resources)

	if len(keys) == 0 {
		self.mutex.RUnlock()
		return []Device{}, total, nil
	}

	ress := make([]Resource, 0, len(keys))
	for _, k := range keys {
		ress = append(ress, self.resources[k])
	}
	devs := self.devicesFromResources(ress)

	self.mutex.RUnlock()
	return devs, total, nil
}

func (self *MemoryStorage) getDevicesCount() int {
	self.mutex.RLock()
	l := len(self.devices)
	self.mutex.RUnlock()
	return l
}

// Returns the total number of resources (from all devices)
func (self *MemoryStorage) getResourcesCount() int {
	self.mutex.RLock()
	l := len(self.resources)
	self.mutex.RUnlock()
	return l
}

// Clean all remote registrations which expire time is larger than the given timestamp
func (self *MemoryStorage) cleanExpired(timestamp time.Time) {
	// log.Printf("Storage cleaner: will clean up all entries expired after %v", timestamp)
	self.mutex.Lock()
	for id, d := range self.devices {
		if d.Ttl >= 0 && !d.Expires.After(timestamp) {
			log.Printf("Storage cleaner: registration %v has expired\n", id)
			for _, rid := range d.Resources {
				delete(self.resources, rid)
			}
			delete(self.devices, id)
		}
	}
	self.mutex.Unlock()
}

// WARNING: the caller must obtain the lock before calling
func (self *MemoryStorage) reindexResources() {
	self.index = make([]string, 0, len(self.resources))
	for _, res := range self.resources {
		self.index = append(self.index, res.Id)
	}
	// sort
	sort.Strings(self.index)
}

func (self *MemoryStorage) getResourceById(id string) (Resource, error) {
	self.mutex.RLock()
	res, ok := self.resources[id]
	if !ok {
		self.mutex.RUnlock()
		return Resource{}, ErrorNotFound
	}
	self.mutex.RUnlock()
	return res, nil
}

func (self *MemoryStorage) devicesFromResources(resources []Resource) []Device {
	// Max len(devices) == len(resources)
	devs := make([]Device, 0, len(resources))
	added := make(map[string]bool)
	for _, r := range resources {
		did := self.resources[r.Id].Device
		_, ok := added[did]
		if !ok {
			added[did] = true
			d, _ := self.get(did)

			// only take resources that provided as input
			d.Resources = nil
			for _, r2 := range resources {
				if r2.Device == d.Id {
					d.Resources = append(d.Resources, r2)
				}
			}
			devs = append(devs, d)
		}
	}
	return devs
}

// Path filtering
func (self *MemoryStorage) pathFilterDevice(path, op, value string) (Device, error) {
	pathTknz := strings.Split(path, ".")

	self.mutex.RLock()
	// return the first one found
	for _, d := range self.devices {
		dev, _ := self.get(d.Id)
		matched, err := catalog.MatchObject(dev, pathTknz, op, value)
		if err != nil {
			self.mutex.RUnlock()
			return Device{}, err
		}
		if matched {
			self.mutex.RUnlock()
			return dev, nil
		}
	}
	self.mutex.RUnlock()
	return Device{}, nil
}

func (self *MemoryStorage) pathFilterDevices(path, op, value string, page, perPage int) ([]Device, int, error) {
	pathTknz := strings.Split(path, ".")

	self.mutex.RLock()
	resourceIds := make([]string, 0, len(self.resources))
	for _, d := range self.devices {
		dev, _ := self.get(d.Id)
		matched, err := catalog.MatchObject(dev, pathTknz, op, value)
		if err != nil {
			self.mutex.RUnlock()
			return []Device{}, 0, err
		}
		if matched {
			// save IDs of all resources of the matched device
			for _, r := range dev.Resources {
				resourceIds = append(resourceIds, r.Id)
			}
		}
	}

	// get the slice of resources as indicated by page
	pageResourceIds := catalog.GetPageOfSlice(resourceIds, page, perPage, MaxPerPage)
	ress := make([]Resource, 0, len(pageResourceIds))
	for _, i := range pageResourceIds {
		ress = append(ress, self.resources[i])
	}

	// convert to devices
	devs := self.devicesFromResources(ress)
	self.mutex.RUnlock()
	return devs, len(resourceIds), nil
}

func (self *MemoryStorage) pathFilterResource(path, op, value string) (Resource, error) {
	pathTknz := strings.Split(path, ".")

	self.mutex.RLock()
	// return the first one found
	for _, d := range self.devices {
		for _, rid := range d.Resources {
			res := self.resources[rid]
			matched, err := catalog.MatchObject(res, pathTknz, op, value)
			if err != nil {
				self.mutex.RUnlock()
				return Resource{}, err
			}
			if matched {
				self.mutex.RUnlock()
				return res, nil
			}
		}
	}
	self.mutex.RUnlock()
	return Resource{}, nil
}

func (self *MemoryStorage) pathFilterResources(path, op, value string, page, perPage int) ([]Resource, int, error) {
	self.mutex.RLock()
	resourceIds := make([]string, 0, len(self.resources))
	pathTknz := strings.Split(path, ".")

	for _, res := range self.resources {
		matched, err := catalog.MatchObject(res, pathTknz, op, value)
		if err != nil {
			self.mutex.RUnlock()
			return []Resource{}, 0, err
		}
		if matched {
			resourceIds = append(resourceIds, res.Id)
		}
	}

	pageResourceIds := catalog.GetPageOfSlice(resourceIds, page, perPage, MaxPerPage)
	ress := make([]Resource, 0, len(pageResourceIds))
	for _, id := range pageResourceIds {
		ress = append(ress, self.resources[id])
	}

	self.mutex.RUnlock()
	return ress, len(resourceIds), nil
}

func NewMemoryStorage() *MemoryStorage {
	storage := &MemoryStorage{
		devices:   make(map[string]StoredDevice),
		resources: make(map[string]Resource),
		index:     []string{},
		mutex:     sync.RWMutex{},
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
