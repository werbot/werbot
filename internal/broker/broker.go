package broker

import (
	"context"
	"fmt"

	"github.com/redis/go-redis/v9"
	"github.com/werbot/werbot/pkg/logger"
)

var log = logger.New()

// Handler is ...
type Handler struct {
	ctx    context.Context
	client *redis.Client
}

// New is ...
func New(ctx context.Context, client *redis.Client) Handler {
	return Handler{
		ctx:    ctx,
		client: client,
	}
}

// WriteConsole is ...
func (c Handler) WriteConsole() error {
	sub := c.client.Subscribe(c.ctx, "console.actions")
	defer sub.Close()

	for {
		msg, err := sub.ReceiveMessage(c.ctx)
		if err != nil {
			return err
		}
		fmt.Println(msg.Channel, msg.Payload)
		/*
		   if val, ok := s.Action["server.inactive"]; ok {
		     log.Info().Int("hostId", val["id"]).Msg("AMQP server.inactive")
		   }
		   if val, ok := s.Action["account.inactive"]; ok {
		     log.Info().Int("accountId", val["id"]).Msg("AMQP account.inactive")
		   }
		*/
	}
}

// AccountStatus is ...
func (c Handler) AccountStatus(accountID, status string) error {
	message := []byte(fmt.Sprintf(`{"account.%v":{"id":%v}}`, status, accountID))
	pub := c.client.Publish(c.ctx, "server.events", message)
	if err := pub.Err(); err != nil {
		return err
	}
	return nil
}

// WriteStatus is ...
func (c Handler) WriteStatus() error {
	sub := c.client.Subscribe(c.ctx, "server.events")
	defer sub.Close()

	for {
		msg, err := sub.ReceiveMessage(c.ctx)
		if err != nil {
			return err
		}
		fmt.Println(msg.Channel, msg.Payload)
	}
}
