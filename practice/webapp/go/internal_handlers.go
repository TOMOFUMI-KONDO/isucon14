package main

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
)

type RideWithDistance struct {
	Ride
	Distance int `db:"distance"`
}

type ChairWithTotalTime struct {
	Chair
	TotalTime float64 `db:"total_time"`
}

// このAPIをインスタンス内から一定間隔で叩かせることで、椅子とライドをマッチングさせる
func internalGetMatching(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var rides []RideWithDistance
	if err := db.SelectContext(
		ctx,
		&rides,
		`SELECT
			*,
			ABS(pickup_latitude - destination_latitude) + ABS(pickup_longitude - destination_longitude) AS distance
		FROM rides
		WHERE chair_id IS NULL
		ORDER BY
			distance DESC,
			created_at`); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Errorf("failed to get ride: %w", err))
		return
	}

	for _, ride := range rides {
		matched := &ChairWithTotalTime{}
		if err := db.GetContext(
			ctx,
			matched,
			`SELECT
				chairs.id,
				(
					ABS(chairs.latitude - ?) + ABS(chairs.longitude - ?) +
					ABS(? - ?) + ABS(? - ?)
				) / chair_models.speed
				AS total_time
			FROM chairs
			JOIN chair_models ON chairs.model = chair_models.name
			WHERE
				chairs.is_active = TRUE AND
				chairs.is_empty = TRUE
			ORDER BY total_time
			LIMIT 1`,
			ride.PickupLatitude, ride.PickupLongitude,
			ride.PickupLatitude, ride.PickupLongitude,
			ride.DestinationLatitude, ride.DestinationLongitude,
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
	}

	w.WriteHeader(http.StatusNoContent)
}

func getRegion(latitude int) string {
	if latitude < 100 {
		return "ChairTown"
	} else {
		return "KoshikakeCity"
	}
}
