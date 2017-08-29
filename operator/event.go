package operator

import (
	"fmt"
	"sync"
	"time"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-operator/pkg/spec"
	"github.com/ffan/tidb-operator/pkg/storage"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// Ewarning event warning type
	Ewarning = "warning"
	// Eerror event error type
	Eerror = "error"
	// Eok event ok type
	Eok = "ok"
)

var (
	eventPd       = "pd"
	eventTikv     = "tikv"
	eventTidb     = "tidb"
	eventMigrator = "migrator"
	eventDb       = "db"
)

// Events resource
type Events struct {
	metav1.TypeMeta `json:",inline"`
	Metadata        metav1.ObjectMeta `json:"metadata,omitempty"`

	Events []*Event `json:"events"`
}

// Event record the tidb creation process
type Event struct {
	Cell            string    `json:"cell,omitempty"`
	SourceComponent string    `json:"sc,omitempty"`
	Key             string    `json:"key,omitempty"`
	Type            string    `json:"type,omitempty"`
	Message         string    `json:"msg,omitempty"`
	FirstSeen       time.Time `json:"first,omitempty"`
	LastSeen        time.Time `json:"last,omitempty"`
	Count           int       `json:"count"`
}

var (
	evtMu sync.Mutex
	evtS  *storage.Storage
)

func eventInit() {
	s, err := storage.NewStorage(getNamespace(), spec.CRDKindEvent)
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
		Count:           1,
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
		e.Message = fmt.Sprintf("%s: %v", msg, err)
		logs.Error("%s[comp:%s, key:%s]: %s", e.Cell, e.SourceComponent, e.Key, e.Message)
	}
	e.save()
}

func (e *Event) save() error {
	e.LastSeen = time.Now()

	evtMu.Lock()
	defer evtMu.Unlock()
	es, err := GetEventsBy(e.Cell)
	if err != nil {
		if err != storage.ErrNoNode {
			return err
		}
		es = &Events{
			TypeMeta: metav1.TypeMeta{
				Kind:       spec.CRDKindEvent,
				APIVersion: spec.SchemeGroupVersion.String(),
			},
			Metadata: metav1.ObjectMeta{
				Name: e.Cell,
			},
		}
	}
	have := false
	for i := range es.Events {
		old := es.Events[i]
		if old.String() == e.String() {
			e.Count++
			es.Events[i] = e
			have = true
			break
		}
	}
	if !have {
		es.Events = append(es.Events, e)
	}
	if es.Metadata.ResourceVersion == "" {
		if err = es.save(); err != nil {
			return err
		}
	}
	return es.update()
}

func (e *Event) String() string {
	return e.Cell + "|" + e.SourceComponent + "|" + e.Message
}

func (es *Events) save() error {
	if err := evtS.Create(es); err != nil {
		return err
	}
	return nil
}

func (es *Events) update() error {
	if err := evtS.RetryUpdate(es.Metadata.Name, es); err != nil {
		return err
	}
	return nil
}

// GetEventsBy get cell events
func GetEventsBy(cell string) (*Events, error) {
	es := &Events{}
	if err := evtS.Get(cell, es); err != nil {
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

// Event creates an event associated with db
func (db *Db) Event(comp, key string) *Event {
	return NewEvent(db.GetName(), comp, key)
}
