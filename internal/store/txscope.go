package store

import (
	"context"
	"database/sql"
	"fmt"
)

type TxScope struct {
	conn *sql.DB
}

func NewTxScope(conn *sql.DB) *TxScope {
	return &TxScope{conn: conn}
}

func (s *TxScope) RunInTx(ctx context.Context, fn func(tx *sql.Tx) error) error {
	tx, err := s.conn.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}
