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

type ChairWithDistance struct {
	Chair
	Distance int `db:"distance"`
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
		order := "distance, chair_models.speed DESC"
		if getRegion(ride.PickupLatitude) != getRegion(ride.DestinationLatitude) {
			order = "chair_models.speed DESC, distance"
		}

		matched := &ChairWithDistance{}
		if err := db.GetContext(
			ctx,
			matched,
			`SELECT
			chairs.id,
			ABS(chairs.latitude - ?) + ABS(chairs.longitude - ?) AS distance 
			FROM chairs
			JOIN chair_models ON chairs.model = chair_models.name
			WHERE
				chairs.is_active = TRUE AND
				chairs.is_empty = TRUE
			ORDER BY ?
			LIMIT 1`,
			ride.PickupLatitude, ride.PickupLongitude, order,
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
