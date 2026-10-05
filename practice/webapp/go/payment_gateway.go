package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
)

var erroredUpstream = errors.New("errored upstream")

type paymentGatewayPostPaymentRequest struct {
	Amount int `json:"amount"`
}

type paymentGatewayGetPaymentsResponseOne struct {
	Amount int    `json:"amount"`
	Status string `json:"status"`
}

var muPaymentGateway sync.RWMutex

func requestPaymentGatewayPostPayment(ctx context.Context, paymentGatewayURL string, token string, param *paymentGatewayPostPaymentRequest, retrieveRidesOrderByCreatedAtAsc func() ([]Ride, error)) error {
	b, err := json.Marshal(param)
	if err != nil {
		return fmt.Errorf("failed to marshal: %w", err)
	}

	// 失敗したらとりあえずリトライ
	// FIXME: 社内決済マイクロサービスのインフラに異常が発生していて、同時にたくさんリクエストすると変なことになる可能性あり
	key := uuid.NewString()
	retry := 0
	for {
		err := func() error {
			muPaymentGateway.RLock()
			//lint:ignore SA2001 意図的に空にしている。write lock の解除待ちだけを行う。
			muPaymentGateway.RUnlock()

			req, err := http.NewRequestWithContext(ctx, http.MethodPost, paymentGatewayURL+"/payments", bytes.NewBuffer(b))
			if err != nil {
				return fmt.Errorf("failed to create post req: %w", err)
			}
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Idempotency-Key", key)

			res, err := http.DefaultClient.Do(req)
			if err != nil {
				return fmt.Errorf("failed to post request: %w", err)
			}
			defer res.Body.Close()

			switch res.StatusCode {
			case http.StatusNoContent:
				return nil
			default:
				return fmt.Errorf("unexpected status: %d", res.StatusCode)
			}
		}()
		if err != nil {
			muPaymentGateway.Lock()
			time.Sleep(1000 * time.Millisecond)
			muPaymentGateway.Unlock()

			if retry < 5 {
				slog.Warn("Failed to request payment gateway, retrying...", retry, err)
				retry++
				continue
			} else {
				slog.Warn("Failed to request payment gateway", err)
				return err
			}
		}
		break
	}

	return nil
}
