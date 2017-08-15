package operator

import (
	"fmt"
	"strings"

	"k8s.io/client-go/pkg/api/v1"

	"time"

	"errors"

	"sort"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-operator/pkg/util/httputil"
	"github.com/ffan/tidb-operator/pkg/util/k8sutil"
	"github.com/ffan/tidb-operator/pkg/util/pdutil"
	"github.com/ffan/tidb-operator/pkg/util/retryutil"
	"github.com/ghodss/yaml"
	"github.com/tidwall/gjson"
)

const (
	defaultTikvPort = 20160
)

var (
	errMultipleStoresOneAddress = errors.New("multiple stores one address")
)

func (tk *Tikv) upgrade() (err error) {
	if len(tk.Stores) < 1 {
		return nil
	}
	var (
		upgraded bool
		count    int
		image    = fmt.Sprintf("%s/tikv:%s", imageRegistry, tk.Version)
	)

	e := tk.Db.Event(eventTikv, "upgrate")
	defer func() {
		// have upgrade
		if count > 0 || err != nil {
			e.Trace(err, fmt.Sprintf("Upgrate tikv to version: %s", tk.Version))
		}
	}()

	if tk.Db.Status.Phase < PhaseTikvStarted {
		err = ErrUnavailable
		return
	}

	names := tk.getSortedStoresKey()
	for _, name := range names {
		upgraded, err = upgradeOne(name, image, tk.Version)
		if err != nil {
			return err
		}
		if upgraded {
			count++
			// wait ok
			tk.cur = name
			if err = tk.waitForStoreOk(); err != nil {
				return err
			}
			time.Sleep(tikvUpgradeInterval)
		}
	}
	return nil
}

func (tk *Tikv) install() (err error) {
	e := tk.Db.Event(eventTikv, "install")
	defer func() {
		ph := PhaseTikvStarted
		if err != nil {
			ph = PhaseTikvStartFailed
		}
		tk.Db.Status.Phase = ph
		e.Trace(err,
			fmt.Sprintf("Install tikv pods with replicas desire: %d, running: %d on k8s",
				tk.Replicas, tk.AvailableReplicas))
	}()

	// savepoint for page show
	tk.Db.Status.Phase = PhaseTikvPending
	if err = tk.Db.update(); err != nil {
		return err
	}

	tk.Stores = make(map[string]*Store)
	for r := 1; r <= tk.Replicas; r++ {
		tk.Member++
		if err = tk._install(); err != nil {
			return err
		}
	}
	return err
}

func (tk *Tikv) _install() (err error) {
	tk.cur = fmt.Sprintf("tikv-%s-%03d", tk.Db.GetName(), tk.Member)
	tk.Stores[tk.cur] = &Store{}
	tk.ReadyReplicas++
	if err = tk.createPod(); err != nil {
		return err
	}
	if err = tk.waitForStoreOk(); err != nil {
		return err
	}
	tk.AvailableReplicas++
	return nil
}

func (tk *Tikv) createPod() (err error) {
	var j []byte
	if j, err = tk.toJSONTemplate(tikvPodYaml); err != nil {
		return err
	}
	var pod *v1.Pod
	if pod, err = k8sutil.CreatePodByJSON(j, waitPodRuningTimeout, func(pod *v1.Pod) {
		k8sutil.SetTidbVersion(pod, tk.Version)
	}); err != nil {
		return err
	}
	s := tk.Stores[tk.cur]
	s.Name = tk.cur
	s.Address = fmt.Sprintf("%s:%d", pod.Status.PodIP, defaultTikvPort)
	s.Node = pod.Spec.NodeName
	return nil
}

func (tk *Tikv) toJSONTemplate(temp string) ([]byte, error) {
	r := strings.NewReplacer(
		"{{version}}", tk.Spec.Version,
		"{{cpu}}", fmt.Sprintf("%v", tk.Spec.CPU),
		"{{mem}}", fmt.Sprintf("%v", tk.Spec.Mem),
		"{{capacity}}", fmt.Sprintf("%v", tk.Spec.Capatity),
		"{{tidbdata_volume}}", fmt.Sprintf("%v", tk.Spec.Volume),
		"{{id}}", fmt.Sprintf("%03v", tk.Member),
		"{{registry}}", imageRegistry,
		"{{cell}}", tk.Db.GetName(),
		"{{mount}}", tk.Spec.Mount,
		"{{namespace}}", getNamespace())
	return yaml.YAMLToJSON([]byte(r.Replace(temp)))
}

