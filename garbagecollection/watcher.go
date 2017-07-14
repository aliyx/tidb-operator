package garbagecollection

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"encoding/json"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-operator/operator"
	"github.com/ffan/tidb-operator/pkg/spec"
	"github.com/ffan/tidb-operator/pkg/util/constants"
	"github.com/ffan/tidb-operator/pkg/util/k8sutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	kwatch "k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"
)

const (
	cleanInterval = 30 * time.Second
)

var (
	supportedPVProvisioners = map[string]struct{}{
		constants.PVProvisionerHostpath: {},
		constants.PVProvisionerNone:     {},
	}
	pvProvisioner PVProvisioner

	// ErrVersionOutdated tidb TPR version outdated
	ErrVersionOutdated = errors.New("requested version is outdated in apiserver")

	initRetryWaitTime = 30 * time.Second

	// registry type db to schema for codec

	schemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	// AddToScheme add user scheme to codec
	AddToScheme = schemeBuilder.AddToScheme
)

// addKnownTypes adds the set of types defined in this package to the supplied scheme.
func addKnownTypes(scheme *runtime.Scheme) error {
	gvk := schema.GroupVersionKind{
		Group:   spec.TPRGroup,
		Version: spec.TPRVersion,
		Kind:    "Tidb",
	}
	scheme.AddKnownTypeWithName(gvk,
		&operator.Db{},
	)
	metav1.AddToGroupVersion(scheme, spec.SchemeGroupVersion)
	return nil
}

// Event tidb TPR event
type Event struct {
	Type   kwatch.EventType
	Object *operator.Db
}

// Watcher watch tidb cluster changes, and make the appropriate deal
type Watcher struct {
	Config

	dbs map[string]*operator.Db
	// Kubernetes resource version of the clusters
	dbRVs map[string]string
}

// Config watch config
type Config struct {
	HostName      string
	Namespace     string
	PVProvisioner string
	Tprclient     *rest.RESTClient
}

// Validate validate config
func (c *Config) Validate() error {
	if _, ok := supportedPVProvisioners[c.PVProvisioner]; !ok {
		return fmt.Errorf(
			"persistent volume provisioner %s is not supported: options = %v",
			c.PVProvisioner, supportedPVProvisioners,
		)
	}
	return nil
}

// NewWatcher new a new watcher isntance
func NewWatcher(cfg Config) *Watcher {
	return &Watcher{
		Config: cfg,
		dbs:    make(map[string]*operator.Db),
		dbRVs:  make(map[string]string),
	}
}

// Run run watcher, exit when an error occurs
func (w *Watcher) Run() error {
	var (
		watchVersion string
		err          error
	)

	for {
		watchVersion, err = w.initResource()
		if err == nil {
			break
		}
		logs.Error("initialization failed: %v", err)
		logs.Info("retry in %v...", initRetryWaitTime)
		time.Sleep(initRetryWaitTime)
		// todo: add max retry?
	}

	if err = w.recycle(); err != nil {
		return err
	}

	go func() {
		for {
			select {
			case <-time.After(cleanInterval):
				w.cleanClusters()
			}
		}
	}()

	logs.Info("starts running from watch version: %s", watchVersion)

	eventCh, errCh := w.watch(watchVersion)

	go func() {
		pt := newPanicTimer(time.Minute, "unexpected long blocking (> 1 Minute) when handling cluster event")

		for ev := range eventCh {
			pt.start()
			if err := w.handleTidbEvent(ev); err != nil {
				logs.Warn("fail to handle event: %v", err)
			}
			pt.stop()
		}
	}()
	return <-errCh
}

func (w *Watcher) cleanClusters() {
	for key := range w.dbs {
		db := w.dbs[key]
		err := operator.DeleteBuriedTikv(db)
		if err != nil {
			logs.Error("failed to delete buried tikv of %s: %v", db.GetName(), err)
		}
	}
}

func (w *Watcher) handleTidbEvent(event *Event) (err error) {
	db := event.Object
	db.AfterPropertiesSet()
	switch event.Type {
	case kwatch.Added:
		w.dbs[db.GetName()] = db
		w.dbRVs[db.GetName()] = db.Metadata.ResourceVersion
	case kwatch.Modified:
		if _, ok := w.dbs[db.GetName()]; !ok {
			return fmt.Errorf("unsafe state. tidb was never created but we received event (%s)", event.Type)
		}
		w.dbs[db.GetName()] = db
		w.dbRVs[db.GetName()] = db.Metadata.ResourceVersion
		if err = gc(w.dbs[db.GetName()], db, pvProvisioner); err != nil {
			return err
		}
	case kwatch.Deleted:
		if _, ok := w.dbs[db.GetName()]; !ok {
			return fmt.Errorf("unsafe state. tidb was never created but we received event (%s)", event.Type)
		}
		delete(w.dbs, db.GetName())
		delete(w.dbRVs, db.GetName())
		if err = gc(w.dbs[db.GetName()], nil, pvProvisioner); err != nil {
			return err
		}
	}
	return err
}

