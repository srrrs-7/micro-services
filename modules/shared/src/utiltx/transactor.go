// Package utiltx provides a generic Transactor for managing database transactions.
// The transaction boundary can be controlled at any layer; the only requirement is
// that repository code uses Tx(ctx) to detect and bind to the active transaction.
//
// # Handler-level (transport layer controls tx — service stays clean)
//
//	// DI (main.go):
//	connDB := database.NewDB(...)
//	tx := utiltx.NewTransactor(connDB)
//	h := route.NewHandler(svc, tx)
//
//	// Handler (route/*.go):
//	func (h *handler) CreateUser(w http.ResponseWriter, r *http.Request) {
//		...
//		err := h.tx.WithinTx(r.Context(), func(ctx context.Context) error {
//			return h.svc.Create(ctx, input)
//		})
//		...
//	}
//
// # Usecase-level (service layer owns tx — for richer orchestration)
//
//	type CreateUserUseCase struct {
//		repo UserRepository
//		tx   utiltx.Transactor
//	}
//
//	func (uc *CreateUserUseCase) Execute(ctx context.Context, name string) error {
//		return uc.tx.WithinTx(ctx, func(ctx context.Context) error {
//			return uc.repo.Save(ctx, &User{Name: name})
//		})
//	}
//
// # Repository side (always the same — transparent tx detection)
//
//	func (r *userRepo) Save(ctx context.Context, u *User) error {
//		q := r.q
//		if tx := utiltx.Tx(ctx); tx != nil {
//			q = db.New(tx)
//		}
//		return q.InsertUser(ctx, db.InsertUserParams{...})
//	}
//
// Transactor passes *sql.Tx through context using a private key, so repository
// implementations never import the concrete transaction type.
package utiltx

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type txKey struct{}

// Transactor controls database transaction lifecycle.
// Usecase layer calls WithinTx; infrastructure reads the active *sql.Tx
// from context via Tx().
//
// Implementations must embed *sql.Tx in context so that repository code
// can retrieve it without importing the concrete transactor type.
type Transactor interface {
	// WithinTx executes fn inside a database transaction.
	// The context passed to fn carries the active *sql.Tx (retrievable via Tx()).
	//   - fn returns nil → transaction is committed.
	//   - fn returns error → transaction is rolled back.
	WithinTx(ctx context.Context, fn func(ctx context.Context) error) error
}

type transactor struct {
	db *sql.DB
}

// NewTransactor creates a Transactor backed by the given *sql.DB.
func NewTransactor(db *sql.DB) Transactor {
	return &transactor{db: db}
}

func (t *transactor) WithinTx(ctx context.Context, fn func(ctx context.Context) error) error {
	tx, err := t.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %v", err)
	}

	txCtx := context.WithValue(ctx, txKey{}, tx)

	if err := fn(txCtx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return errors.Join(err, fmt.Errorf("rollback: %v", rbErr))
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit: %v", err)
	}
	return nil
}

// Tx extracts the active *sql.Tx from context. Returns nil if no transaction
// is active (i.e. the caller is outside a WithinTx boundary).
//
// Usage in repository implementations:
//
//	func (r *repo) Save(ctx context.Context, u *User) error {
//		q := r.q
//		if tx := utiltx.Tx(ctx); tx != nil {
//			q = db.New(tx)
//		}
//		return q.InsertUser(ctx, ...)
//	}
//
// This works because *sql.Tx satisfies sqlc's DBTX interface.
func Tx(ctx context.Context) *sql.Tx {
	tx, _ := ctx.Value(txKey{}).(*sql.Tx)
	return tx
}
