package operator

import (
	"fmt"
	"strings"

	"time"

	"errors"

	"sort"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-operator/pkg/util/httputil"
	"github.com/ffan/tidb-operator/pkg/util/k8sutil"
	"github.com/ffan/tidb-operator/pkg/util/pdutil"
	"github.com/ffan/tidb-operator/pkg/util/retryutil"
	"github.com/tidwall/gjson"
	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	defaultTikvPort = 20160

	// GB ~/pingcap/tikv/src/util/config.rs<GB>
	GB = 1024 * 1024 * 1024
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
		image    = fmt.Sprintf("%s/tikv:%s", ImageRegistry, tk.Version)
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
			time.Sleep(tikvUpgradeInterval)
			if err = tk.waitForStoreOk(); err != nil {
				return err
			}
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
	if err = tk.Db.patch(nil); err != nil {
		return err
	}

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
	if tk.Stores == nil {
		tk.Stores = make(map[string]*Store)
	}
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

const tikvCmd = `
p=$(mountpath "/host" {{mount}})
data_dir=$p/$HOSTNAME
echo "Current data dir:$data_dir"
if [ -d $data_dir ]; then
  echo "Resuming with existing data dir"
else
  echo "First run for this tikv"
fi
/tikv-server \
--store="$data_dir" \
--addr="0.0.0.0:20160" \
--capacity={{capacity}} \
--advertise-addr="$POD_IP:20160" \
--pd="pd-{{cell}}:2379" \
--config="/etc/tikv/config.toml"
`

func (tk *Tikv) createPod() (err error) {
	r := strings.NewReplacer(
		"{{capacity}}", fmt.Sprintf("%d", tk.Spec.Capatity*GB),
		"{{cell}}", tk.Db.GetName(),
		"{{mount}}", tk.Spec.Mount)
	cmd := r.Replace(tikvCmd)

	pod := &v1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:   fmt.Sprintf("tikv-%s-%03v", tk.Db.GetName(), tk.Member),
			Labels: tk.Db.getLabels("tikv"),
		},
		Spec: v1.PodSpec{
			TerminationGracePeriodSeconds: GetTerminationGracePeriodSeconds(),
			RestartPolicy:                 v1.RestartPolicyNever,
			Containers: []v1.Container{
				v1.Container{
					Name:            "tikv",
					Image:           ImageRegistry + "/tikv:" + tk.Version,
					ImagePullPolicy: v1.PullAlways,
					Ports:           []v1.ContainerPort{{ContainerPort: 20160}},
					VolumeMounts: []v1.VolumeMount{
						{Name: "datadir", MountPath: "/host"},
					},
					Resources: v1.ResourceRequirements{
						Limits: k8sutil.MakeResourceList(tk.CPU, tk.Mem),
					},
					Env: []v1.EnvVar{
						k8sutil.MakeTZEnvVar(),
						k8sutil.MakePodIPEnvVar(),
					},
					Command: []string{
						"bash", "-c", cmd,
					},
				},
			},
		},
	}

	// set volume

	if len(tk.Volume) == 0 {
		pod.Spec.Volumes = append(pod.Spec.Volumes, k8sutil.MakeEmptyDirVolume("datadir"))
	} else {
		pod.Spec.Volumes = append(pod.Spec.Volumes, v1.Volume{
			Name: "datadir",
			VolumeSource: v1.VolumeSource{
				HostPath: &v1.HostPathVolumeSource{
					Path: tk.Volume,
				},
			},
		})
	}

	// save image version
	k8sutil.SetTidbVersion(pod, tk.Version)

	// PD and TiKV instances, it is recommended that each instance individually deploy a hard disk
	// to avoid IO conflicts and affect performance
	pod.Spec.Affinity = &v1.Affinity{
		PodAntiAffinity: &v1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []v1.WeightedPodAffinityTerm{
				v1.WeightedPodAffinityTerm{
					Weight: 80,
					PodAffinityTerm: v1.PodAffinityTerm{
						LabelSelector: &metav1.LabelSelector{
							MatchExpressions: []metav1.LabelSelectorRequirement{
								metav1.LabelSelectorRequirement{
									Key:      "component",
									Operator: metav1.LabelSelectorOpIn,
									Values:   []string{"pd"},
								},
							},
						},
						TopologyKey: "kubernetes.io/hostname",
					},
				},
			},
		},
	}

	if pod, err = k8sutil.CreateAndWaitPod(pod, waitPodRuningTimeout); err != nil {
		return err
	}

	s := tk.Stores[tk.cur]
	s.Name = tk.cur
	s.Address = fmt.Sprintf("%s:%d", pod.Status.PodIP, defaultTikvPort)
	s.Node = pod.Spec.NodeName
	return nil
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
		logs.Info("start reconciling tikv %q cluster", db.GetName())
		err = kv.reconcile()
		logs.Info("end reconcile tikv %q cluster", db.GetName())
	}
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
		logs.Error("current db %q is unavailable, stores count: 0", tk.Db.GetName())
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
	if len(ret.Array()) < 1 {
		logs.Warn(
			"could not get up stores, maybe pd %q cluster has just gone through a leader switch or other reasons",
			tk.Db.GetName())
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
				if s.DownTimes >= tikvAllowMaximumDowntimes {
					logs.Warn("mark tikv %q offline that over max downtimes: %d", name, tikvAllowMaximumDowntimes)
					s.State = StoreOffline
					tk.AvailableReplicas--
				} else {
					s.DownTimes++
					logs.Warn("mark tikv %q down times: %d", name, s.DownTimes)
				}
			}
		} else {
			// reset down times to zero
			if s.DownTimes > 0 {
				logs.Info("reover store %q status from down to up", name)
				s.DownTimes = 0
			}
			if s.State == StoreOffline {
				// may be down -> up
				s.State = StoreOnline
				tk.AvailableReplicas++
			}
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
		logs.Warn("the tikv %q cluster status is inconsistent, ready replicas: %d, available replicas: %d",
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

		del := false
		// Delete the pod that does not exist to prevent ip conflict with new pod
		_, err = k8sutil.GetPod(name)
		if apierrors.IsNotFound(err) {
			del = true
			logs.Warn("delete the store %q that does not exist in k8s", name)
		}

		// delete pod if the downtime is more than max downtime
		if !del {
			elapsed := time.Now().Unix() - hb.Unix()
			if sn == "Down" && elapsed > tikvMaxDowntime {
				logs.Warn("delete the store %q which over max downtime %ds", name, tikvMaxDowntime)
				del = true
			}
		}
		if del {
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
		if (replicas + tk.Replicas) > md.Tikv.Max {
			return fmt.Errorf("the replicas of tikv exceeds max %d", md.Tikv.Max)
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
