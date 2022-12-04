package nats

import (
	"sync"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/werbot/werbot/internal/logger"
)

var log = logger.New("pkg/nats")

// Service is ...
type Service struct {
	con *nats.Conn
}

// New is ...
func New(dsn string) *Service {
	nc, err := nats.Connect(dsn, nats.ReconnectWait(5*time.Second))
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to connect to NATS server")
	}
	// defer nc.Close()

	return &Service{
		con: nc,
	}
}

// AccountStatus is ...
// func (n *Service) AccountStatus(accountID int32, status string) {
//	message := []byte(fmt.Sprintf(`{"account.%v":{"id":%d}}`, status, accountID))
//	if err := n.con.Publish("server.events", message); err != nil {
//		log.Error().Err(err).Msgf("Publish in server.events")
//	}
//}

// WriteStatus is ...
// for the test :)
// func (n *Service) WriteStatus() {
//	n.con.Subscribe("server.events", func(m *nats.Msg) {
//		fmt.Printf("Received a message: %s\n", string(m.Data))
//	})
//}

// WriteConsole is ...
func (n *Service) WriteConsole() {
	ec, err := nats.NewEncodedConn(n.con, nats.JSON_ENCODER)
	if err != nil {
		log.Error().Err(err).Msgf("Error with NewEncodedConn")
	}
	// defer ec.Close()

	type actions struct {
		Action map[string]map[string]int
	}

	wg := sync.WaitGroup{}
	wg.Add(1)

	if _, err := ec.Subscribe("console.actions", func(s *actions) {
		if val, ok := s.Action["server.inactive"]; ok {
			log.Info().Int("hostId", val["id"]).Msg("AMQP server.inactive")
		}
		if val, ok := s.Action["account.inactive"]; ok {
			log.Info().Int("accountId", val["id"]).Msg("AMQP account.inactive")
		}
		wg.Done()
	}); err != nil {
		log.Error().Err(err).Msgf("Error Subscribe")
	}

	wg.Wait()
}
