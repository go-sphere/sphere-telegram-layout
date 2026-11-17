package dao

import (
	"context"
	"database/sql"
	"errors"

	"github.com/go-sphere/sphere-telegram-layout/internal/pkg/database/ent"
	"github.com/go-sphere/sphere/log"
)

type txOptions struct {
	Isolation    sql.IsolationLevel
	ReadOnly     bool
	CommitHook   ent.CommitHook
	RollbackHook ent.RollbackHook
}

func newTxOptions(opts ...Option) *txOptions {
	defaults := &txOptions{
		Isolation:    sql.LevelDefault,
		ReadOnly:     false,
		CommitHook:   nil,
		RollbackHook: nil,
	}
	for _, opt := range opts {
		opt(defaults)
	}
	return defaults
}

type Option func(*txOptions)

func WithTxIsolation(level sql.IsolationLevel) Option {
	return func(opts *txOptions) {
		opts.Isolation = level
	}
}

func WithTxReadOnly(readOnly bool) Option {
	return func(opts *txOptions) {
		opts.ReadOnly = readOnly
	}
}

func WithTxCommitHook(hook ent.CommitHook) Option {
	return func(opts *txOptions) {
		opts.CommitHook = hook
	}
}

func WithTxRollbackHook(hook ent.RollbackHook) Option {
	return func(opts *txOptions) {
		opts.RollbackHook = hook
	}
}

func WithTx[T any](ctx context.Context, db *ent.Client, exe func(ctx context.Context, tx *ent.Client) (*T, error), opts ...Option) (*T, error) {
	txOpts := newTxOptions(opts...)
	tx, err := db.BeginTx(ctx, &sql.TxOptions{
		Isolation: txOpts.Isolation,
		ReadOnly:  txOpts.ReadOnly,
	})
	if err != nil {
		return nil, err
	}
	if txOpts.CommitHook != nil {
		tx.OnCommit(txOpts.CommitHook)
	}
	if txOpts.RollbackHook != nil {
		tx.OnRollback(txOpts.RollbackHook)
	}
	defer func() {
		if reason := recover(); reason != nil {
			log.Warn("WithTx panic", log.Any("error", reason))
			_ = tx.Rollback()
			panic(reason)
		}
	}()
	result, err := exe(ctx, tx.Client())
	if err != nil {
		if rErr := tx.Rollback(); rErr != nil && !errors.Is(rErr, sql.ErrTxDone) {
			return nil, errors.Join(err, rErr)
		}
		return nil, err
	}
	if cErr := tx.Commit(); cErr != nil {
		return nil, cErr
	}
	return result, nil
}

func WithTxEx(ctx context.Context, db *ent.Client, exe func(ctx context.Context, tx *ent.Client) error, opts ...Option) error {
	_, err := WithTx(ctx, db, func(ctx context.Context, tx *ent.Client) (*struct{}, error) {
		return nil, exe(ctx, tx)
	}, opts...)
	return err
}
