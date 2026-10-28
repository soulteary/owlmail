package relay

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// RelayRecoveryInspection is an offline inventory; no records are changed or delivered.
type RelayRecoveryInspection struct {
	Pending          int `json:"pending"`
	Completed        int `json:"completed"`
	MissingSources   int `json:"missing_sources"`
	DuplicateSources int `json:"duplicate_sources"`
}

// InspectRecovery validates persisted records with the normal loader's reader.
// It never creates directories, prunes expired jobs, acquires leases, or starts workers.
func InspectRecovery(mailDirectory string) (RelayRecoveryInspection, error) {
	result := RelayRecoveryInspection{}
	directory := filepath.Join(mailDirectory, ".owlmail-meta", "relay-jobs")
	if _, err := os.Stat(directory); os.IsNotExist(err) {
		return result, nil
	} else if err != nil {
		return result, err
	}
	jobs, err := readPersistedRelayJobs(directory)
	if err != nil {
		return result, err
	}
	retained := 0
	now := time.Now().UTC()
	for _, job := range jobs {
		if job.CompletedAt == nil || now.Sub(*job.CompletedAt) <= defaultRelayJobTTL {
			retained++
		}
	}
	if retained > defaultRelayJobLimit {
		return result, fmt.Errorf("relay recovery record limit exceeded")
	}
	seen := make(map[string]bool)
	for _, job := range jobs {
		if job.CompletedAt != nil {
			result.Completed++
			continue
		}
		result.Pending++
		if seen[job.EmailID] {
			result.DuplicateSources++
		}
		seen[job.EmailID] = true
		if filepath.Base(job.EmailID) != job.EmailID || strings.ContainsAny(job.EmailID, "/\\") || job.EmailID == "." || job.EmailID == ".." {
			result.MissingSources++
			continue
		}
		info, err := os.Lstat(filepath.Join(mailDirectory, job.EmailID+".eml"))
		if err != nil || !info.Mode().IsRegular() {
			result.MissingSources++
		}
	}
	return result, nil
}
