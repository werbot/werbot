package asynq

import (
	"runtime"
	"time"

	"github.com/hibiken/asynq"

	"github.com/werbot/werbot/pkg/uuid"
	"github.com/werbot/werbot/pkg/worker"
)

type ServerOption func(s *server) error

// BatchConfig is ...
func BatchConfig(maxSize, maxDelay, gracePeriod int) ServerOption {
	return func(s *server) error {
		s.batchConfig.maxSize = maxSize
		s.batchConfig.maxDelay = time.Second * time.Duration(maxDelay)
		s.batchConfig.gracePeriod = time.Second * time.Duration(gracePeriod)
		return nil
	}
}

type server struct {
	redisURI    string
	asynqSrv    *asynq.Server
	asynqMux    *asynq.ServeMux
	asynqSch    *asynq.Scheduler
	batchConfig *batchConfig

	queues   queues
	tasks    []worker.Task
	cronjobs []worker.Cronjob
}

// NewServer is ...
func NewServer(redisURI string, opts ...ServerOption) worker.Server {
	s := &server{
		redisURI:    redisURI,
		queues:      queues{cronQueue: 1},
		tasks:       []worker.Task{},
		cronjobs:    []worker.Cronjob{},
		batchConfig: &batchConfig{},
	}

	for _, opt := range opts {
		if err := opt(s); err != nil {
			return nil
		}
	}

	return s
}

func (s *server) HandleTask(pattern worker.TaskPattern, handler worker.TaskHandler, opts ...worker.TaskOption) {
	pattern.MustValidate()

	if _, ok := s.queues[pattern.Queue()]; !ok {
		s.queues[pattern.Queue()] = 1
	}

	task := worker.Task{Pattern: pattern, Handler: handler}
	for _, opt := range opts {
		opt(&task)
	}

	s.tasks = append(s.tasks, task)
}

// HandleCron is ...
func (s *server) HandleCron(spec worker.CronSpec, handler worker.CronHandler) {
	spec.MustValidate()

	cronjob := worker.Cronjob{
		Identifier: uuid.New(),
		Spec:       spec,
		Handler:    handler,
	}

	s.cronjobs = append(s.cronjobs, cronjob)
}

func (s *server) Start() error {
	addr, err := asynq.ParseRedisURI(s.redisURI)
	if err != nil {
		return err
	}

	logLevel := asynq.WarnLevel

	s.asynqSch = asynq.NewScheduler(
		addr,
		&asynq.SchedulerOpts{
			LogLevel: logLevel,
		},
	)
	s.asynqMux = asynq.NewServeMux()
	s.asynqSrv = asynq.NewServer(
		addr,
		asynq.Config{
			Concurrency:      runtime.NumCPU(),
			Queues:           s.queues,
			LogLevel:         logLevel,
			GroupAggregator:  asynq.GroupAggregatorFunc(aggregate),
			GroupMaxSize:     s.batchConfig.maxSize,
			GroupMaxDelay:    s.batchConfig.maxDelay,
			GroupGracePeriod: s.batchConfig.gracePeriod,
		},
	)

	for _, t := range s.tasks {
		s.asynqMux.HandleFunc(t.Pattern.String(), taskToAsynq(t.Handler))
	}

	for _, c := range s.cronjobs {
		s.asynqMux.HandleFunc(c.Identifier, cronToAsynq(c.Handler))
		task := asynq.NewTask(c.Identifier, nil, asynq.Queue(cronQueue))
		if _, err := s.asynqSch.Register(c.Spec.String(), task); err != nil {
			return worker.ErrHandleCronFailed
		}
	}

	if err := s.asynqSrv.Start(s.asynqMux); err != nil {
		return worker.ErrServerStartFailed
	}

	if err := s.asynqSch.Start(); err != nil {
		return worker.ErrServerStartFailed
	}

	return nil
}

func (s *server) Shutdown() {
	s.asynqSrv.Shutdown()
	s.asynqSch.Shutdown()
}
