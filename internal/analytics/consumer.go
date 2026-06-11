package analytics

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/venkataYedupati/url-shortener-analytics-platform/internal/events"
	"github.com/venkataYedupati/url-shortener-analytics-platform/internal/store"
)

type Consumer struct {
	reader *events.KafkaConsumer
	store  *store.Store
	log    *slog.Logger
}

func NewConsumer(reader *events.KafkaConsumer, store *store.Store, log *slog.Logger) *Consumer {
	return &Consumer{reader: reader, store: store, log: log}
}

func (c *Consumer) Run(ctx context.Context) error {
	for {
		msg, err := c.reader.Fetch(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return nil
			}
			c.log.Warn("failed to fetch kafka message", "error", err)
			time.Sleep(500 * time.Millisecond)
			continue
		}

		event, err := events.DecodeClick(msg)
		if err != nil {
			c.log.Warn("failed to decode click event", "error", err)
			if commitErr := c.reader.Commit(ctx, msg); commitErr != nil {
				c.log.Warn("failed to commit invalid message", "error", commitErr)
			}
			continue
		}

		if event.OccurredAt.IsZero() {
			event.OccurredAt = msg.Time.UTC()
		}
		if event.OccurredAt.IsZero() {
			event.OccurredAt = time.Now().UTC()
		}

		if err := c.store.RecordClick(ctx, event); err != nil {
			c.log.Error("failed to record click event", "link_code", event.LinkCode, "error", err)
			time.Sleep(250 * time.Millisecond)
			continue
		}
		if err := c.reader.Commit(ctx, msg); err != nil {
			c.log.Warn("failed to commit click event", "link_code", event.LinkCode, "error", err)
		}
	}
}
