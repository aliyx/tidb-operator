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

	e := NewEvent(tk.Db.GetName(), "tidb/tikv", "upgrate")
	defer func() {
		// have upgrade
		if count > 0 || err != nil {
			e.Trace(err, fmt.Sprintf("Upgrate tikv to version: %s", tk.Version))
		}
	}()

	if tk.Db.Status.Phase < PhaseTikvStarted {
		err = ErrUnavailable
	}

	names := tk.getStoresKey()
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
			time.Sleep(upgradeInterval)
		}
	}
	return nil
}

func (tk *Tikv) install() (err error) {
	e := NewEvent(tk.Db.GetName(), "tidb/tikv", "install")
	defer func() {
		ph := PhaseTikvStarted
		if err != nil {
			ph = PhaseTikvStartFailed
		}
		tk.Db.Status.Phase = ph
		e.Trace(err,
			fmt.Sprintf("Install tikv pods with replicas desire: %d, running: %d on k8s",
				tk.Spec.Replicas, tk.AvailableReplicas))
	}()

	// savepoint for page show
	tk.Db.Status.Phase = PhaseTikvPending
	if err = tk.Db.update(); err != nil {
		return err
	}

	tk.Stores = make(map[string]*Store)
	for r := 1; r <= tk.Spec.Replicas; r++ {
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
		b, err := tk.isBuried(s)
		if err != nil {
			return err
		}
		if b {
			if err = k8sutil.DeletePods(name); err != nil {
				return err
			}
			tk.ReadyReplicas--
			delete(tk.Stores, name)
		}
	}
	return nil
}

func (tk *Tikv) isBuried(s *Store) (bool, error) {
	j, err := pdutil.PdStoreIDGet(tk.Db.Pd.OuterAddresses[0], s.ID)
	if err != nil {
		if err == httputil.ErrNotFound {
			logs.Warn("can't get store:%d", s.ID)
			return true, nil
		}
		return false, err
	}
	ret := gjson.Get(j, "store.state")
	if ret.Type == gjson.Null {
		return false, fmt.Errorf("cannt get store[%s] state", s.Name)
	}
	return StoreStatus(ret.Int()) == StoreTombstone, nil
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

func (db *Db) reconcileTikvs(replicas int) error {
	if replicas < 1 {
		return nil
	}

	var (
		err     error
		kv      = db.Tikv
		op      = "scale"
		changed = true
	)

	if kv.Replicas == replicas {
		op = "reconcile"
	}
	e := NewEvent(db.GetName(), "tidb/tikv", op)
	defer func(a, r int) {
		if err != nil {
			db.Status.ScaleState |= tikvScaleErr
		}
		if changed {
			if op == "scale" {
				e.Trace(err, fmt.Sprintf("Scale tikv replicas from %d to %d", r, replicas))
			} else {
				e.Trace(err, fmt.Sprintf("Reconcile tikv replicas from %d to %d", a, replicas))
			}
		}
	}(kv.AvailableReplicas, kv.Replicas)

	// check available replica

	if replicas == kv.AvailableReplicas {
		if err = kv.checkStoresStatus(); err != nil {
			logs.Error("check tikv %q stores status: %v", db.GetName(), err)
			return err
		}
	}
	if replicas == kv.AvailableReplicas && replicas == kv.ReadyReplicas {
		changed = false
		return nil
	}

	switch n := replicas - kv.Replicas; {
	case n > 0:
		err = kv.increase(n)
	case n < 0:
		err = kv.decrease(-n)
	default:
		err = kv.reconcile()
	}

	return err
}

// mark offline store
func (tk *Tikv) checkStoresStatus() error {
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

	// get all online tikvs
	ret = gjson.Get(j, fmt.Sprintf("stores.#[store.state==%d]#.store.id", StoreOnline))
	if ret.Type == gjson.Null {
		return nil
	}
	for _, s := range tk.Stores {
		online := false
		for _, sid := range ret.Array() {
			id := int(sid.Int())
			if s.ID == id {
				online = true
				break
			}
		}
		if !online {
			logs.Warn("mark store %q offline", s.Name)
			s.State = StoreOffline
			tk.AvailableReplicas--
		}
	}
	return nil
}

// Only mark store status is 'offline' and decrease AvailableReplicas
func (tk *Tikv) decrease(replicas int) (err error) {
	if (tk.Spec.Replicas - replicas) < 3 {
		return fmt.Errorf("the replicas of tikv must more than %d", 3)
	}
	if replicas*3 > tk.Spec.Replicas {
		return fmt.Errorf("each scale dowm can not be less than one-third")
	}

	logs.Info("start scaling down tikv %q pods count from %d to %d",
		tk.Db.GetName(), tk.Replicas, (tk.Replicas - replicas))

	tk.Replicas -= replicas

	names := tk.getStoresKey()
	for i := 0; i < replicas; i++ {
		name := names[i]
		if err = pdutil.PdStoreDelete(tk.Db.Pd.OuterAddresses[0], tk.Stores[name].ID); err != nil {
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
	md := getNonNullMetadata()
	if (replicas + tk.Replicas) > md.Spec.Tikv.Max {
		return fmt.Errorf("the replicas of tikv exceeds max %d", md.Spec.Tikv.Max)
	}
	if replicas > tk.Spec.Replicas*2 {
		return fmt.Errorf("each scale up can not exceed 2 times")
	}

	logs.Info("start scaling up tikv %q pods count: %d", tk.Db.GetName(), replicas)

	tk.Replicas += replicas
	for i := 0; i < replicas; i++ {
		tk.Member++
		if err = tk._install(); err != nil {
			return err
		}
	}

	logs.Info("end scale up tikv %q pod desire: %d, ready: %d, available: %d",
		tk.Db.GetName(), tk.Replicas, tk.ReadyReplicas, tk.AvailableReplicas)
	return nil
}

// Reconcile current available tikvs with desired
// 1. Delete the tikv that has not yet been registered
// 2. Delete buried tikv
// 3. Reconcile desire tikv consistent
func (tk *Tikv) reconcile() (err error) {
	// delete all tikvs which has not yet been registered to tidb cluster
	if tk.ReadyReplicas != tk.AvailableReplicas {
		for k, s := range tk.Stores {
			if s.ID < 1 {
				if err = k8sutil.DeletePods(k); err != nil {
					return err
				}
				delete(tk.Stores, k)
				tk.ReadyReplicas--
				logs.Warn("delete no started tikv %s", k)
			}
		}
	}

	// delete buried tikv
	if tk.ReadyReplicas != tk.AvailableReplicas {
		if err = tk.deleteBuriedTikv(); err != nil {
			return err
		}
	}

	if tk.ReadyReplicas != len(tk.Stores) || tk.AvailableReplicas != len(tk.Stores) {
		logs.Warn("the current tikvs %q count is inconsistent", tk.Db.GetName())
		return
	}

	if tk.AvailableReplicas < tk.Replicas {
		tk.Replicas = tk.AvailableReplicas
		return tk.increase(tk.Replicas - tk.AvailableReplicas)
	} else if tk.AvailableReplicas > tk.Replicas {
		tk.Replicas = tk.AvailableReplicas
		return tk.decrease(tk.AvailableReplicas - tk.Replicas)
	}

	return nil
}

func (tk *Tikv) getStoresKey() []string {
	var keys []string
	for k := range tk.Stores {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
