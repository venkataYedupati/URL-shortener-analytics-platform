package events

import (
	"context"
	"encoding/json"
	"time"

	"github.com/segmentio/kafka-go"

	"github.com/venkataYedupati/url-shortener-analytics-platform/internal/config"
	"github.com/venkataYedupati/url-shortener-analytics-platform/internal/model"
)

type Publisher interface {
	PublishClick(ctx context.Context, event model.ClickEvent) error
	Close() error
}

type KafkaPublisher struct {
	writer *kafka.Writer
}

func NewKafkaPublisher(cfg config.Config) *KafkaPublisher {
	return &KafkaPublisher{
		writer: &kafka.Writer{
			Addr:         kafka.TCP(cfg.KafkaBrokers...),
			Topic:        cfg.KafkaClickTopic,
			Balancer:     &kafka.LeastBytes{},
			RequiredAcks: kafka.RequireOne,
			BatchTimeout: 10 * time.Millisecond,
			Async:        true,
		},
	}
}

func (p *KafkaPublisher) PublishClick(ctx context.Context, event model.ClickEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	return p.writer.WriteMessages(ctx, kafka.Message{
		Key:   []byte(event.LinkCode),
		Value: payload,
		Time:  event.OccurredAt,
	})
}

func (p *KafkaPublisher) Close() error {
	return p.writer.Close()
}

type KafkaConsumer struct {
	reader *kafka.Reader
}

func NewKafkaConsumer(cfg config.Config) *KafkaConsumer {
	return &KafkaConsumer{
		reader: kafka.NewReader(kafka.ReaderConfig{
			Brokers:     cfg.KafkaBrokers,
			Topic:       cfg.KafkaClickTopic,
			GroupID:     cfg.KafkaConsumerGroup,
			MinBytes:    1,
			MaxBytes:    10e6,
			StartOffset: kafka.FirstOffset,
		}),
	}
}

func (c *KafkaConsumer) Fetch(ctx context.Context) (kafka.Message, error) {
	return c.reader.FetchMessage(ctx)
}

func (c *KafkaConsumer) Commit(ctx context.Context, msg kafka.Message) error {
	return c.reader.CommitMessages(ctx, msg)
}

func (c *KafkaConsumer) Close() error {
	return c.reader.Close()
}

func DecodeClick(msg kafka.Message) (model.ClickEvent, error) {
	var event model.ClickEvent
	err := json.Unmarshal(msg.Value, &event)
	return event, err
}
