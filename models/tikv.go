package models

import (
	"fmt"
	"strings"
	"sync"

	"k8s.io/client-go/pkg/api/v1"

	"time"

	"errors"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-k8s/pkg/k8sutil"
	"github.com/ffan/tidb-k8s/pkg/retryutil"
	"github.com/ghodss/yaml"
	"github.com/tidwall/gjson"
)

const (
	defaultTikvPort = 20160
)

var (
	errMultipleStoresOneAddress = errors.New("multiple stores one address")
)

// Tikv 元数据存储模块
type Tikv struct {
	Spec              Spec              `json:"spec"`
	Member            int               `json:"member"`
	ReadyReplicas     int               `json:"readyReplicas"`
	AvailableReplicas int               `json:"availableReplicas"`
	Stores            map[string]*Store `json:"stores,omitempty"`

	cur string
	Db  *Tidb `json:"-"`
}

// Store tikv在tidb集群中的状态
type Store struct {
	// tikv info
	ID      int    `json:"id,omitempty"`
	Address string `json:"address,omitempty"`
	Node    string `json:"nodeName,omitempty"`
	State   int    `json:"state,omitempty"`
}

func (kv *Tikv) beforeSave() error {
	if err := kv.Spec.validate(); err != nil {
		return err
	}
	md := getCachedMetadata()
	max := md.Units.Tikv.Max
	if kv.Spec.Replicas < 3 || kv.Spec.Replicas > max {
		return fmt.Errorf("replicas must be >= 3 and <= %d", max)
	}
	kv.Spec.Volume = strings.Trim(md.K8s.Volume, " ")
	if len(kv.Spec.Volume) == 0 {
		kv.Spec.Volume = "emptyDir: {}"
	} else {
		kv.Spec.Volume = fmt.Sprintf("hostPath: {path: %s}", kv.Spec.Volume)
	}
	if kv.Spec.Capatity < 1 {
		kv.Spec.Capatity = md.Units.Tikv.Capacity
	}
	return nil
}

// GetTikv get a tikv instance
func GetTikv(cell string) (*Tikv, error) {
	db, err := GetTidb(cell)
	if err != nil {
		return nil, err
	}
	kv := db.Tikv
	kv.Db = db
	return kv, nil
}

func (kv *Tikv) install() (err error) {
	e := NewEvent(kv.Db.Cell, "tikv", "install")
	kv.Db.Status.Phase = tikvPending
	kv.Db.update()
	kv.Stores = make(map[string]*Store)
	defer func() {
		ph := tikvStarted
		if err != nil {
			ph = tikvStartFailed
		}
		kv.Db.Status.Phase = ph
		if uerr := kv.Db.update(); uerr != nil {
			logs.Error("update tidb error: %v", uerr)
		}
		e.Trace(err, fmt.Sprintf("Install tikv pods with replicas desire: %d, running: %d on k8s", kv.Spec.Replicas, kv.AvailableReplicas))
	}()
	for r := 1; r <= kv.Spec.Replicas; r++ {
		kv.Member++
		if err = kv._install(); err != nil {
			return err
		}
	}
	return err
}

func (kv *Tikv) _install() (err error) {
	kv.cur = fmt.Sprintf("tikv-%s-%03d", kv.Db.Cell, kv.Member)
	kv.Stores[kv.cur] = &Store{}
	kv.ReadyReplicas++
	if err = kv.createPod(); err != nil {
		return err
	}
	if err = kv.waitForOk(); err != nil {
		return err
	}
	kv.AvailableReplicas++
	return nil
}

func (kv *Tikv) createPod() (err error) {
	var j []byte
	if j, err = kv.toJSONTemplate(tikvPodYaml); err != nil {
		return err
	}
	var pod *v1.Pod
	if pod, err = k8sutil.CreateAndWaitPodByJSON(j, waitPodRuningTimeout); err != nil {
		return err
	}
	s := kv.Stores[kv.cur]
	s.Address = fmt.Sprintf("%s:%d", pod.Status.PodIP, defaultTikvPort)
	s.Node = pod.Spec.NodeName
	return nil
}

func (kv *Tikv) toJSONTemplate(temp string) ([]byte, error) {
	r := strings.NewReplacer(
		"{{version}}", kv.Spec.Version,
		"{{cpu}}", fmt.Sprintf("%v", kv.Spec.CPU),
		"{{mem}}", fmt.Sprintf("%v", kv.Spec.Mem),
		"{{capacity}}", fmt.Sprintf("%v", kv.Spec.Capatity),
		"{{tidbdata_volume}}", fmt.Sprintf("%v", kv.Spec.Volume),
		"{{id}}", fmt.Sprintf("%03v", kv.Member),
		"{{registry}}", imageRegistry,
		"{{cell}}", kv.Db.Cell,
		"{{namespace}}", getNamespace())
	str := r.Replace(temp)
	j, err := yaml.YAMLToJSON([]byte(str))
	if err != nil {
		return nil, err
	}
	return j, nil
}

