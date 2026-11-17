package broker

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	rmq "github.com/rabbitmq/rabbitmq-amqp-go-client/pkg/rabbitmqamqp"
)

type RabbitClient struct {
	env    *rmq.Environment
	conn   *rmq.AmqpConnection
	logger *slog.Logger
}

func Connect(ctx context.Context, url string, logger *slog.Logger) (*RabbitClient, error) {
	if logger == nil {
		logger = slog.Default()
	}
	log := logger.With(slog.String("component", "rabbit_connection"))

	options := &rmq.AmqpConnOptions{
		RecoveryConfiguration: &rmq.RecoveryConfiguration{
			ActiveRecovery:           true,
			BackOffReconnectInterval: 3 * time.Second,
			MaxReconnectAttempts:     10,
		},
	}

	env := rmq.NewEnvironment(url, options)
	conn, err := env.NewConnection(ctx)
	if err != nil {
		return nil, fmt.Errorf("creating RabbitMQ connection: %w", err)
	}

	stateChanged := make(chan *rmq.StateChanged, 5)
	conn.NotifyStatusChange(stateChanged)
	go func(ch <-chan *rmq.StateChanged) {
		for state := range ch {
			log.Info("connection state changed", slog.String("state", fmt.Sprintf("%v", state)))
		}
	}(stateChanged)

	log.Info("connected to RabbitMQ")

	return &RabbitClient{
		env:    env,
		conn:   conn,
		logger: log,
	}, nil
}

func (r *RabbitClient) Connection() *rmq.AmqpConnection {
	return r.conn
}

func (r *RabbitClient) Close(ctx context.Context) error {
	if err := r.env.CloseConnections(ctx); err != nil {
		return fmt.Errorf("closing RabbitMQ environment: %w", err)
	}
	r.logger.Info("RabbitMQ connections closed")
	return nil
}
