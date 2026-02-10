package engine

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Stats holds aggregate information about the workspace.
type Stats struct {
	TotalSites    int         `json:"total_sites"`
	TotalFiles    int         `json:"total_files"`
	TotalSize     int64       `json:"total_size_bytes"`
	TotalSizeHuman string    `json:"total_size"`
	OldestSync    time.Time   `json:"oldest_sync,omitempty"`
	NewestSync    time.Time   `json:"newest_sync,omitempty"`
	SiteStats     []SiteStats `json:"sites"`
}

// SiteStats holds per-site statistics.
type SiteStats struct {
	Domain    string    `json:"domain"`
	URL       string    `json:"url"`
	FileCount int       `json:"file_count"`
	Size      int64     `json:"size_bytes"`
	SizeHuman string    `json:"size"`
	LastSync  time.Time `json:"last_sync,omitempty"`
	Age       string    `json:"age,omitempty"` // human-readable time since last sync
}

// Stats returns aggregate statistics about the workspace.
func (e *Engine) Stats(ctx context.Context) (*Stats, error) {
	s := &Stats{}

	for domain, siteCfg := range e.Config.Sites {
		ss := SiteStats{
			Domain:   domain,
			URL:      siteCfg.URL,
			LastSync: siteCfg.LastSync,
		}

		// Walk site directory for file count and size
		siteDir := e.Store.SiteDir(domain)
		err := filepath.Walk(siteDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return nil // skip errors
			}
			if info.IsDir() {
				if info.Name() == "_meta" {
					return filepath.SkipDir
				}
				return nil
			}
			ss.FileCount++
			ss.Size += info.Size()
			return nil
		})
		if err != nil && !os.IsNotExist(err) {
			return nil, fmt.Errorf("walking %s: %w", domain, err)
		}

		ss.SizeHuman = humanSize(ss.Size)
		if !ss.LastSync.IsZero() {
			ss.Age = humanAge(time.Since(ss.LastSync))
		}

		s.TotalSites++
		s.TotalFiles += ss.FileCount
		s.TotalSize += ss.Size

		if s.OldestSync.IsZero() || (!siteCfg.LastSync.IsZero() && siteCfg.LastSync.Before(s.OldestSync)) {
			s.OldestSync = siteCfg.LastSync
		}
		if siteCfg.LastSync.After(s.NewestSync) {
			s.NewestSync = siteCfg.LastSync
		}

		s.SiteStats = append(s.SiteStats, ss)
	}

	s.TotalSizeHuman = humanSize(s.TotalSize)
	return s, nil
}

func humanSize(b int64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(b)/float64(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(b)/float64(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(b)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", b)
	}
}

// Stale returns sites that haven't been synced within the given duration.
func (e *Engine) Stale(ctx context.Context, threshold time.Duration) ([]SiteStats, error) {
	stats, err := e.Stats(ctx)
	if err != nil {
		return nil, err
	}

	cutoff := time.Now().Add(-threshold)
	var stale []SiteStats
	for _, s := range stats.SiteStats {
		if s.LastSync.IsZero() || s.LastSync.Before(cutoff) {
			stale = append(stale, s)
		}
	}
	return stale, nil
}

func humanAge(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		days := int(d.Hours() / 24)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}
}
