// package garbagecollection

// import (
// 	"encoding/json"
// 	"errors"
// 	"fmt"
// 	"io"
// 	"net/http"
// 	"sync"
// 	"time"

// 	"github.com/coreos/etcd-operator/pkg/analytics"
// 	"github.com/coreos/etcd-operator/pkg/cluster"

// 	"github.com/astaxie/beego/logs"
// 	"github.com/ffan/tidb-k8s/models"
// 	"github.com/ffan/tidb-k8s/pkg/spec"
// 	"github.com/ffan/tidb-k8s/pkg/util/constants"
// 	"github.com/ffan/tidb-k8s/pkg/util/k8sutil"
// 	kwatch "k8s.io/apimachinery/pkg/watch"
// 	"k8s.io/client-go/kubernetes"
// )

// var (
// 	supportedPVProvisioners = map[string]struct{}{
// 		constants.PVProvisionerLocal: {},
// 		constants.PVProvisionerNone:  {},
// 	}

// 	ErrVersionOutdated = errors.New("requested version is outdated in apiserver")

// 	initRetryWaitTime = 30 * time.Second

// 	MasterHost string
// )

// type Event struct {
// 	Type   kwatch.EventType
// 	Object *models.Db
// }

// type Watcher struct {
// 	Config

// 	// TODO: combine the three cluster map.
// 	tidbs map[string]*models.Db
// 	// Kubernetes resource version of the clusters
// 	tidbRVs   map[string]string
// 	stopChMap map[string]chan struct{}

// 	waitCluster sync.WaitGroup
// }

// type Config struct {
// 	Namespace     string
// 	PVProvisioner string
// 	KubeCli       kubernetes.Interface
// }

// func (c *Config) Validate() error {
// 	if _, ok := supportedPVProvisioners[c.PVProvisioner]; !ok {
// 		return fmt.Errorf(
// 			"persistent volume provisioner %s is not supported: options = %v",
// 			c.PVProvisioner, supportedPVProvisioners,
// 		)
// 	}
// 	return nil
// }

// func New(cfg Config) *Watcher {
// 	return &Watcher{
// 		Config:    cfg,
// 		tidbs:     make(map[string]*models.Db),
// 		tidbRVs:   make(map[string]string),
// 		stopChMap: map[string]chan struct{}{},
// 	}
// }

// func (w *Watcher) Run() error {
// 	var (
// 		watchVersion string
// 		err          error
// 	)

// 	for {
// 		watchVersion, err = w.initResource()
// 		if err == nil {
// 			break
// 		}
// 		logs.Error("initialization failed: %v", err)
// 		logs.Info("retry in %v...", initRetryWaitTime)
// 		time.Sleep(initRetryWaitTime)
// 		// todo: add max retry?
// 	}

// 	logs.Info("starts running from watch version: %s", watchVersion)

// 	defer func() {
// 		for _, stopC := range w.stopChMap {
// 			close(stopC)
// 		}
// 		w.waitCluster.Wait()
// 	}()

// 	eventCh, errCh := w.watch(watchVersion)

// 	go func() {
// 		pt := newPanicTimer(time.Minute, "unexpected long blocking (> 1 Minute) when handling cluster event")

// 		for ev := range eventCh {
// 			pt.start()
// 			if err := c.handleClusterEvent(ev); err != nil {
// 				c.logger.Warningf("fail to handle event: %v", err)
// 			}
// 			pt.stop()
// 		}
// 	}()
// 	return <-errCh
// }

// func (c *Controller) handleClusterEvent(event *Event) error {
// 	clus := event.Object

// 	if clus.Status.IsFailed() {
// 		clustersFailed.Inc()
// 		if event.Type == kwatch.Deleted {
// 			delete(c.clusters, clus.Metadata.Name)
// 			delete(c.clusterRVs, clus.Metadata.Name)
// 			return nil
// 		}
// 		return fmt.Errorf("ignore failed cluster (%s). Please delete its TPR", clus.Metadata.Name)
// 	}

// 	// TODO: add validation to spec update.
// 	clus.Spec.Cleanup()

// 	switch event.Type {
// 	case kwatch.Added:
// 		stopC := make(chan struct{})
// 		nc := cluster.New(c.makeClusterConfig(), clus, stopC, &c.waitCluster)

// 		c.stopChMap[clus.Metadata.Name] = stopC
// 		c.clusters[clus.Metadata.Name] = nc
// 		c.clusterRVs[clus.Metadata.Name] = clus.Metadata.ResourceVersion

// 		analytics.ClusterCreated()
// 		clustersCreated.Inc()
// 		clustersTotal.Inc()

// 	case kwatch.Modified:
// 		if _, ok := c.clusters[clus.Metadata.Name]; !ok {
// 			return fmt.Errorf("unsafe state. cluster was never created but we received event (%s)", event.Type)
// 		}
// 		c.clusters[clus.Metadata.Name].Update(clus)
// 		c.clusterRVs[clus.Metadata.Name] = clus.Metadata.ResourceVersion
// 		clustersModified.Inc()

