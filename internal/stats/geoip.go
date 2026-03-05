package stats

import (
	"net"
	"sync"

	"github.com/oschwald/maxminddb-golang"
)

// GeoIPLookup provides country code lookup from IP addresses using MaxMind GeoLite2 database.
type GeoIPLookup struct {
	reader *maxminddb.Reader
	mu     sync.RWMutex
}

// geoIPRecord is the structure for reading country data from MaxMind DB.
type geoIPRecord struct {
	Country struct {
		ISOCode string `maxminddb:"iso_code"`
	} `maxminddb:"country"`
}

// NewGeoIPLookup creates a new GeoIP lookup instance from a MaxMind database file.
// The database file should be a GeoLite2-Country.mmdb or GeoIP2-Country.mmdb file.
func NewGeoIPLookup(dbPath string) (*GeoIPLookup, error) {
	reader, err := maxminddb.Open(dbPath)
	if err != nil {
		return nil, err
	}
	return &GeoIPLookup{reader: reader}, nil
}

// LookupCountry returns the ISO 3166-1 alpha-2 country code for the given IP address.
// Returns an empty string if the lookup fails or the IP is not found.
func (g *GeoIPLookup) LookupCountry(ipStr string) string {
	if g == nil || g.reader == nil {
		return ""
	}

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return ""
	}

	g.mu.RLock()
	defer g.mu.RUnlock()

	var record geoIPRecord
	if err := g.reader.Lookup(ip, &record); err != nil {
		return ""
	}

	return record.Country.ISOCode
}

// Close closes the MaxMind database reader.
func (g *GeoIPLookup) Close() error {
	if g == nil || g.reader == nil {
		return nil
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	return g.reader.Close()
}

// NoOpGeoIPLookup is a no-op implementation that always returns empty strings.
// Used when GeoIP is disabled.
type NoOpGeoIPLookup struct{}

// LookupCountry always returns an empty string.
func (NoOpGeoIPLookup) LookupCountry(string) string {
	return ""
}

// Close is a no-op.
func (NoOpGeoIPLookup) Close() error {
	return nil
}

// GeoIPProvider is the interface for GeoIP lookups.
type GeoIPProvider interface {
	LookupCountry(ip string) string
	Close() error
}