func (kv *Tikv) waitForOk() (err error) {
	logs.Info("waiting for run tikv %s ok...", kv.cur)
	interval := 3 * time.Second
	err = retryutil.Retry(interval, int(waitTidbComponentAvailableTimeout/(interval)), func() (bool, error) {
		j, err := pdStoresGet(kv.Db.Pd.OuterAddresses[0])
		if err != nil {
			logs.Error("get stores by pd API: %v", err)
			return false, nil
		}
		ret := gjson.Get(j, "count")
		if ret.Int() < 1 {
			logs.Warn("current stores count: %d", ret.Int())
			return false, nil
		}
		// 获取online的tikv
		s := kv.Stores[kv.cur]
		ret = gjson.Get(j, fmt.Sprintf("stores.#[store.address==%s]#.store.id", s.Address))
		if ret.Type == gjson.Null {
			logs.Warn("cannt get store[%s]", kv.Stores[kv.cur].Address)
			return false, nil
		}
		if len(ret.Array()) != 1 {
			logs.Error("get multiple store by address[%s]", kv.Stores[kv.cur].Address)
			return false, errMultipleStoresOneAddress
		}
		s.ID = int(ret.Array()[0].Int())
		s.State = 0
		return true, nil
	})
	if err != nil {
		logs.Error("wait tikv %s available: %v", kv.cur, err)
	} else {
		logs.Info("tikv %s ok", kv.cur)
	}
	return err
}

func (kv *Tikv) uninstall() (err error) {
	cell := kv.Db.Cell
	e := NewEvent(cell, "tikv", "uninstall")
	defer func() {
		kv.Stores = nil
		kv.Member = 0
		kv.cur = ""
		kv.AvailableReplicas = 0
		kv.ReadyReplicas = 0
		if uerr := kv.Db.update(); uerr != nil {
			logs.Error("update tidb error: %v", uerr)
		}
		e.Trace(err, fmt.Sprintf("Uninstall tikv %d pods", kv.Spec.Replicas))
	}()
	if err := k8sutil.DeletePodsBy(cell, "tikv"); err != nil {
		return err
	}
	return err
}

func (db *Tidb) scaleTikvs(replica int, wg *sync.WaitGroup) {
	if replica < 1 {
		return
	}
	kv := db.Tikv
	if replica == kv.Spec.Replicas {
		return
	}
	wg.Add(1)
	go func() {
		db.Lock()
		defer func() {
			db.Unlock()
			wg.Done()
		}()
		var err error
		e := NewEvent(db.Cell, "tikv", "scale")
		defer func(r int) {
			if err != nil {
				db.Status.ScaleState |= tikvScaleErr
			}
			db.update()
			e.Trace(err, fmt.Sprintf(`Scale tikv "%s" replica: %d->%d`, db.Cell, r, replica))
		}(kv.Spec.Replicas)
		switch n := replica - kv.Spec.Replicas; {
		case n > 0:
			err = kv.increase(n)
		case n < 0:
			err = kv.decrease(-n)
		}
	}()
}

func (kv *Tikv) decrease(replicas int) error {
	return fmt.Errorf("current unsupport for reducing the number of tikvs src:%d desc:%d", kv.Spec.Replicas, replicas)
}

func (kv *Tikv) increase(replicas int) (err error) {
	md := getCachedMetadata()
	if (replicas + kv.Spec.Replicas) > md.Units.Tikv.Max {
		return fmt.Errorf("the replicas of tikv exceeds max %d", md.Units.Tikv.Max)
	}
	if replicas > kv.Spec.Replicas*3 || kv.Spec.Replicas > replicas*3 {
		return fmt.Errorf("each scale can not exceed 2 times")
	}
	logs.Info("start incrementally scale tikv pod count: %d", replicas)
	for i := 0; i <= replicas; i++ {
		kv.Member++
		if err = kv._install(); err != nil {
			return err
		}
	}
	logs.Info("end incrementally scale tikv %s pod desire: %d, available: %d", kv.Db.Cell, kv.Spec.Replicas, kv.AvailableReplicas)
	return err
}

func (kv *Tikv) isNil() bool {
	return kv.Spec.Replicas < 1
}
