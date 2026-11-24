package storage

import (
	"strings"

	"github.com/pkg/errors"
	"gorm.io/gorm"

	"github.com/go-oidfed/lighthouse/storage/model"
)

const (
	// defaultEventsLimit is the page size when no limit is given.
	defaultEventsLimit = 50
	// maxSubordinateEventsLimit caps GetBySubordinateID.
	maxSubordinateEventsLimit = 100
	// maxEventsQueryLimit caps Query.
	maxEventsQueryLimit = 10000
)

// SubordinateEventsStorage implements the SubordinateEventStore interface using GORM.
type SubordinateEventsStorage struct {
	db *gorm.DB
}

// NewSubordinateEventsStorage creates a new SubordinateEventsStorage.
func NewSubordinateEventsStorage(db *gorm.DB) *SubordinateEventsStorage {
	return &SubordinateEventsStorage{db: db}
}

// Add creates a new event record. When EntityID is empty and SubordinateID is
// set, EntityID is filled from the subordinate row, including a removed
// (soft-deleted) one; an unknown subordinate leaves it empty.
func (s *SubordinateEventsStorage) Add(event model.SubordinateEvent) error {
	if event.EntityID == "" && event.SubordinateID != 0 {
		var entityIDs []string
		if err := s.db.Unscoped().Model(&model.ExtendedSubordinateInfo{}).
			Where("id = ?", event.SubordinateID).
			Limit(1).
			Pluck("entity_id", &entityIDs).Error; err != nil {
			return errors.Wrap(err, "subordinate_events: failed to look up subordinate entity ID")
		}
		if len(entityIDs) > 0 {
			event.EntityID = entityIDs[0]
		}
	}
	if err := s.db.Create(&event).Error; err != nil {
		return errors.Wrap(err, "subordinate_events: failed to create event")
	}
	return nil
}

// GetBySubordinateID returns events for a subordinate with optional filtering and pagination.
// Returns the events, total count (for pagination), and any error.
func (s *SubordinateEventsStorage) GetBySubordinateID(
	subordinateID uint, opts model.EventQueryOpts,
) ([]model.SubordinateEvent, int64, error) {
	query := s.db.Model(&model.SubordinateEvent{}).Where("subordinate_id = ?", subordinateID)
	return findEvents(query, opts, maxSubordinateEventsLimit)
}

// Query returns events of all entities matching opts, newest first.
// Returns the events, total count (for pagination), and any error.
func (s *SubordinateEventsStorage) Query(opts model.EventQueryOpts) ([]model.SubordinateEvent, int64, error) {
	return findEvents(s.db.Model(&model.SubordinateEvent{}), opts, maxEventsQueryLimit)
}

// likeEscaper escapes the LIKE wildcards and the escape character itself, '!'
// (portable across SQLite, PostgreSQL and MySQL, unlike a backslash).
var likeEscaper = strings.NewReplacer("!", "!!", "%", "!%", "_", "!_")

// RemovedEntityIDs returns up to limit distinct entity IDs, sorted, that
// contain search (case-insensitively) and have events but no current
// (non-deleted) subordinate.
func (s *SubordinateEventsStorage) RemovedEntityIDs(search string, limit int) ([]string, error) {
	current := s.db.Model(&model.ExtendedSubordinateInfo{}).Select("entity_id")
	pattern := "%" + likeEscaper.Replace(strings.ToLower(search)) + "%"
	var ids []string
	if err := s.db.Model(&model.SubordinateEvent{}).
		Distinct("entity_id").
		Where("entity_id <> ''").
		Where("LOWER(entity_id) LIKE ? ESCAPE '!'", pattern).
		Where("entity_id NOT IN (?)", current).
		Order("entity_id").
		Limit(limit).
		Pluck("entity_id", &ids).Error; err != nil {
		return nil, errors.Wrap(err, "subordinate_events: failed to search removed entity IDs")
	}
	return ids, nil
}

// findEvents applies the filters in opts to query and returns one page of
// events, newest first, with the total count before pagination.
func findEvents(
	query *gorm.DB, opts model.EventQueryOpts, maxLimit int,
) ([]model.SubordinateEvent, int64, error) {
	// Apply defaults
	limit := opts.Limit
	if limit <= 0 {
		limit = defaultEventsLimit
	}
	limit = min(limit, maxLimit)

	offset := max(opts.Offset, 0)

	// Apply filters
	if opts.EventType != nil && *opts.EventType != "" {
		query = query.Where("type = ?", *opts.EventType)
	}
	if len(opts.EventTypes) > 0 {
		query = query.Where("type IN ?", opts.EventTypes)
	}
	if opts.EntityID != nil {
		query = query.Where("entity_id = ?", *opts.EntityID)
	}
	if opts.Actor != nil {
		query = query.Where("actor = ?", *opts.Actor)
	}
	if opts.FromTime != nil {
		query = query.Where("timestamp >= ?", *opts.FromTime)
	}
	if opts.ToTime != nil {
		query = query.Where("timestamp <= ?", *opts.ToTime)
	}

	// Get total count before pagination
	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, errors.Wrap(err, "subordinate_events: failed to count events")
	}

	// Apply pagination and ordering (newest first)
	var events []model.SubordinateEvent
	if err := query.Order("timestamp DESC, id DESC").
		Limit(limit).
		Offset(offset).
		Find(&events).Error; err != nil {
		return nil, 0, errors.Wrap(err, "subordinate_events: failed to get events")
	}

	return events, total, nil
}
