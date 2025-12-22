package model

// LegacySubordinateStorageBackend is an interface to store ExtendedSubordinateInfo
type LegacySubordinateStorageBackend interface {
	Write(entityID string, info ExtendedSubordinateInfo) error
	Delete(entityID string) error
	Block(entityID string) error
	Approve(entityID string) error
	Subordinate(entityID string) (*ExtendedSubordinateInfo, error)
	Active() LegacySubordinateStorageQuery
	Blocked() LegacySubordinateStorageQuery
	Pending() LegacySubordinateStorageQuery
	Load() error
}

// LegacySubordinateStorageQuery is an interface to query ExtendedSubordinateInfo from storage
type LegacySubordinateStorageQuery interface {
	Subordinates() ([]ExtendedSubordinateInfo, error)
	EntityIDs() ([]string, error)
	AddFilter(filter LegacySubordinateStorageQueryFilter, value any) error
}

// LegacySubordinateStorageQueryFilter is a function to filter ExtendedSubordinateInfo
type LegacySubordinateStorageQueryFilter func(info ExtendedSubordinateInfo, value any) bool

// SubordinateStorageBackend is an interface to store ExtendedSubordinateInfo
type SubordinateStorageBackend interface {
	Add(info ExtendedSubordinateInfo) error
	Update(entityID string, info ExtendedSubordinateInfo) error
	Delete(entityID string) error
	DeleteByDBID(id string) error
	UpdateStatus(entityID string, status Status) error
	UpdateStatusByDBID(id string, status Status) error
	Get(entityID string) (*ExtendedSubordinateInfo, error)
	GetByDBID(id string) (*ExtendedSubordinateInfo, error)
	GetAll() ([]BasicSubordinateInfo, error)
	GetByStatus(status Status) ([]BasicSubordinateInfo, error)
	GetByEntityTypes(entityTypes []string) ([]BasicSubordinateInfo, error)
	GetByAnyEntityType(entityTypes []string) ([]BasicSubordinateInfo, error)
	GetByStatusAndEntityTypes(status Status, entityTypes []string) ([]BasicSubordinateInfo, error)
	GetByStatusAndAnyEntityType(status Status, entityTypes []string) ([]BasicSubordinateInfo, error)
	Load() error
}