func (tk *Tikv) waitForStoreOk() (err error) {
	interval := 3 * time.Second
	s := tk.Stores[tk.cur]
	err = retryutil.Retry(interval, int(waitTidbComponentAvailableTimeout/(interval)), func() (bool, error) {
		j, err := pdutil.PdStoresGet(tk.Db.Pd.OuterAddresses[0])
		if err != nil {
			logs.Error("could not get stores by pd API: %v", err)
			return false, nil
		}
		ret := gjson.Get(j, "count")
		if ret.Int() < 1 {
			logs.Warn("current stores count: %d", ret.Int())
			return false, nil
		}
		// get the tikv store
		ret = gjson.Get(j, fmt.Sprintf("stores.#[store.address==%s]#.store.id", s.Address))
		if ret.Type == gjson.Null {
			logs.Warn("cannt get store[%s]", tk.Stores[tk.cur].Address)
			return false, nil
		}
		if len(ret.Array()) != 1 {
			logs.Error("get multiple store by address[%s]", tk.Stores[tk.cur].Address)
			return false, errMultipleStoresOneAddress
		}
		s.ID = int(ret.Array()[0].Int())
		s.State = StoreOnline
		return true, nil
	})
	if err != nil {
		logs.Error("tikv %q available: %v", tk.cur, err)
	}
	return err
}

// delete store that status is tombstone
func (tk *Tikv) deleteBuriedTikv() error {
	for name, s := range tk.Stores {
		st, err := tk.getStoreState(s)
		if err != nil {
			return err
		}
		if st == StoreTombstone {
			logs.Info("delete tikv: %q, status: tombstone", name)
			if err = pdutil.PdStoreDelete(tk.Db.Pd.OuterAddresses[0], s.ID); err != nil {
				return err
			}
			if err = k8sutil.DeletePod(name, terminationGracePeriodSeconds); err != nil {
				return err
			}
			tk.ReadyReplicas--
			delete(tk.Stores, name)
		}
	}
	return nil
}

func (tk *Tikv) getStoreState(s *Store) (StoreStatus, error) {
	j, err := pdutil.PdStoreIDGet(tk.Db.Pd.OuterAddresses[0], s.ID)
	if err != nil {
		if err == httputil.ErrNotFound {
			logs.Warn("can't get store:%d", s.ID)
			return StoreUnknown, nil
		}
		return StoreUnknown, err
	}
	ret := gjson.Get(j, "store.state")
	if ret.Type == gjson.Null {
		return StoreUnknown, fmt.Errorf("cannt get store[%s] state", s.Name)
	}
	return StoreStatus(ret.Int()), nil
}

func (tk *Tikv) uninstall() (err error) {
	if err = k8sutil.DeletePodsBy(tk.Db.GetName(), "tikv"); err != nil {
		return err
	}
	tk.Stores = nil
	tk.Member = 0
	tk.cur = ""
	tk.AvailableReplicas = 0
	tk.ReadyReplicas = 0
	return nil
}

