package inbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	ordersv1 "github.com/remodov/marketplace-system-go/contracts/orders/v1"
)

const (
	ChannelEmail  = "EMAIL"
	StatusPending = "PENDING"
)

var ErrNoRecipient = errors.New("в событии нет адресата")

type IncomingEvent struct {
	ID      uuid.UUID
	Type    string
	Payload []byte
}

type Notification struct {
	ID          uuid.UUID `json:"id"`
	EventID     uuid.UUID `json:"eventId"`
	EventType   string    `json:"eventType"`
	UserID      uuid.UUID `json:"userId"`
	Channel     string    `json:"channel"`
	TemplateKey string    `json:"templateKey"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"createdAt"`
}

type Clock interface {
	Now() time.Time
}

type Processor struct {
	pool  *pgxpool.Pool
	clock Clock
}

func NewProcessor(pool *pgxpool.Pool, clock Clock) *Processor {
	return &Processor{pool: pool, clock: clock}
}

func (p *Processor) Process(ctx context.Context, event IncomingEvent) (bool, error) {
	recipient, err := recipientOf(event)
	if err != nil {
		return false, err
	}
	processed := false
	err = pgx.BeginFunc(ctx, p.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx,
			`INSERT INTO processed_events (event_id, event_type, processed_at) VALUES ($1, $2, $3)
			 ON CONFLICT (event_id) DO NOTHING`, event.ID, event.Type, p.clock.Now())
		if err != nil {
			return fmt.Errorf("processed_events insert: %w", err)
		}
		if tag.RowsAffected() == 0 {
			return nil
		}
		_, err = tx.Exec(ctx,
			`INSERT INTO notifications (id, event_id, event_type, user_id, channel, template_key, status, payload, created_at)
			 VALUES ($1, $2, $3, $4, $5, $6, $7, $8::jsonb, $9)`,
			uuid.New(), event.ID, event.Type, recipient, ChannelEmail, templateOf(event.Type), StatusPending, string(event.Payload), p.clock.Now())
		if err != nil {
			return fmt.Errorf("notifications insert: %w", err)
		}
		processed = true
		return nil
	})
	return processed, err
}

func (p *Processor) ListByUser(ctx context.Context, userID uuid.UUID) ([]Notification, error) {
	rows, err := p.pool.Query(ctx,
		`SELECT id, event_id, event_type, user_id, channel, template_key, status, created_at
		 FROM notifications WHERE user_id = $1 ORDER BY created_at DESC LIMIT 100`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []Notification
	for rows.Next() {
		var n Notification
		if err := rows.Scan(&n.ID, &n.EventID, &n.EventType, &n.UserID, &n.Channel, &n.TemplateKey, &n.Status, &n.CreatedAt); err != nil {
			return nil, err
		}
		result = append(result, n)
	}
	return result, rows.Err()
}

func recipientOf(event IncomingEvent) (uuid.UUID, error) {
	var base ordersv1.OrderEventBase
	if err := json.Unmarshal(event.Payload, &base); err != nil {
		return uuid.Nil, fmt.Errorf("payload %s не по контракту: %w", event.Type, err)
	}
	recipient := base.CustomerID
	if event.Type == ordersv1.EventDisputeOpened {
		recipient = base.SellerID
	}
	if recipient == uuid.Nil {
		return uuid.Nil, ErrNoRecipient
	}
	return recipient, nil
}

func templateOf(eventType string) string {
	switch eventType {
	case ordersv1.EventOrderCreated:
		return "order-created"
	case ordersv1.EventOrderPaid:
		return "order-paid"
	case ordersv1.EventOrderShipped:
		return "order-shipped"
	case ordersv1.EventDisputeOpened:
		return "dispute-opened-seller"
	default:
		return "order-status-changed"
	}
}
