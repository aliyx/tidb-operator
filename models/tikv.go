package models

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"strconv"

	"github.com/astaxie/beego/logs"
	"github.com/ffan/tidb-k8s/models/utils"
	"github.com/tidwall/gjson"
)

// Tikv 元数据存储模块
type Tikv struct {
	K8sInfo
	Volume   string `json:"tidbdata_volume"`
	Capatity int    `json:"capatity,omitempty"`

	Db *Tidb `json:"-"`

	cur    string
	Stores map[string]Store `json:"stores,omitempty"`
}

// Store tikv在tidb集群中的状态
type Store struct {
	// tikv info
	ID      int    `json:"s_id,omitempty"`
	Address string `json:"s_address,omitempty"`
	State   int    `json:"s_state,omitempty"`
}

// NewTikv return a Pd instance
func NewTikv() *Tikv {
	return &Tikv{}
}

// beforeSave 创建之前的检查工作
func (kv *Tikv) beforeSave() error {
	if err := kv.validate(); err != nil {
		return err
	}
	md, err := GetMetadata()
	if err != nil {
		return err
	}
	kv.Volume = strings.Trim(md.K8s.Volume, " ")
	if len(kv.Volume) == 0 {
		kv.Volume = "emptyDir: {}"
	} else {
		kv.Volume = fmt.Sprintf("hostPath: {path: %s}", kv.Volume)
	}
	if kv.Capatity < 1 {
		kv.Capatity = md.Units.Tikv.Capacity
	}
	return nil
}

func (kv *Tikv) validate() error {
	if err := kv.K8sInfo.validate(); err != nil {
		return err
	}
	md, _ := GetMetadata()
	max := md.Units.Tikv.Max
	if kv.Replicas < 3 || kv.Replicas > max {
		return fmt.Errorf("replicas must be >= 3 and <= %d", max)
	}
	return nil
}

func (kv *Tikv) update() error {
	return kv.Db.Update()
}

// GetTikv 获取tikv元数据
func GetTikv(cell string) (*Tikv, error) {
	db, err := GetTidb(cell)
	if err != nil {
		return nil, err
	}
	kv := db.Tikv
	kv.Db = db
	return kv, nil
}

// Run tikv
func (kv *Tikv) run() (err error) {
	e := NewEvent(kv.Db.Cell, "tikv", "start")
	kv.Stores = make(map[string]Store)
	defer func() {
		st := TikvStarted
		if err != nil {
			st = TikvStartFailed
		} else {
			kv.Ok = true
		}
		kv.Db.Status = st
		kv.Db.Update()
		e.Trace(err, fmt.Sprintf("Start tikv %d pods on k8s", kv.Replicas))
	}()
	for r := 1; r <= kv.Replicas; r++ {
		if err = kv._run(r); err != nil {
			return err
		}
	}
	return err
}

func (kv *Tikv) _run(r int) (err error) {
	// 先设置，防止tikv启动失败的情况下，没有保存tikv信息，导致delete时失败
	kv.cur = fmt.Sprintf("tikv-%s-%d", kv.Db.Cell, r)
	kv.Stores[kv.cur] = Store{}
	if err = createPod(kv.getK8sTemplate(k8sTikvPod, r)); err != nil {
		return err
	}
	if err = kv.waitForComplete(startTidbTimeout); err != nil {
		return err
	}
	return nil
}

// getK8sTemplate 生成k8s tikv template
func (kv *Tikv) getK8sTemplate(t string, id int) string {
	r := strings.NewReplacer(
		"{{version}}", kv.Version,
		"{{cpu}}", fmt.Sprintf("%v", kv.CPU),
		"{{mem}}", fmt.Sprintf("%v", kv.Mem),
		"{{capacity}}", fmt.Sprintf("%v", kv.Capatity),
		"{{tidbdata_volume}}", fmt.Sprintf("%v", kv.Volume),
		"{{id}}", fmt.Sprintf("%v", id),
		"{{registry}}", dockerRegistry,
		"{{cell}}", kv.Db.Cell,
		"{{namespace}}", getNamespace())
	s := r.Replace(t)
	return s
}

