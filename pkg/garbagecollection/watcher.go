package garbagecollection

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-k8s/models"
	"github.com/ffan/tidb-k8s/pkg/spec"
	"github.com/ffan/tidb-k8s/pkg/util/constants"
	"github.com/ffan/tidb-k8s/pkg/util/k8sutil"
	kwatch "k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
)

var (
	supportedPVProvisioners = map[string]struct{}{
		constants.PVProvisionerLocal: {},
		constants.PVProvisionerNone:  {},
	}

	ErrVersionOutdated = errors.New("requested version is outdated in apiserver")

	initRetryWaitTime = 30 * time.Second
)

type Event struct {
	Type   kwatch.EventType
	Object *models.Db
}

type Watcher struct {
	Config

	// TODO: combine the three cluster map.
	tidbs map[string]*models.Db
	// Kubernetes resource version of the clusters
	tidbRVs   map[string]string
	stopChMap map[string]chan struct{}

	waitCluster sync.WaitGroup
}

type Config struct {
	Namespace     string
	PVProvisioner string
	KubeCli       kubernetes.Interface
}

func (c *Config) Validate() error {
	if _, ok := supportedPVProvisioners[c.PVProvisioner]; !ok {
		return fmt.Errorf(
			"persistent volume provisioner %s is not supported: options = %v",
			c.PVProvisioner, supportedPVProvisioners,
		)
	}
	return nil
}

func New(cfg Config) *Watcher {
	return &Watcher{
		Config:    cfg,
		tidbs:     make(map[string]*models.Db),
		tidbRVs:   make(map[string]string),
		stopChMap: map[string]chan struct{}{},
	}
}

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

	logs.Info("starts running from watch version: %s", watchVersion)

	defer func() {
		for _, stopC := range w.stopChMap {
			close(stopC)
		}
		w.waitCluster.Wait()
	}()

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

func (w *Watcher) handleTidbEvent(event *Event) error {
	tidb := event.Object

	switch event.Type {
	case kwatch.Added:
		logs.Debug("add tidb: %+v", *tidb)

		w.stopChMap[tidb.Metadata.Name] = make(chan struct{})
		w.tidbs[tidb.Metadata.Name] = tidb
		w.tidbRVs[tidb.Metadata.Name] = tidb.Metadata.ResourceVersion
	case kwatch.Modified:
		logs.Debug("update tidb: %+v", *tidb)
		if _, ok := w.tidbs[tidb.Metadata.Name]; !ok {
			return fmt.Errorf("unsafe state. tidb was never created but we received event (%s)", event.Type)
		}
		// w.tidbs[tidb.Metadata.Name].Update(clus)
		w.tidbRVs[tidb.Metadata.Name] = tidb.Metadata.ResourceVersion

	case kwatch.Deleted:
		logs.Debug("delete tidb: %+v", *tidb)
		if _, ok := w.tidbs[tidb.Metadata.Name]; !ok {
			return fmt.Errorf("unsafe state. tidb was never created but we received event (%s)", event.Type)
		}
		// w.tidbs[tidb.Metadata.Name].Delete()
		delete(w.tidbs, tidb.Metadata.Name)
		delete(w.tidbRVs, tidb.Metadata.Name)
	}
	return nil
}

func (w *Watcher) findAllTidbs() (string, error) {
	logs.Info("finding existing tidbs...")
	tidbList, err := models.GetAllDbs()
	if err != nil {
		return "", err
	}
	if tidbList == nil {
		return "", nil
	}

	for i := range tidbList.Items {
		tidb := tidbList.Items[i]
		w.stopChMap[tidb.Metadata.Name] = make(chan struct{})
		w.tidbs[tidb.Metadata.Name] = &tidb
		w.tidbRVs[tidb.Metadata.Name] = tidb.Metadata.ResourceVersion
	}

	return tidbList.Metadata.ResourceVersion, nil
}

func (w *Watcher) initResource() (string, error) {
	var (
		watchVersion = "0"
		err          error
	)
	if err = w.createTPR(); err != nil {
		return "", fmt.Errorf("fail to create TPR: %v", err)
	}
	watchVersion, err = w.findAllTidbs()
	if err != nil {
		return "", err
	}

	if w.Config.PVProvisioner != constants.PVProvisionerNone {

	}
	return watchVersion, nil
}

func (w *Watcher) createTPR() error {
	if err := k8sutil.CreateTPR(spec.TPRKindTidb); err != nil {
		return err
	}
	return nil
}

// watch creates a go routine, and watches the cluster.etcd kind resources from
// the given watch version. It emits events on the resources through the returned
// event chan. Errors will be reported through the returned error chan. The go routine
// exits on any error.
func (w *Watcher) watch(watchVersion string) (<-chan *Event, <-chan error) {
	eventCh := make(chan *Event)
	// On unexpected error case, controller should exit
	errCh := make(chan error, 1)

	go func() {
		defer close(eventCh)

		for {
			resp, err := k8sutil.WatchTidbs(w.Config.Namespace, watchVersion)
			if err != nil {
				errCh <- err
				return
			}
			if resp.StatusCode != http.StatusOK {
				resp.Body.Close()
				errCh <- errors.New("invalid status code: " + resp.Status)
				return
			}

			logs.Info("start watching at %v", watchVersion)

			decoder := json.NewDecoder(resp.Body)
			for {
				ev, st, err := pollEvent(decoder)
				if err != nil {
					if err == io.EOF { // apiserver will close stream periodically
						logs.Debug("apiserver closed stream")
						break
					}

					logs.Error("received invalid event from API server: %v", err)
					errCh <- err
					return
				}

				if st != nil {
					resp.Body.Close()

					if st.Code == http.StatusGone {
						// event history is outdated.
						// if nothing has changed, we can go back to watch again.
						tidbList, err := models.GetAllDbs()
						if err == nil && !w.isTidbsCacheStale(tidbList.Items) {
							watchVersion = tidbList.Metadata.ResourceVersion
							break
						}

						// if anything has changed (or error on relist), we have to rebuild the state.
						// go to recovery path
						errCh <- ErrVersionOutdated
						return
					}

					logs.Critical("unexpected status response from API server: %v", st.Message)
				}

				logs.Debug("tidb cluster event: %v %v", ev.Type, ev.Object)

				watchVersion = ev.Object.Metadata.ResourceVersion
				eventCh <- ev
			}

			resp.Body.Close()
		}
	}()

	return eventCh, errCh
}

func (w *Watcher) isTidbsCacheStale(currentTidbs []models.Db) bool {
	if len(w.tidbRVs) != len(currentTidbs) {
		return true
	}

	for _, ct := range currentTidbs {
		rv, ok := w.tidbRVs[ct.Metadata.Name]
		if !ok || rv != ct.Metadata.ResourceVersion {
			return true
		}
	}

	return false
}
