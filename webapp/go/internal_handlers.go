package main

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	"github.com/jmoiron/sqlx"
)

// このAPIをインスタンス内から一定間隔で叩かせることで、椅子とライドをマッチングさせる
func internalGetMatching(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	tx, err := db.Beginx()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	defer tx.Rollback()

	// MEMO: 一旦最も待たせているリクエストに適当な空いている椅子マッチさせる実装とする。おそらくもっといい方法があるはず…
	rides := make([]*Ride, 0)
	if err := tx.SelectContext(ctx, &rides, `SELECT id FROM rides WHERE chair_id IS NULL`); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			if err := tx.Commit(); err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Errorf("failed to get rides: %w", err))
		return
	}

	matched := make([]string, 0, len(rides))
	if err := tx.SelectContext(
		ctx,
		&matched,
		`SELECT id FROM chairs WHERE is_active = TRUE AND available = TRUE LIMIT ?`,
		len(rides),
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			if err := tx.Commit(); err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Errorf("failed to get available chair: %w", err))
		return
	}

	assignedRides := make([]*Ride, 0, len(matched))
	for i, m := range matched {
		assignedRides = append(assignedRides, rides[i])
		assignedRides[i].ChairID = sql.NullString{String: m, Valid: true}
	}

	query, args, err := sqlx.In("UPDATE chairs SET available = FALSE WHERE id IN (?)", matched)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("failed to get available chair: %w", err))
		return
	}
	if _, err := tx.ExecContext(ctx, query, args...); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	for _, r := range assignedRides {
		if _, err := tx.ExecContext(ctx, "UPDATE rides SET chair_id = ? WHERE id = ?", r.ChairID, r.ID); err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
	}

	if err := tx.Commit(); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
