package kafkautil

import (
    "context"
    "encoding/json"
    "github.com/segmentio/kafka-go"
    "time"
    "net"
    "fmt"
)
const 	DialTimeout      = 4 * time.Second  

type Consumer[T any] struct {
    reader *kafka.Reader
}

func NewConsumer[T any](cfg Config) *Consumer[T] {
    r := kafka.NewReader(kafka.ReaderConfig{
        Brokers: cfg.Brokers,
        GroupID: cfg.GroupID,
        Topic:   cfg.Topic,
    })
    return &Consumer[T]{reader: r}
}

func (c *Consumer[T]) Read(ctx context.Context) (T, error) {
    var zero T

    msg, err := c.reader.FetchMessage(ctx)
    if err != nil {
        return zero, err
    }

    var payload T
    if err := json.Unmarshal(msg.Value, &payload); err != nil {
        return zero, err
    }

    if err := c.reader.CommitMessages(ctx, msg); err != nil {
        return zero, err
    }

    return payload, nil
}

func (c *Consumer[T]) ReadWithMeta(ctx context.Context) (T, kafka.Message, error) {
    var zero T
    msg, err := c.reader.FetchMessage(ctx)
    if err != nil {
        return zero, msg, err
    }
    var out T
    if err := json.Unmarshal(msg.Value, &out); err != nil {
        // surface the decode error but still return raw msg so caller can commit/DLQ
        return zero, msg, fmt.Errorf("unmarshal: %w", err)
    }
    return out, msg, nil
}

// Commit the offset of a specific message
func (c *Consumer[T]) Commit(ctx context.Context, msg kafka.Message) error {
    return c.reader.CommitMessages(ctx, msg)
}

func (c *Consumer[T]) Close() error {
    return c.reader.Close()
}

// Try TCP dial to any broker to fail fast on obvious misconfig.
func CanReachAnyBroker(brokers []string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for _, addr := range brokers {

		dl := time.Until(deadline)
		if dl <= 0 {
			return false
		}
		conn, err := net.DialTimeout("tcp", addr, min(dl, DialTimeout))
		if err == nil {
			_ = conn.Close()
			return true
		}
	}
	return false
}
