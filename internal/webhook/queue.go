package webhook

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

const (
	defaultRedisPrefix = "owlmail:webhooks"
	redisConsumerGroup = "owlmail"
	redisClaimIdle     = 30 * time.Second
	redisReadBlock     = time.Second
)

type queueReceipt struct {
	id  string
	job deliveryJob
}

type deliveryQueue interface {
	Enqueue(context.Context, deliveryJob) error
	Claim(context.Context) (*queueReceipt, error)
	Ack(context.Context, *queueReceipt) error
	DeadLetter(context.Context, *queueReceipt, []Result) error
	Renew(context.Context, *queueReceipt) error
	Pending(context.Context) (int64, error)
	Close() error
}

type redisDeliveryQueue struct {
	client     *redis.Client
	stream     string
	deadLetter string
	consumer   string
}

type deadLetterRecord struct {
	Job      deliveryJob `json:"job"`
	FailedAt time.Time   `json:"failedAt"`
	Failures []failure   `json:"failures"`
}

type failure struct {
	Target     string `json:"target"`
	StatusCode int    `json:"statusCode,omitempty"`
	Attempts   int    `json:"attempts"`
	Error      string `json:"error"`
}

func newRedisDeliveryQueue(ctx context.Context, rawURL, prefix, consumer string) (*redisDeliveryQueue, error) {
	options, err := redis.ParseURL(rawURL)
	if err != nil {
		return nil, fmt.Errorf("parse webhook Redis URL: %w", err)
	}
	client := redis.NewClient(options)
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, fmt.Errorf("connect to webhook Redis: %w", err)
	}
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		prefix = defaultRedisPrefix
	}
	queue := &redisDeliveryQueue{
		client:     client,
		stream:     prefix + ":events",
		deadLetter: prefix + ":dead-letter",
		consumer:   consumer,
	}
	if err := client.XGroupCreateMkStream(ctx, queue.stream, redisConsumerGroup, "0").Err(); err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		_ = client.Close()
		return nil, fmt.Errorf("create webhook Redis consumer group: %w", err)
	}
	return queue, nil
}

func (queue *redisDeliveryQueue) Enqueue(ctx context.Context, job deliveryJob) error {
	encoded, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("encode webhook job: %w", err)
	}
	if err := queue.client.XAdd(ctx, &redis.XAddArgs{
		Stream: queue.stream,
		Values: map[string]interface{}{"job": encoded},
	}).Err(); err != nil {
		return fmt.Errorf("enqueue webhook job: %w", err)
	}
	return nil
}

func (queue *redisDeliveryQueue) Claim(ctx context.Context) (*queueReceipt, error) {
	claimed, _, err := queue.client.XAutoClaim(ctx, &redis.XAutoClaimArgs{
		Stream:   queue.stream,
		Group:    redisConsumerGroup,
		Consumer: queue.consumer,
		MinIdle:  redisClaimIdle,
		Start:    "0-0",
		Count:    1,
	}).Result()
	if err != nil && !errors.Is(err, redis.Nil) {
		return nil, fmt.Errorf("reclaim webhook job: %w", err)
	}
	if len(claimed) > 0 {
		return decodeRedisMessage(claimed[0])
	}

	streams, err := queue.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group:    redisConsumerGroup,
		Consumer: queue.consumer,
		Streams:  []string{queue.stream, ">"},
		Count:    1,
		Block:    redisReadBlock,
	}).Result()
	if errors.Is(err, redis.Nil) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("claim webhook job: %w", err)
	}
	if len(streams) == 0 || len(streams[0].Messages) == 0 {
		return nil, nil
	}
	return decodeRedisMessage(streams[0].Messages[0])
}

func decodeRedisMessage(message redis.XMessage) (*queueReceipt, error) {
	raw, exists := message.Values["job"]
	if !exists {
		return nil, fmt.Errorf("webhook Redis entry %s has no job field", message.ID)
	}
	var encoded []byte
	switch value := raw.(type) {
	case string:
		encoded = []byte(value)
	case []byte:
		encoded = value
	default:
		return nil, fmt.Errorf("webhook Redis entry %s has invalid job field", message.ID)
	}
	var job deliveryJob
	if err := json.Unmarshal(encoded, &job); err != nil {
		return nil, fmt.Errorf("decode webhook Redis entry %s: %w", message.ID, err)
	}
	return &queueReceipt{id: message.ID, job: job}, nil
}

func (queue *redisDeliveryQueue) Ack(ctx context.Context, receipt *queueReceipt) error {
	if receipt == nil {
		return nil
	}
	_, err := queue.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.XAck(ctx, queue.stream, redisConsumerGroup, receipt.id)
		pipe.XDel(ctx, queue.stream, receipt.id)
		return nil
	})
	if err != nil {
		return fmt.Errorf("acknowledge webhook job: %w", err)
	}
	return nil
}

// Renew resets the pending entry's idle time while its HTTP delivery is still
// active. JUSTID avoids inflating Redis' delivery-attempt counter on each
// heartbeat.
func (queue *redisDeliveryQueue) Renew(ctx context.Context, receipt *queueReceipt) error {
	if receipt == nil {
		return nil
	}
	ids, err := queue.client.XClaimJustID(ctx, &redis.XClaimArgs{
		Stream: queue.stream, Group: redisConsumerGroup, Consumer: queue.consumer,
		MinIdle: 0, Messages: []string{receipt.id},
	}).Result()
	if err != nil {
		return fmt.Errorf("renew webhook job lease: %w", err)
	}
	if len(ids) != 1 || ids[0] != receipt.id {
		return fmt.Errorf("renew webhook job lease: entry %s is no longer pending", receipt.id)
	}
	return nil
}

func (queue *redisDeliveryQueue) DeadLetter(ctx context.Context, receipt *queueReceipt, results []Result) error {
	if receipt == nil {
		return nil
	}
	record := deadLetterRecord{Job: receipt.job, FailedAt: time.Now().UTC()}
	for _, result := range results {
		if result.Err == nil {
			continue
		}
		record.Failures = append(record.Failures, failure{
			Target: result.Target, StatusCode: result.StatusCode,
			Attempts: result.Attempts, Error: result.Err.Error(),
		})
	}
	encoded, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("encode webhook dead letter: %w", err)
	}
	_, err = queue.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		pipe.XAdd(ctx, &redis.XAddArgs{Stream: queue.deadLetter, Values: map[string]interface{}{"record": encoded}})
		pipe.XAck(ctx, queue.stream, redisConsumerGroup, receipt.id)
		pipe.XDel(ctx, queue.stream, receipt.id)
		return nil
	})
	if err != nil {
		return fmt.Errorf("store webhook dead letter: %w", err)
	}
	return nil
}

func (queue *redisDeliveryQueue) Pending(ctx context.Context) (int64, error) {
	count, err := queue.client.XLen(ctx, queue.stream).Result()
	if err != nil {
		return 0, fmt.Errorf("count webhook jobs: %w", err)
	}
	return count, nil
}

func (queue *redisDeliveryQueue) Close() error {
	return queue.client.Close()
}