func (kv *Tikv) waitForComplete(wait time.Duration) error {
	if err := waitPodsRuning(wait, kv.cur); err != nil {
		return err
	}
	if err := utils.RetryIfErr(wait, func() error {
		j, err := pdStoresGet(kv.Db.Pd.Nets[0].String())
		if err != nil {
			return err
		}
		ret := gjson.Get(j, "count")
		if ret.Int() < 1 {
			return fmt.Errorf("no tikv cluster")
		}
		// 获取online的tikv
		ret = gjson.Get(j, "stores.#[store.state==0]#.store.id")
		logs.Debug("PdStores: %v", ret)
		if ret.Type == gjson.Null {
			return fmt.Errorf("no online tikv cluster")
		}
		for _, id := range ret.Array() {
			var have bool
			storeID := id.Int()
			if storeID < 1 {
				// 未知错误
				logs.Warn("Invalid store id: %d", storeID)
				continue
			}
			for _, s := range kv.Stores {
				if int(storeID) == s.ID {
					have = true
				}
			}
			if !have {
				kv.Stores[kv.cur] = Store{
					ID: int(id.Int()),
				}
			}
		}
		for _, st := range kv.Stores {
			if st.ID < 1 {
				return fmt.Errorf("%s no started", kv.cur)
			}
		}
		return nil
	}); err != nil {
		return fmt.Errorf(`start up tikvs timout`)
	}
	return nil
}

func (kv *Tikv) stop() (err error) {
	cell := kv.Db.Cell
	e := NewEvent(cell, "tikv", "stop")
	defer func() {
		st := TikvStoped
		if err != nil {
			st = TikvStopFailed
		}
		kv.Ok = false
		kv.Stores = nil
		e.Trace(err, fmt.Sprintf("Stop tikv %d pods", kv.Replicas))
		kv.update()
		rollout(cell, st)
	}()
	if len(kv.Stores) > 0 {
		if err := delPodsBy(cell, "tikv"); err != nil {
			return err
		}
	} else {
		logs.Error(`No pods "tikv-%s-*", if it exists, please delete it manually`, cell)
	}
	return err
}

// ScaleTikvs 扩容tikv模块,目前replicas只能增减不能减少
func ScaleTikvs(replicas int, cell string) error {
	kv, err := GetTikv(cell)
	if err != nil || kv == nil || !kv.Ok {
		return fmt.Errorf("module tikv not started: %v", err)
	}
	e := NewEvent(cell, "tikv", "scale")
	defer func() {
		e.Trace(err, fmt.Sprintf(`Scale tikv "%s" from %d to %d`, cell, kv.Replicas, replicas))
	}()
	switch n := replicas - kv.Replicas; {
	case n > 0:
		err = kv.increase(replicas)
	case n < 0:
		err = kv.decrease(replicas)
	default:
		return nil
	}
	if uerr := kv.update(); uerr != nil || err != nil {
		return fmt.Errorf("%v\n%v", err, uerr)
	}
	return nil
}

func (kv *Tikv) decrease(replicas int) error {
	return fmt.Errorf("current unsupport for reducing the number of tikvs src:%d desc:%d", kv.Replicas, replicas)
}

func (kv *Tikv) increase(replicas int) (err error) {
	md, _ := GetMetadata()
	if replicas > md.Units.Tikv.Max {
		return fmt.Errorf("the replicas of tikv exceeds max %d", md.Units.Tikv.Max)
	}
	if replicas > kv.Replicas*3 {
		return fmt.Errorf("each expansion can not exceed 1 times")
	}
	keys := getMapSortedKeys(kv.Stores)
	max, _ := strconv.Atoi(strings.Split(keys[len(keys)-1], "-")[2])
	logs.Debug("max:%d src:%d desc:%d", max, kv.Replicas, replicas)
	for i := max + 1; i <= max+(replicas-kv.Replicas); i++ {
		kv.Replicas = kv.Replicas + 1
		if err = kv._run(i); err != nil {
			return err
		}
	}
	return err
}

// getMapSortedKeys 获取map被排序之后的keys
func getMapSortedKeys(m map[string]Store) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func (kv *Tikv) isNil() bool {
	return kv.Replicas < 1
}
