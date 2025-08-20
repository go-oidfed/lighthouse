package main

// Deprecated: FileStorage is deprecated and will be removed in a future release.
// Use the GORM storage backend instead. See the README.md file for migration instructions.

import (
	"encoding/json"
	"os"
	"path"
	"slices"
	"sync"

	"github.com/pkg/errors"
	slices2 "tideland.dev/go/slices"

	"github.com/go-oidfed/lighthouse/storage"
	"github.com/go-oidfed/lighthouse/storage/model"
)

// FileStorage is a storage backend for storing things in files
type FileStorage struct {
	files map[string]*file
}

type file struct {
	path  string
	mutex sync.RWMutex
}

// NewFileStorage creates a new FileStorage at the given path
//
// Deprecated: Use the GORM storage backend instead. See the README.md file for migration instructions.
func NewFileStorage(basepath string) *FileStorage {
	return &FileStorage{
		files: map[string]*file{
			"subordinates":          {path: path.Join(basepath, "subordinates.json")},
			"trust_marked_entities": {path: path.Join(basepath, "trust_marked_entities.json")},
		},
	}
}

func (store *FileStorage) SubordinateStorage() loadLegacySubordinateInfos {
	return func() ([]legacySubordinateInfo, error) {
		data, err := os.ReadFile(store.files["subordinates"].path)
		if err != nil {
			return nil, err
		}
		var infosMap map[string]legacySubordinateInfo
		err = json.Unmarshal(data, &infosMap)
		if err != nil {
			return nil, err
		}
		infos := make([]legacySubordinateInfo, len(infosMap))
		i := 0
		for _, v := range infosMap {
			infos[i] = v
			i++
		}
		return infos, nil
	}
}

// TrustMarkedEntitiesStorage returns a file-based TrustMarkedEntitiesStorageBackend
func (store *FileStorage) TrustMarkedEntitiesStorage() storage.TrustMarkedEntitiesStorageBackend {
	return trustMarkedEntitiesFileStorage{store.files["trust_marked_entities"]}
}

func addToSliceIfNotExists[C comparable](item C, slice []C) []C {
	if !slices.Contains(slice, item) {
		slice = append(slice, item)
	}
	return slice
}

func removeFromSlice[C comparable](item C, slice []C) (out []C) {
	for _, i := range slice {
		if i != item {
			out = append(out, i)
		}
	}
	return
}

// trustMarkedEntitiesFileStorage is a file-based TrustMarkedEntitiesStorageBackend
type trustMarkedEntitiesFileStorage struct {
	*file
}

func (s trustMarkedEntitiesFileStorage) readUnlocked() (infos map[string]map[model.Status][]string, err error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	err = json.Unmarshal(data, &infos)
	if err != nil {
		// try to unmarshal legacy file format
		var legacyInfos map[string][]string
		if e := json.Unmarshal(data, &legacyInfos); e != nil {
			return nil, err
		}
		err = nil
		for k, v := range legacyInfos {
			mappedV := map[model.Status][]string{
				model.StatusActive: v,
			}
			infos[k] = mappedV
		}
	}
	return
}
func (s trustMarkedEntitiesFileStorage) writeUnlocked(infos map[string]map[model.Status][]string) (err error) {
	data, err := json.Marshal(infos)
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0600)
}

// Block implements the TrustMarkedEntitiesStorageBackend
func (s trustMarkedEntitiesFileStorage) Block(trustMarkType, entityID string) error {
	return s.write(trustMarkType, entityID, model.StatusBlocked)
}

// Approve implements the TrustMarkedEntitiesStorageBackend
func (s trustMarkedEntitiesFileStorage) Approve(trustMarkType, entityID string) error {
	return s.write(trustMarkType, entityID, model.StatusActive)
}

// Request implements the TrustMarkedEntitiesStorageBackend
func (s trustMarkedEntitiesFileStorage) Request(trustMarkType, entityID string) error {
	return s.write(trustMarkType, entityID, model.StatusPending)
}

// TrustMarkedStatus implements the TrustMarkedEntitiesStorageBackend
func (s trustMarkedEntitiesFileStorage) TrustMarkedStatus(trustMarkType, entityID string) (model.Status, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	infosMap, err := s.readUnlocked()
	if err != nil {
		return -1, err
	}
	infos, ok := infosMap[trustMarkType]
	if !ok {
		return model.StatusInactive, nil
	}
	for status, ids := range infos {
		if slices.Contains(ids, entityID) {
			return status, nil
		}
	}
	return model.StatusInactive, nil
}

func (s trustMarkedEntitiesFileStorage) Active(trustMarkType string) ([]string, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	infosMap, err := s.readUnlocked()
	if err != nil {
		return nil, err
	}
	if trustMarkType != "" {
		infos, ok := infosMap[trustMarkType]
		if !ok {
			return nil, nil
		}
		return infos[model.StatusActive], nil
	}
	var entityIDs []string
	for _, infos := range infosMap {
		ids, ok := infos[model.StatusActive]
		if !ok {
			continue
		}
		entityIDs = append(entityIDs, ids...)
	}
	return slices2.Unique(entityIDs), nil

}

func (s trustMarkedEntitiesFileStorage) Blocked(trustMarkType string) ([]string, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	infosMap, err := s.readUnlocked()
	if err != nil {
		return nil, err
	}
	infos, ok := infosMap[trustMarkType]
	if !ok {
		return nil, nil
	}
	return infos[model.StatusBlocked], nil
}

func (s trustMarkedEntitiesFileStorage) Pending(trustMarkType string) ([]string, error) {
	s.mutex.RLock()
	defer s.mutex.RUnlock()
	infosMap, err := s.readUnlocked()
	if err != nil {
		return nil, err
	}
	infos, ok := infosMap[trustMarkType]
	if !ok {
		return nil, nil
	}
	return infos[model.StatusPending], nil
}

func (s trustMarkedEntitiesFileStorage) write(trustMarkType, entityID string, status model.Status) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()
	infos, err := s.readUnlocked()
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if infos == nil {
		infos = make(map[string]map[model.Status][]string)
	}
	tme, ok := infos[trustMarkType]
	if !ok {
		tme = make(map[model.Status][]string)
	}
	// remove entityID from other status
	for st, entities := range tme {
		if st != status {
			tme[st] = removeFromSlice(entityID, entities)
		}
	}
	// add entityID to correct status
	entities, ok := tme[status]
	if !ok {
		entities = make([]string, 0)
	}
	entities = addToSliceIfNotExists(entityID, entities)
	tme[status] = entities

	infos[trustMarkType] = tme
	return s.writeUnlocked(infos)
}

// Delete implements the TrustMarkedEntitiesStorageBackend
func (s trustMarkedEntitiesFileStorage) Delete(trustMarkType, entityID string) error {
	return s.write(trustMarkType, entityID, -1)
}

// Load implements the TrustMarkedEntitiesStorageBackend
func (trustMarkedEntitiesFileStorage) Load() error {
	return nil
}

// HasTrustMark implements the TrustMarkedEntitiesStorageBackend
func (s trustMarkedEntitiesFileStorage) HasTrustMark(trustMarkType, entityID string) (bool, error) {
	tme, err := s.Active(trustMarkType)
	if err != nil {
		return false, err
	}
	if tme == nil {
		return false, nil
	}
	return slices.Contains(tme, entityID), nil
}
