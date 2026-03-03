package storage

import (
	"github.com/pkg/errors"
	"gorm.io/gorm"

	"github.com/go-oidfed/lighthouse/storage/model"
)

// SubordinateEventsStorage implements the SubordinateEventStore interface using GORM.
type SubordinateEventsStorage struct {
	db *gorm.DB
}

// NewSubordinateEventsStorage creates a new SubordinateEventsStorage.
func NewSubordinateEventsStorage(db *gorm.DB) *SubordinateEventsStorage {
	return &SubordinateEventsStorage{db: db}
}

// Add creates a new event record.
func (s *SubordinateEventsStorage) Add(event model.SubordinateEvent) error {
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
	// Apply defaults
	limit := opts.Limit
	if limit <= 0 {
		limit = 50
	}
	if limit > 100 {
		limit = 100
	}

	offset := opts.Offset
	if offset < 0 {
		offset = 0
	}

	// Build query
	query := s.db.Model(&model.SubordinateEvent{}).Where("subordinate_id = ?", subordinateID)

	// Apply filters
	if opts.EventType != nil && *opts.EventType != "" {
		query = query.Where("type = ?", *opts.EventType)
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

// DeleteBySubordinateID removes all events for a subordinate.
func (s *SubordinateEventsStorage) DeleteBySubordinateID(subordinateID uint) error {
	if err := s.db.Where("subordinate_id = ?", subordinateID).
		Delete(&model.SubordinateEvent{}).Error; err != nil {
		return errors.Wrap(err, "subordinate_events: failed to delete events")
	}
	return nil
}
