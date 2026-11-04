//go:build mobileassets

package mobilecore

import (
	"context"

	"aipanyi/backend/events"
)

func emitMobile(event string, payload map[string]any) {
	events.Emit(context.Background(), event, payload)
}
