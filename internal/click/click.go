package click

import (
	"context"
	"database/sql"
	"sync"
	"time"
)

type Info struct {
	LinkID   int64
	IPHash   string
	Referrer string
	Country  string
}

type Service struct {
	db    *sql.DB
	queue chan Info
	done  chan struct{}
	wg    sync.WaitGroup
}

func NewService(db *sql.DB, bufferSize int) *Service {
	s := &Service{
		db:    db,
		queue: make(chan Info, bufferSize),
		done:  make(chan struct{}),
	}

	s.wg.Add(1)
	go s.worker()

	return s
}

func (s *Service) Record(ctx context.Context, info Info) error {
	select {
	case s.queue <- info:
		return nil
	default:
		return nil
	}
}

func (s *Service) Count(ctx context.Context, linkID int64) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM clicks WHERE link_id = ?`, linkID).Scan(&count)
	return count, err
}

func (s *Service) Close() {
	close(s.queue)
	s.wg.Wait()
}

func (s *Service) worker() {
	defer s.wg.Done()

	for info := range s.queue {
		s.insertClick(info)
	}
}

func (s *Service) insertClick(info Info) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, _ = s.db.ExecContext(ctx,
		`INSERT INTO clicks (link_id, ip_hash, referrer, country) VALUES (?, ?, ?, ?)`,
		info.LinkID, info.IPHash, info.Referrer, info.Country)
}