func (w *Watcher) findAllDbs() (string, error) {
	logs.Info("finding existing tidbs...")
	dbList, err := operator.GetAllDbs()
	if err != nil {
		return "", err
	}
	if dbList == nil {
		return "", nil
	}

	for i := range dbList.Items {
		db := &dbList.Items[i]
		db.AfterPropertiesSet()
		w.dbs[db.Metadata.Name] = db
		w.dbRVs[db.Metadata.Name] = db.Metadata.ResourceVersion
	}

	return dbList.Metadata.ResourceVersion, nil
}

func (w *Watcher) initResource() (string, error) {
	var (
		watchVersion = "0"
		err          error
	)
	if err = k8sutil.CreateTPR(spec.TPRKindTidb); err != nil {
		return "", fmt.Errorf("fail to create TPR: %v", err)
	}
	watchVersion, err = w.findAllDbs()
	if err != nil {
		return "", err
	}

	switch w.PVProvisioner {
	case constants.PVProvisionerNone:
		logs.Info("current pv provisioner is pod.")
		pvProvisioner = &NilPVProvisioner{}
	case constants.PVProvisionerHostpath:
		md, err := operator.GetMetadata()
		if err != nil {
			return "", err
		}
		logs.Info("current pv provisioner is hostpath, path: %s", md.Spec.K8s.Volume)
		pvProvisioner = &HostPathPVProvisioner{
			HostName: w.HostName,
			Dir:      md.Spec.K8s.Volume,
		}
	}
	return watchVersion, nil
}

// recycle unrecycled resource
func (w *Watcher) recycle() error {
	var all []*operator.Store
	for _, db := range w.dbs {
		for _, s := range db.Tikv.Stores {
			all = append(all, s)
		}
	}
	return pvProvisioner.Clean(all)
}

// watch creates a go routine, and watches the tidb kind resources from
// the given watch version. It emits events on the resources through the returned
// event chan. Errors will be reported through the returned error chan. The go routine
// exits on any error.
func (w *Watcher) watch(watchVersion string) (<-chan *Event, <-chan error) {
	eventCh := make(chan *Event)
	// On unexpected error case, watcher should exit
	errCh := make(chan error, 1)

	go func() {
		defer close(eventCh)

		for {
			resp, err := k8sutil.WatchTidbs(w.Tprclient, w.Namespace, watchVersion)
			if err != nil {
				logs.Error("watch tidb: %v", err)
				errCh <- err
				return
			}
			logs.Info("start watching at %v", watchVersion)
			for {
				e, ok := <-resp.ResultChan()
				// no more values to receive and the channel is closed
				if !ok {
					break
				}
				obj, _ := json.Marshal(e.Object)
				logs.Debug("tidb cluster event: %v %s", e.Type, obj)
				ev, st := parse(e)
				if st != nil {
					resp.Stop()

					if st.Code == http.StatusGone {
						// event history is outdated.
						// if nothing has changed, we can go back to watch again.
						dbList, err := operator.GetAllDbs()
						if err == nil && !w.isDbsCacheUnstable(dbList.Items) {
							watchVersion = dbList.Metadata.ResourceVersion
							break
						}

						// if anything has changed (or error on relist), we have to rebuild the state.
						// go to recovery path
						errCh <- ErrVersionOutdated
						return
					}

					logs.Critical("unexpected status response from API server: %v", st.Message)
				}

				watchVersion = ev.Object.Metadata.ResourceVersion
				eventCh <- ev
			}
			errCh <- errors.New("test")
		}
	}()

	return eventCh, errCh
}

func (w *Watcher) isDbsCacheUnstable(currentDbs []operator.Db) bool {
	if len(w.dbRVs) != len(currentDbs) {
		return true
	}

	for _, cd := range currentDbs {
		rv, ok := w.dbRVs[cd.Metadata.Name]
		if !ok || rv != cd.Metadata.ResourceVersion {
			return true
		}
	}

	return false
}
