package watcher

import (
	"github.com/fsnotify/fsnotify"
	"github.com/werbot/werbot/pkg/logger"
)

var (
	log = logger.New()
)

// Watcher is ...
type Watcher struct {
	Watcher  *fsnotify.Watcher // Watcher is ...
	Changes  chan struct{}     // Changes is ...
	Watching []string          // Watching is ...
}

// NewWatcher is ...
func NewWatcher() (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	w := &Watcher{
		Watcher: fsw,
		Changes: make(chan struct{}, 100),
	}

	go func() {
		for err := range fsw.Errors {
			log.Error(err).Send()
		}
	}()

	go func() {
		for event := range fsw.Events {
			if event.Op&fsnotify.Write == fsnotify.Write {
				w.Changes <- struct{}{}
			}
		}
	}()

	return w, nil
}

// Add watches a file for changes.
func (w *Watcher) Add(file string) {
	w.Watching = append(w.Watching, file)
	if err := w.Watcher.Add(file); err != nil {
		log.Error(err).Msg("Add file")
	}
}

// Close stops watching all files.
func (w *Watcher) Close() error {
	if err := w.Watcher.Close(); err != nil {
		return err
	}
	return nil
}

// StopWatchingAll stops watching all files.
func (w *Watcher) StopWatchingAll() {
	for _, f := range w.Watching {
		if err := w.Watcher.Remove(f); err != nil {
			log.Error(err).Send()
		}
	}
	w.Watching = nil
}