func (db *Db) reconcileTikvs() error {
	var (
		err     error
		kv      = db.Tikv
		changed = true
	)

	e := db.Event(eventTikv, "reconcile")
	defer func(a, r int) {
		if err != nil {
			db.Status.ScaleState |= tikvScaleErr
		}
		if changed || err != nil {
			e.Trace(err, fmt.Sprintf("Reconcile tikv replicas from %d to %d", a, r))
		}
	}(kv.AvailableReplicas, kv.Replicas)

	// check available replica
	if err = kv.checkStores(); err != nil {
		logs.Error("check tikv %q stores status: %v", db.GetName(), err)
		return err
	}

	// no change
	if kv.Replicas == kv.AvailableReplicas && kv.Replicas == kv.ReadyReplicas {
		logs.Debug("tikv %q cluster is normal", kv.Db.GetName())
		changed = false
		return nil
	}

	logs.Info("start reconciling tikv %q cluster", kv.Db.GetName())
	r := kv.AvailableReplicas
	if r < kv.ReadyReplicas {
		r = kv.ReadyReplicas
	}
	switch n := kv.Replicas - r; {
	case n > 0:
		logs.Info("start scaling up tikv %q pods count from %d to %d", kv.Db.GetName(), r, kv.Replicas)
		err = kv.increase(n)
		logs.Info("end scale up tikv %q pods desire: %d, ready: %d, available: %d",
			db.GetName(), kv.Replicas, kv.ReadyReplicas, kv.AvailableReplicas)
	case n < 0:
		logs.Info("start scaling down tikv %q pods count from %d to %d",
			db.GetName(), r, kv.Replicas)
		err = kv.decrease(-n)
		logs.Info("end scale down tikv %q pods desire: %d, ready: %d, available: %d",
			db.GetName(), kv.Replicas, kv.ReadyReplicas, kv.AvailableReplicas)
	default:
		err = kv.reconcile()
	}
	logs.Info("end reconcile tikv %q cluster", kv.Db.GetName())
	return err
}

// mark offline store
func (tk *Tikv) checkStores() error {
	j, err := pdutil.PdStoresGet(tk.Db.Pd.OuterAddresses[0])
	if err != nil {
		return err
	}

	ret := gjson.Get(j, "count")
	if ret.Int() < 1 {
		logs.Warn("current db %q available stores count: 0", tk.Db.GetName())
		for _, s := range tk.Stores {
			logs.Warn("mark store %q offline", s.Name)
			s.State = StoreOffline
		}
		tk.AvailableReplicas = 0
		return nil
	}

	// Remove uncontrolled store
	ret = gjson.Get(j, "stores.#.store.id")
	if ret.Type != gjson.Null {
		for _, sid := range ret.Array() {
			have := false
			id := int(sid.Int())
			for _, s := range tk.Stores {
				if s.ID == id {
					have = true
					break
				}
			}
			if !have {
				logs.Warn("delete uncontrolled store id:%v", sid)
				if err = pdutil.PdStoreDelete(tk.Db.Pd.OuterAddresses[0], id); err != nil {
					return err
				}
			}
		}
	}

	// Remove uncontrolled pod
	pods, err := k8sutil.GetPods(tk.Db.GetName(), "tikv")
	if err != nil {
		return err
	}
	for _, pod := range pods {
		have := false
		for sn := range tk.Stores {
			if sn == pod.GetName() {
				have = true
			}
		}
		if !have {
			logs.Warn("delete uncontrolled pod %q", pod.GetName())
			if err = k8sutil.DeletePod(pod.GetName(), terminationGracePeriodSeconds); err != nil {
				return err
			}
		}
	}

	// get all online tikvs
	ret = gjson.Get(j, "stores.#[store.state_name==Up]#.store.id")
	if ret.Type == gjson.Null {
		return nil
	}
	for name, s := range tk.Stores {
		online := false
		for _, sid := range ret.Array() {
			id := int(sid.Int())
			if s.ID == id {
				online = true
				break
			}
		}
		if !online {
			if s.State == StoreOnline {
				logs.Warn("mark tikv %q offline", name)
				s.State = StoreOffline
				tk.AvailableReplicas--
			}
		} else if s.State == StoreOffline {
			// may be down -> up
			s.State = StoreOnline
			tk.AvailableReplicas++
		}
	}
	return nil
}

// Only mark store status is 'offline' and decrease AvailableReplicas
func (tk *Tikv) decrease(replicas int) (err error) {
	names := tk.getSortedStoresKey()
	for i := 0; i < replicas; i++ {
		name := names[i]
		if err = pdutil.PdStoreDemote(tk.Db.Pd.OuterAddresses[0], tk.Stores[name].ID); err != nil {
			return err
		}
		old := tk.Stores[name].State
		tk.Stores[name].State = StoreOffline
		tk.AvailableReplicas--
		logs.Warn("mark tikv %q state from %d to %d", name, old, StoreOffline)
	}

	return nil
}

func (tk *Tikv) increase(replicas int) (err error) {
	for i := 0; i < replicas; i++ {
		tk.Member++
		if err = tk._install(); err != nil {
			return err
		}
	}
	return nil
}

