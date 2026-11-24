package main

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
)

type Kind int

const (
	KindBatch Kind = iota
	KindFatal
)

func classifyErr(err error) Kind {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return KindFatal
	}

	var pgErr *pgconn.PgError

	if errors.As(err, &pgErr) {
		switch pgErr.Code[:2] {
		case "22", "23", "40": // data, integrity, serialization/deadlock
			return KindBatch
		}
		if pgErr.Code == "55PO3" { // lock not available 
			return KindBatch
		}
		return KindFatal // 08, 28, 53, 58, XX, 25, 3D, 3F, 42, unknown
	}
	return KindFatal // network, io, scan errors: unknown means stop
}

func ambiguousErr(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr){
		return pgErr.Code[:2] == "08"
	}
	return !errors.As(err, &pgErr)
}