// 	case kwatch.Deleted:
// 		if _, ok := c.clusters[clus.Metadata.Name]; !ok {
// 			return fmt.Errorf("unsafe state. cluster was never created but we received event (%s)", event.Type)
// 		}
// 		c.clusters[clus.Metadata.Name].Delete()
// 		delete(c.clusters, clus.Metadata.Name)
// 		delete(c.clusterRVs, clus.Metadata.Name)
// 		analytics.ClusterDeleted()
// 		clustersDeleted.Inc()
// 		clustersTotal.Dec()
// 	}
// 	return nil
// }

// func (w *Watcher) findAllTidbs() (string, error) {
// 	logs.Info("finding existing tidbs...")
// 	tidbList, err := models.GetAllDbs()
// 	if err != nil {
// 		return "", err
// 	}

// 	for i := range tidbList.Items {
// 		tidb := tidbList.Items[i]

// 		stopC := make(chan struct{})
// 		w.stopChMap[tidb.Metadata.Name] = stopC
// 		w.tidbs[tidb.Metadata.Name] = tidb
// 		w.tidbRVs[tidb.Metadata.Name] = tidb.Metadata.ResourceVersion
// 	}

// 	return tidbList.Metadata.ResourceVersion, nil
// }

// func (w *Watcher) initResource() (string, error) {
// 	watchVersion := "0"
// 	err := w.createTPR()
// 	if err != nil {
// 		if k8sutil.IsKubernetesResourceAlreadyExistError(err) {
// 			// TPR has been initialized before. We need to recover existing cluster.
// 			watchVersion, err = w.findAllTidbs()
// 			if err != nil {
// 				return "", err
// 			}
// 		} else {
// 			return "", fmt.Errorf("fail to create TPR: %v", err)
// 		}
// 	}
// 	if w.Config.PVProvisioner != constants.PVProvisionerNone {
// 	}
// 	return watchVersion, nil
// }

// func (w *Watcher) createTPR() error {
// 	if err := k8sutil.CreateTPR(spec.TPRKindTidb); err != nil {
// 		return err
// 	}

// 	return k8sutil.WaitEtcdTPRReady(w.kubeCli.CoreV1().RESTClient(), 3*time.Second, 30*time.Second, w.Namespace)
// }

// // watch creates a go routine, and watches the cluster.etcd kind resources from
// // the given watch version. It emits events on the resources through the returned
// // event chan. Errors will be reported through the returned error chan. The go routine
// // exits on any error.
// func (w *Watcher) watch(watchVersion string) (<-chan *Event, <-chan error) {
// 	eventCh := make(chan *Event)
// 	// On unexpected error case, controller should exit
// 	errCh := make(chan error, 1)

// 	go func() {
// 		defer close(eventCh)

// 		for {
// 			resp, err := k8sutil.WatchTidbs(MasterHost, w.Config.Namespace, watchVersion)
// 			if err != nil {
// 				errCh <- err
// 				return
// 			}
// 			if resp.StatusCode != http.StatusOK {
// 				resp.Body.Close()
// 				errCh <- errors.New("invalid status code: " + resp.Status)
// 				return
// 			}

// 			logs.Info("start watching at %v", watchVersion)

// 			decoder := json.NewDecoder(resp.Body)
// 			for {
// 				ev, st, err := pollEvent(decoder)
// 				if err != nil {
// 					if err == io.EOF { // apiserver will close stream periodically
// 						c.logger.Debug("apiserver closed stream")
// 						break
// 					}

// 					logs.Error("received invalid event from API server: %v", err)
// 					errCh <- err
// 					return
// 				}

// 				if st != nil {
// 					resp.Body.Close()

// 					if st.Code == http.StatusGone {
// 						// event history is outdated.
// 						// if nothing has changed, we can go back to watch again.
// 						clusterList, err := k8sutil.GetClusterList(c.Config.KubeCli.CoreV1().RESTClient(), c.Config.Namespace)
// 						if err == nil && !c.isClustersCacheStale(clusterList.Items) {
// 							watchVersion = clusterList.Metadata.ResourceVersion
// 							break
// 						}

// 						// if anything has changed (or error on relist), we have to rebuild the state.
// 						// go to recovery path
// 						errCh <- ErrVersionOutdated
// 						return
// 					}

// 					c.logger.Fatalf("unexpected status response from API server: %v", st.Message)
// 				}

// 				c.logger.Debugf("etcd cluster event: %v %v", ev.Type, ev.Object.Spec)

// 				watchVersion = ev.Object.Metadata.ResourceVersion
// 				eventCh <- ev
// 			}

// 			resp.Body.Close()
// 		}
// 	}()

// 	return eventCh, errCh
// }

// func (c *Controller) isClustersCacheStale(currentClusters []spec.Cluster) bool {
// 	if len(c.clusterRVs) != len(currentClusters) {
// 		return true
// 	}

// 	for _, cc := range currentClusters {
// 		rv, ok := c.clusterRVs[cc.Metadata.Name]
// 		if !ok || rv != cc.Metadata.ResourceVersion {
// 			return true
// 		}
// 	}

// 	return false
// }
