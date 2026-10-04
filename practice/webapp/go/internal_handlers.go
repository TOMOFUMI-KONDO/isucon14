package main

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
)

type ChairWithDistance struct {
	Chair
	Distance int `db:"distance"`
}

// このAPIをインスタンス内から一定間隔で叩かせることで、椅子とライドをマッチングさせる
func internalGetMatching(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// MEMO: 一旦最も待たせているリクエストに適当な空いている椅子マッチさせる実装とする。おそらくもっといい方法があるはず…
	var ride Ride
	if err := db.GetContext(ctx, &ride, `SELECT * FROM rides WHERE chair_id IS NULL ORDER BY created_at LIMIT 1`); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Errorf("failed to get ride: %w", err))
		return
	}

	matched := &ChairWithDistance{}
	if err := db.GetContext(
		ctx,
		matched,
		`SELECT
			id,
			ABS(chairs.latitude - ?) + ABS(chairs.longitude - ?) AS distance 
		FROM chairs
		WHERE
			is_active = TRUE AND
			is_empty = TRUE
		ORDER BY distance
		LIMIT 1`,
		ride.PickupLatitude, ride.PickupLongitude,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Errorf("failed to get matched chair: %w", err))
	}

	if _, err := db.ExecContext(ctx, "UPDATE rides SET chair_id = ? WHERE id = ?", matched.ID, ride.ID); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("failed to set chair_id to the ride: %w", err))
		return
	}

	if _, err := db.ExecContext(ctx, "UPDATE chairs SET is_empty = FALSE WHERE id = ?", matched.ID); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Errorf("failed to set is_empty to the chair: %w", err))
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
