package broker

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// var log = logger.New()

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
		   if val, ok := s.Action["scheme.inactive"]; ok {
		     log.Info().Int("hostId", val["id"]).Msg("AMQP scheme.inactive")
		   }
		   if val, ok := s.Action["account.inactive"]; ok {
		     log.Info().Int("accountId", val["id"]).Msg("AMQP account.inactive")
		   }
		*/
	}
}

// AccountStatus is ...
func (c Handler) AccountStatus(schemeID, status string) error {
	payload, err := json.Marshal(map[string]map[string]string{
		"scheme." + status: {"id": schemeID},
	})
	if err != nil {
		return err
	}

	pub := c.client.Publish(c.ctx, "scheme.events", payload)
	return pub.Err()
}

// WriteStatus is ...
func (c Handler) WriteStatus() error {
	sub := c.client.Subscribe(c.ctx, "scheme.events")
	defer sub.Close()

	for {
		msg, err := sub.ReceiveMessage(c.ctx)
		if err != nil {
			return err
		}
		fmt.Println(msg.Channel, msg.Payload)
	}
}