// Reconcile current available tikvs with desired
// 1. Delete the tikv that has not yet been registered
// 2. Delete buried tikv
// 3. Reconcile desire tikv consistent
func (tk *Tikv) reconcile() (err error) {
	// delete all tikvs which has not yet been registered to tidb cluster
	if tk.ReadyReplicas > tk.AvailableReplicas {
		for k, s := range tk.Stores {
			if s.ID < 1 {
				if err = k8sutil.DeletePod(k, terminationGracePeriodSeconds); err != nil {
					return err
				}
				delete(tk.Stores, k)
				tk.ReadyReplicas--
				logs.Warn("delete not started tikv %s", k)
			}
		}
	}

	// delete buried tikv
	if tk.ReadyReplicas != tk.AvailableReplicas {
		if err = tk.deleteBuriedTikv(); err != nil {
			return err
		}
		if err = tk.tryDeleteDownTikv(); err != nil {
			return err
		}
	}
	tk.ReadyReplicas = len(tk.Stores)
	count := 0
	for _, s := range tk.Stores {
		if s.State == StoreOnline {
			count++
		}
	}
	tk.AvailableReplicas = count

	if tk.AvailableReplicas != tk.ReadyReplicas {
		logs.Warn("the tikvs %q status is inconsistent, ready: %d, available: %d",
			tk.Db.GetName(), tk.ReadyReplicas, tk.AvailableReplicas)
		return
	}

	return
}

// delete store that status is down
func (tk *Tikv) tryDeleteDownTikv() error {
	for name, s := range tk.Stores {
		if s.State == StoreOnline {
			continue
		}
		sn, hb, err := tk.getStoreStateName(s)
		if err != nil {
			return err
		}
		elapsed := time.Now().Unix() - hb.Unix()
		// delete pod if the downtime is more than 1 hour
		if sn == "Down" && elapsed > tikvMaxDowntime {
			logs.Warn("delete the store %q which over downtime", name)
			if err = pdutil.PdStoreDelete(tk.Db.Pd.OuterAddresses[0], s.ID); err != nil {
				return err
			}
			if err = k8sutil.DeletePod(name, terminationGracePeriodSeconds); err != nil {
				return err
			}
			tk.ReadyReplicas--
			delete(tk.Stores, name)
		}
	}
	return nil
}

// get store state_name and last_heartbeat_ts
func (tk *Tikv) getStoreStateName(s *Store) (string, *time.Time, error) {
	j, err := pdutil.PdStoreIDGet(tk.Db.Pd.OuterAddresses[0], s.ID)
	if err != nil {
		if err == httputil.ErrNotFound {
			logs.Warn("can't get store:%d", s.ID)
			return "", nil, nil
		}
		return "", nil, err
	}
	ret := gjson.Get(j, "store.state_name")
	if ret.Type == gjson.Null {
		return "", nil, fmt.Errorf("cannt get store[%s] state name", s.Name)
	}
	hb := gjson.Get(j, "status.last_heartbeat_ts")
	if ret.Type == gjson.Null {
		return "", nil, fmt.Errorf("cannt get store[%s] last_heartbeat_ts", s.Name)
	}
	t, err := time.Parse(time.RFC3339Nano, hb.String())
	if err != nil {
		return "", nil, err
	}
	return ret.String(), &t, nil
}

func (tk *Tikv) getSortedStoresKey() []string {
	var keys []string
	for k := range tk.Stores {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func (tk *Tikv) checkScale(replicas int) error {
	md := getNonNullMetadata()
	c := replicas - tk.Replicas
	if c > 0 {
		if (replicas + tk.Replicas) > md.Spec.Tikv.Max {
			return fmt.Errorf("the replicas of tikv exceeds max %d", md.Spec.Tikv.Max)
		}
		if replicas > tk.Spec.Replicas*2 {
			return fmt.Errorf("each scale up can not exceed 2 times")
		}
	} else if c < 0 {
		if (tk.Spec.Replicas - replicas) < 3 {
			return fmt.Errorf("the replicas of tikv must more than %d", 3)
		}
		if replicas*3 > tk.Spec.Replicas {
			return fmt.Errorf("each scale dowm can not be less than one-third")
		}
	}
	return nil
}
