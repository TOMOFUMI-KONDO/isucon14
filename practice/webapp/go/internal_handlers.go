package main

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"slices"
)

type RideWithDistance struct {
	Ride
	Distance int `db:"distance"`
}

type ChairWithSpeed struct {
	Chair
	Speed int `db:"speed"`
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
		writeError(w, http.StatusInternalServerError, fmt.Errorf("failed to get rides: %w", err))
		return
	}

	var chairs []ChairWithSpeed
	if err := db.SelectContext(
		ctx,
		&chairs,
		`SELECT chairs.*, chair_models.speed
		FROM chairs
		JOIN chair_models ON chair_models.name = chairs.model
		WHERE
			chairs.is_active = TRUE AND
			chairs.is_empty = TRUE`,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeError(w, http.StatusInternalServerError, fmt.Errorf("failed to chairs: %w", err))
		return
	}

	for _, ride := range rides {
		matchedID := ""
		matchedIdx := -1
		totalTime := 1 << 10
		for i, chair := range chairs {
			pickupDistance := abs(chair.Latitude-ride.PickupLatitude) + abs(chair.Longitude-ride.PickupLongitude)
			rideDistance := abs(ride.PickupLatitude-ride.DestinationLatitude) + abs(ride.PickupLongitude-ride.DestinationLongitude)
			totalTimeTmp := (pickupDistance + rideDistance) / chair.Speed
			if matchedID == "" || totalTimeTmp < totalTime {
				matchedID = chair.ID
				matchedIdx = i
				totalTime = totalTimeTmp
			}
		}

		if matchedIdx != -1 {
			chairs = slices.Delete(chairs, matchedIdx, matchedIdx+1)
		}

		if _, err := db.ExecContext(ctx, "UPDATE rides SET chair_id = ? WHERE id = ?", matchedID, ride.ID); err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Errorf("failed to set chair_id to the ride: %w", err))
			continue
		}

		if _, err := db.ExecContext(ctx, "UPDATE chairs SET is_empty = FALSE WHERE id = ?", matchedID); err != nil {
			writeError(w, http.StatusInternalServerError, fmt.Errorf("failed to set is_empty to the chair: %w", err))
			continue
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
