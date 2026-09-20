package stats

import "time"

// SiteStats holds aggregate counters derived from the links and clicks tables.
type SiteStats struct {
	TotalLinks   int64
	ActiveLinks  int64
	RevokedLinks int64
	TotalClicks  int64

	// TotalCharsSaved is Σ over links of (len(target_url) - len(shortened_url)) × clicks,
	// where shortened_url = baseURL + slug. May be negative if targets are shorter
	// than their shortened form.
	TotalCharsSaved int64
	// AvgCharsSavedPerLink is TotalCharsSaved / TotalLinks (0 when there are no links).
	AvgCharsSavedPerLink float64
	// HumanSecondsSaved assumes 80 words-per-minute typing at 5 characters per word.
	HumanSecondsSaved float64
}

// RuntimeStats holds process-level metrics.
type RuntimeStats struct {
	Uptime         time.Duration
	RSSBytes       int64 // 0 when unavailable (e.g. non-Linux)
	HeapAllocBytes int64
	HeapInuseBytes int64
	SysBytes       int64
	Goroutines     int64
	DBSizeBytes    int64
}

// ReplicationInfo reports the Litestream replication state.
type ReplicationInfo struct {
	Enabled    bool
	InSync     bool
	LocalTXID  int64
	RemoteTXID int64
	Err        string // non-empty if status could not be determined
}

// PageData is everything the /_/stats template needs.
type PageData struct {
	Site        SiteStats
	Runtime     RuntimeStats
	Replication ReplicationInfo
	GeneratedAt time.Time
}
