package bootstrap_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	kafkago "github.com/segmentio/kafka-go"

	ordersv1 "github.com/remodov/marketplace-system-go/contracts/orders/v1"
	"github.com/remodov/marketplace-system-go/services/order/internal/adapter/out/kafka"
	"github.com/remodov/marketplace-system-go/services/order/internal/core/order/port/out"
)

func TestKafkaPublisher_deliversPayloadWithHeaders(t *testing.T) {
	broker := os.Getenv("KAFKA_BROKERS")
	if broker == "" {
		broker = "localhost:9094"
	}
	dialCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	conn, err := kafkago.DialContext(dialCtx, "tcp", broker)
	if err != nil {
		t.Skipf("Kafka на %s недоступна (%v): подними стенд командой docker compose -f infra/compose.yaml up -d", broker, err)
	}
	_ = conn.Close()

	topic := ordersv1.Topic + ".test-" + uuid.NewString()
	publisher := kafka.NewPublisher([]string{broker}, topic)
	t.Cleanup(func() { _ = publisher.Close() })
	message := out.OutboxMessage{
		ID: uuid.New(), AggregateType: "Order", AggregateID: uuid.New(), EventType: ordersv1.EventOrderCreated,
		EventVersion: 1, Payload: []byte(`{"orderId":"x"}`), OccurredAt: now,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := publisher.Publish(ctx, message); err != nil {
		t.Fatal(err)
	}

	reader := kafkago.NewReader(kafkago.ReaderConfig{Brokers: []string{broker}, Topic: topic, Partition: 0, StartOffset: kafkago.FirstOffset})
	t.Cleanup(func() { _ = reader.Close() })
	received, err := reader.ReadMessage(ctx)
	if err != nil {
		t.Fatal(err)
	}
	headers := map[string]string{}
	for _, h := range received.Headers {
		headers[h.Key] = string(h.Value)
	}
	if string(received.Key) != message.AggregateID.String() || string(received.Value) != `{"orderId":"x"}` {
		t.Fatalf("ключ или тело не совпали: %s %s", received.Key, received.Value)
	}
	if headers[ordersv1.HeaderEventID] != message.ID.String() || headers[ordersv1.HeaderEventType] != ordersv1.EventOrderCreated || headers[ordersv1.HeaderEventVersion] != "1" {
		t.Fatalf("заголовки события: %v", headers)
	}
}
