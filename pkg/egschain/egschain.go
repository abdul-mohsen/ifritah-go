// Package egschain reserves and commits the invoice counter (ICV) and the previous invoice hash
// (PIH) of one EGS. The EGS is the branch's branch_zatca_config row, so the chain is keyed by
// branch. Reserve runs in a short transaction that commits before any XML is built or signed.
// Commit runs once the hash of the signed document is known. Only the owner of a reservation can
// commit or release it.
package egschain

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"

	"github.com/go-sql-driver/mysql"

	db "ifritah/web-service-gin/pkg/db/gen"
)

var (
	// ErrNotFound means the branch has no EGS row.
	ErrNotFound = errors.New("egschain: the branch has no EGS")
	// ErrBusy means the branch row is locked by another transaction, or another document holds
	// the chain. That includes an expired reservation for another document: its ICV is never
	// given to a different document. The caller retries later.
	ErrBusy = errors.New("egschain: the EGS chain is busy")
	// ErrNotOwner means the reservation was committed, released, or taken over by another owner.
	ErrNotOwner = errors.New("egschain: the reservation is not owned by the caller")

	errIncomplete = errors.New("egschain: the reservation columns are incomplete")
)

const (
	maxKeyLen  = 255
	maxHashLen = 255
	tokenBytes = 16
)

// Reservation is one document's claim on the next ICV of an EGS. Pass it back to Commit or
// Release.
type Reservation struct {
	BranchID    uint32
	DocumentKey string
	Token       string
	ICV         uint64
	// PreviousHash is the committed hash of the previous signed document. It is empty for the
	// first document of the chain.
	PreviousHash   string
	LeaseExpiresAt time.Time
}

// Reserve takes the next ICV of the branch's EGS for documentKey, inside tx.
//
//   - A key that holds a live reservation gets that reservation back, and nothing is incremented.
//   - A key whose lease has expired takes the reservation over with a new token and the same ICV.
//     The previous owner's token stops working.
//   - Any other key gets ErrBusy, whether the other reservation is live or expired.
//   - If another transaction holds the branch row, Reserve returns ErrBusy at once. It never
//     waits for the row, so a worker does not hold a pool connection while it queues.
func Reserve(ctx context.Context, tx *sql.Tx, branchID uint32, documentKey string, now time.Time, lease time.Duration) (Reservation, error) {
	if branchID == 0 || documentKey == "" || len(documentKey) > maxKeyLen || lease <= 0 {
		return Reservation{}, errors.New("egschain: a branch, a document key and a lease are required")
	}
	q := db.New(tx)
	chain, err := q.LockEGSChain(ctx, branchID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return Reservation{}, ErrNotFound
	case isRowLocked(err):
		return Reservation{}, ErrBusy
	case err != nil:
		return Reservation{}, err
	}
	if chain.IcvReservationKey != nil {
		if *chain.IcvReservationKey != documentKey {
			return Reservation{}, ErrBusy
		}
		if chain.ReservationLeaseExpiresAt != nil && chain.ReservationLeaseExpiresAt.After(now) {
			return heldReservation(chain)
		}
	}
	token, err := newToken()
	if err != nil {
		return Reservation{}, err
	}
	res := Reservation{
		BranchID:       branchID,
		DocumentKey:    documentKey,
		Token:          token,
		ICV:            chain.LastIcv + 1,
		PreviousHash:   previousHash(chain.PreviousInvoiceHash),
		LeaseExpiresAt: now.Add(lease),
	}
	if _, err := q.StartEGSChainReservation(ctx, db.StartEGSChainReservationParams{
		ReservationKey:   &documentKey,
		ReservationToken: &res.Token,
		ReservedIcv:      &res.ICV,
		LeaseExpiresAt:   &res.LeaseExpiresAt,
		BranchID:         branchID,
	}); err != nil {
		return Reservation{}, err
	}
	return res, nil
}

// Commit records signedHash as the chain's previous hash, advances the committed ICV to the
// reserved one, and clears the reservation. ErrNotOwner means the reservation no longer belongs to
// the caller, and the chain is unchanged.
func Commit(ctx context.Context, tx *sql.Tx, r Reservation, signedHash string) error {
	if signedHash == "" || len(signedHash) > maxHashLen {
		return errors.New("egschain: a signed document hash of 1 to 255 bytes is required")
	}
	res, err := db.New(tx).CommitEGSChainReservation(ctx, db.CommitEGSChainReservationParams{
		PreviousInvoiceHash: &signedHash,
		BranchID:            r.BranchID,
		ReservationToken:    &r.Token,
		ReservationKey:      &r.DocumentKey,
		ReservedIcv:         &r.ICV,
	})
	return ownedRows(res, err)
}

// Release clears the reservation without moving the chain. The committed ICV is kept, so the next
// document gets the same ICV.
func Release(ctx context.Context, tx *sql.Tx, r Reservation) error {
	res, err := db.New(tx).ReleaseEGSChainReservation(ctx, db.ReleaseEGSChainReservationParams{
		BranchID:         r.BranchID,
		ReservationToken: &r.Token,
		ReservationKey:   &r.DocumentKey,
	})
	return ownedRows(res, err)
}

// heldReservation returns the live reservation that the locked row holds.
func heldReservation(chain db.LockEGSChainRow) (Reservation, error) {
	if chain.IcvReservationKey == nil || chain.IcvReservationToken == nil || chain.ReservedIcv == nil || chain.ReservationLeaseExpiresAt == nil {
		return Reservation{}, errIncomplete
	}
	return Reservation{
		BranchID:       chain.BranchID,
		DocumentKey:    *chain.IcvReservationKey,
		Token:          *chain.IcvReservationToken,
		ICV:            *chain.ReservedIcv,
		PreviousHash:   previousHash(chain.PreviousInvoiceHash),
		LeaseExpiresAt: *chain.ReservationLeaseExpiresAt,
	}, nil
}

// ownedRows turns a write that must change exactly one row into ErrNotOwner when it changes none.
func ownedRows(res sql.Result, err error) error {
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrNotOwner
	}
	return nil
}

func previousHash(hash *string) string {
	if hash == nil {
		return ""
	}
	return *hash
}

func newToken() (string, error) {
	b := make([]byte, tokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// mysqlErrLockNoWait is the MySQL error for a NOWAIT lock that another transaction holds.
const mysqlErrLockNoWait = 3572

// isRowLocked reports whether err means that another transaction holds the row.
func isRowLocked(err error) bool {
	var mysqlErr *mysql.MySQLError
	return errors.As(err, &mysqlErr) && mysqlErr.Number == mysqlErrLockNoWait
}
