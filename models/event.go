package models

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/ffan/tidb-k8s/pkg/storage"
)

const (
	// Ewarning event warning type
	Ewarning = "warning"
	// Eerror event error type
	Eerror = "error"
	// Eok event ok type
	Eok = "ok"
)

// Event record the tidb creation process
type Event struct {
	Cell            string    `json:"cell,omitempty"`
	SourceComponent string    `json:"sc,omitempty"`
	Key             string    `json:"key,omitempty"`
	Type            string    `json:"type,omitempty"`
	Message         string    `json:"msg,omitempty"`
	FirstSeen       time.Time `json:"first,omitempty"`
	LastSeen        time.Time `json:"last,omitempty"`
}

var (
	evtMu sync.Mutex
	evtS  storage.Storage
)

func eventInit() {
	s, err := storage.NewDefaultStorage(eventNamespace, etcdAddress)
	if err != nil {
		panic(fmt.Errorf("Create storage event error: %v", err))
	}
	evtS = s
}

// NewEvent new a event instance
func NewEvent(cell, comp, key string) *Event {
	return &Event{
		Cell:            cell,
		Key:             key,
		FirstSeen:       time.Now(),
		SourceComponent: comp,
	}
}

// Trace record event
func (e *Event) Trace(err error, msg ...string) {
	e.Type = Eok
	e.Message = msg[0]
	if len(msg) > 1 {
		e.Type = msg[1] // status
	}
	if err != nil {
		e.Type = Eerror
		e.Message = fmt.Sprintf("%s:%v", msg, err)
	}
	e.save()
}

func (e *Event) save() error {
	evtMu.Lock()
	defer evtMu.Unlock()
	es, err := GetEventsBy(e.Cell)
	if err != nil {
		return err
	}
	e.LastSeen = time.Now()
	es = append(es, *e)
	if err := save(es...); err != nil {
		return err
	}
	return nil
}

func save(es ...Event) error {
	if len(es) < 1 {
		return nil
	}
	for _, e := range es {
		if e.Cell == "" {
			return fmt.Errorf("cell is nil")
		}
	}
	j, err := json.Marshal(es)
	if err != nil {
		return err
	}
	if err := evtS.Create(es[0].Cell, j); err != nil {
		return err
	}
	return nil
}

// GetEventsBy get cell events
func GetEventsBy(cell string) ([]Event, error) {
	bs, err := evtS.Get(cell)
	if err != nil {
		if err != storage.ErrNoNode {
			return nil, err
		}
		return []Event{}, nil
	}
	es := []Event{}
	if err := json.Unmarshal(bs, &es); err != nil {
		return nil, err
	}
	return es, nil
}

// DelEventsBy del cell all events
func delEventsBy(cell string) error {
	if err := evtS.Delete(cell); err != nil && err != storage.ErrNoNode {
		return err
	}
	return nil
}
