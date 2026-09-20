package stats

import (
	"context"
	"time"
)

// ReplicationProvider reports current Litestream replication status.
// Implementations must be safe for concurrent use.
type ReplicationProvider func(ctx context.Context) ReplicationInfo

// Service assembles the full stats page payload.
type Service struct {
	store       *Store
	baseURL     string
	startTime   time.Time
	replication ReplicationProvider
}

func NewService(store *Store, baseURL string, startTime time.Time, replication ReplicationProvider) *Service {
	if baseURL != "" && baseURL[len(baseURL)-1] != '/' {
		baseURL += "/"
	}
	return &Service{
		store:       store,
		baseURL:     baseURL,
		startTime:   startTime,
		replication: replication,
	}
}

// DisabledReplication is a ReplicationProvider used when replication is off.
func DisabledReplication(context.Context) ReplicationInfo {
	return ReplicationInfo{}
}

// Page computes all stats for one render of /_/stats.
func (s *Service) Page(ctx context.Context) (*PageData, error) {
	site, err := s.store.SiteStats(ctx, s.baseURL)
	if err != nil {
		return nil, err
	}
	FillDerived(site)

	rt := CollectRuntime(s.startTime)
	rt.DBSizeBytes, err = s.store.DBSizeBytes(ctx)
	if err != nil {
		return nil, err
	}

	repl := DisabledReplication(ctx)
	if s.replication != nil {
		repl = s.replication(ctx)
	}

	return &PageData{
		Site:        *site,
		Runtime:     rt,
		Replication: repl,
		GeneratedAt: time.Now().UTC(),
	}, nil
}

// FillDerived computes the average and human-time fields from raw counters.
func FillDerived(s *SiteStats) {
	if s.TotalLinks > 0 {
		s.AvgCharsSavedPerLink = float64(s.TotalCharsSaved) / float64(s.TotalLinks)
	}
	s.HumanSecondsSaved = HumanSecondsSaved(s.TotalCharsSaved)
}

// HumanSecondsSaved converts characters saved into seconds of human typing
// at typingWPM words per minute and charsPerWord characters per word.
func HumanSecondsSaved(chars int64) float64 {
	return float64(chars) * 60 / float64(humanCharRate)
}
