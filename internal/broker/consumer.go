package broker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/Azure/go-amqp"
	"github.com/Yisusocanto/pixel-reviews-moderator/internal/application"
	rmq "github.com/rabbitmq/rabbitmq-amqp-go-client/pkg/rabbitmqamqp"
)

type MessageProcessor interface {
	Handle(ctx context.Context, data []byte) error
}

type Consumer struct {
	conn      *rmq.AmqpConnection
	processor MessageProcessor
	workers   int
	queueName string
	logger    *slog.Logger
}

func NewConsumer(
	processor MessageProcessor,
	queueName string,
	conn *rmq.AmqpConnection,
	logger *slog.Logger,
) *Consumer {
	if logger == nil {
		logger = slog.Default()
	}
	return &Consumer{
		conn:      conn,
		processor: processor,
		workers:   3,
		queueName: queueName,
		logger:    logger.With(slog.String("component", "consumer"), slog.String("queue", queueName)),
	}
}

func (c *Consumer) Run(ctx context.Context) error {
	rmqConsumer, err := c.conn.NewConsumer(
		ctx,
		c.queueName,
		&rmq.ConsumerOptions{InitialCredits: int32(c.workers)},
	)
	if err != nil {
		return fmt.Errorf("creating consumer: %w", err)
	}

	jobs := make(chan rmq.IDeliveryContext, c.workers)
	var wg sync.WaitGroup

	for i := 0; i < c.workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for delivery := range jobs {
				c.processMessage(workerID, delivery)
			}
		}(i)
	}

	c.logger.Info("consumer started", slog.Int("workers", c.workers))

consumeLoop:
	for {
		delivery, err := rmqConsumer.Receive(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) || ctx.Err() != nil {
				c.logger.Info("stopping consumer loop", slog.Any("reason", err))
				break consumeLoop
			}

			time.Sleep(50 * time.Millisecond)
			continue
		}

		select {
		case jobs <- delivery:
		case <-ctx.Done():
			c.safeRequeue(delivery)
			break consumeLoop
		}
	}

	close(jobs)
	wg.Wait()

	closeCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := rmqConsumer.Close(closeCtx); err != nil {
		return fmt.Errorf("closing consumer: %w", err)
	}

	c.logger.Info("consumer shut down cleanly")
	return nil
}

func (c *Consumer) processMessage(workerID int, delivery rmq.IDeliveryContext) {
	log := c.logger.With(slog.Int("worker_id", workerID))

	defer func() {
		if r := recover(); r != nil {
			log.Error("recovered panic in worker", slog.Any("panic", r))
			c.safeRequeue(delivery)
		}
	}()

	msg := delivery.Message()
	if msg != nil && msg.Header != nil && msg.Header.DeliveryCount >= 3 {
		log.Warn("message exceeded max delivery count, discarding to DLQ",
			slog.Int("delivery_count", int(msg.Header.DeliveryCount)),
		)
		c.safeDiscard(delivery, "max delivery attempts exceeded")
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	data := delivery.Message().GetData()
	err := c.processor.Handle(ctx, data)
	cancel()

	if err == nil {
		c.safeAccept(delivery)
		return
	}

	if errors.Is(err, application.ErrDiscard) {
		log.Warn("discarding message to DLQ", slog.Any("reason", err))
		c.safeDiscard(delivery, err.Error())
		return
	}

	log.Warn("requeueing message after temporary failure", slog.Any("reason", err))
	c.safeRequeue(delivery)
}

func (c *Consumer) safeAccept(delivery rmq.IDeliveryContext) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := delivery.Accept(ctx); err != nil {
		c.logger.Error("failed to accept message", slog.Any("error", err))
	}
}

func (c *Consumer) safeRequeue(delivery rmq.IDeliveryContext) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := delivery.Requeue(ctx); err != nil {
		c.logger.Error("failed to requeue message", slog.Any("error", err))
	}
}

func (c *Consumer) safeDiscard(delivery rmq.IDeliveryContext, reason string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := delivery.Discard(ctx, &amqp.Error{
		Condition:   amqp.ErrCondInvalidField,
		Description: reason,
	}); err != nil {
		c.logger.Error("failed to discard message", slog.Any("error", err))
	}
}
