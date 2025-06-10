package consumer

import (
    "context"
    "encoding/json"
    "github.com/segmentio/kafka-go"
)

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

func (c *Consumer[T]) Close() error {
    return c.reader.Close()
}
