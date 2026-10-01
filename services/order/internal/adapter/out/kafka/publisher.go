package kafka

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/segmentio/kafka-go"

	ordersv1 "github.com/remodov/marketplace-system-go/contracts/orders/v1"
	"github.com/remodov/marketplace-system-go/services/order/internal/core/order/port/out"
)

type Publisher struct {
	writer *kafka.Writer
}

var _ out.ExternalEventPublisher = (*Publisher)(nil)

func NewPublisher(brokers []string, topic string) *Publisher {
	return &Publisher{writer: &kafka.Writer{
		Addr:                   kafka.TCP(brokers...),
		Topic:                  topic,
		Balancer:               &kafka.Hash{},
		RequiredAcks:           kafka.RequireAll,
		AllowAutoTopicCreation: true,
		BatchTimeout:           10 * time.Millisecond,
	}}
}

const topicCreationAttempts = 10

func (p *Publisher) Publish(ctx context.Context, m out.OutboxMessage) error {
	var err error
	for range topicCreationAttempts {
		err = p.writer.WriteMessages(ctx, message(m))
		if !errors.Is(err, kafka.UnknownTopicOrPartition) {
			return err
		}
		select {
		case <-time.After(300 * time.Millisecond):
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return err
}

func message(m out.OutboxMessage) kafka.Message {
	return kafka.Message{
		Key:   []byte(m.AggregateID.String()),
		Value: m.Payload,
		Headers: []kafka.Header{
			{Key: ordersv1.HeaderEventID, Value: []byte(m.ID.String())},
			{Key: ordersv1.HeaderEventType, Value: []byte(m.EventType)},
			{Key: ordersv1.HeaderEventVersion, Value: []byte(strconv.Itoa(m.EventVersion))},
			{Key: ordersv1.HeaderAggregateType, Value: []byte(m.AggregateType)},
			{Key: ordersv1.HeaderAggregateID, Value: []byte(m.AggregateID.String())},
			{Key: ordersv1.HeaderOccurredAt, Value: []byte(m.OccurredAt.UTC().Format(time.RFC3339Nano))},
		},
	}
}

func (p *Publisher) Close() error { return p.writer.Close() }
