package catalog

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

//go:embed metrics-catalog.json
var embeddedCatalog []byte

// Metric maps expvar key to TSV column name.
type Metric struct {
	TSV    string `json:"tsv"`
	Expvar string `json:"expvar"`
}

// LogPattern maps grep pattern to TSV column.
type LogPattern struct {
	Column  string `json:"column"`
	Pattern string `json:"pattern"`
}

// Gate describes one QA gate.
type Gate struct {
	ID   string `json:"id"`
	Rule string `json:"rule"`
}

// Catalog holds metrics, gates, and profile mappings.
type Catalog struct {
	Version     int                 `json:"version"`
	Metrics     []Metric            `json:"metrics"`
	LogPatterns []LogPattern        `json:"log_patterns"`
	Gates       []Gate              `json:"gates"`
	Profiles    map[string][]string `json:"profiles"`
}

// Load reads the metrics catalog from path or the embedded default.
func Load(path string) (Catalog, error) {
	data := embeddedCatalog
	if path != "" {
		b, err := os.ReadFile(path)
		if err != nil {
			return Catalog{}, fmt.Errorf("read catalog %s: %w", path, err)
		}
		data = b
	}

	var c Catalog
	if err := json.Unmarshal(data, &c); err != nil {
		return Catalog{}, fmt.Errorf("parse catalog: %w", err)
	}
	return c, nil
}

// DefaultPath returns the repo-relative catalog path for dev overrides.
func DefaultPath() string {
	return filepath.Join("internal", "catalog", "metrics-catalog.json")
}

// GatesForProfile returns gate IDs for a profile name.
func (c Catalog) GatesForProfile(profile string) ([]string, bool) {
	ids, ok := c.Profiles[profile]
	return ids, ok
}
