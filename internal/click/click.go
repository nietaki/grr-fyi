package click

import (
	"context"
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
	store *Store
	queue chan Info
	done  chan struct{}
	wg    sync.WaitGroup
}

func NewService(store *Store, bufferSize int) *Service {
	s := &Service{
		store: store,
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
	return s.store.Count(ctx, linkID)
}

func (s *Service) CountDistinctIPs(ctx context.Context, linkID int64) (int64, error) {
	return s.store.CountDistinctIPs(ctx, linkID)
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

	_ = s.store.Insert(ctx, info)
}
