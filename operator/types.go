package operator

import (
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TidbList is a list of tidb clusters.
type TidbList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list metadata
	// More info: http://releases.k8s.io/HEAD/docs/devel/api-conventions.md#metadata
	Metadata metav1.ListMeta `json:"metadata,omitempty"`
	// Items is a list of third party objects
	Items []Db `json:"items"`
}

// Db tidb metadata
type Db struct {
	*sync.Mutex // not copy

	metav1.TypeMeta `json:",inline"`
	Metadata        metav1.ObjectMeta `json:"metadata,omitempty"`

	Owner  *Owner `json:"owner,omitempty"`
	Schema Schema `json:"schema"`
	Pd     *Pd    `json:"pd"`
	Tikv   *Tikv  `json:"tikv"`
	Tidb   *Tidb  `json:"tidb"`
	Status Status `json:"status"`
}

// Tidb tidb module
type Tidb struct {
	Spec `json:",inline"`

	Db *Db `json:"-"`
}

// Owner creater
type Owner struct {
	ID     string `json:"userId"` //user
	Name   string `json:"userName"`
	Desc   string `json:"desc,omitempty"`
	Reason string `json:"reason,omitempty"`
}

// Schema database schema
type Schema struct {
	Name     string `json:"name"`
	User     string `json:"user"`
	Password string `json:"password"`
}

// Spec describe a pd/tikv/tidb specification
type Spec struct {
	CPU      int    `json:"cpu"`
	Mem      int    `json:"mem"`
	Version  string `json:"version"`
	Replicas int    `json:"replicas"`
	Volume   string `json:"tidbdata_volume,omitempty"`
	Capatity int    `json:"capatity,omitempty"`
}

// Status tidb status
type Status struct {
	Available    bool   `json:"available"`
	Phase        Phase  `json:"phase"`
	MigrateState string `json:"migrateState"`
	ScaleState   int    `json:"scaleState"`
	Desc         string `json:"desc,omitempty"`

	OuterAddresses       []string `json:"outerAddresses,omitempty"`
	OuterStatusAddresses []string `json:"outerStatusAddresses,omitempty"`
}

// Pd 元数据
type Pd struct {
	Spec `json:",inline"`

	InnerAddresses []string `json:"innerAddresses,omitempty"`
	OuterAddresses []string `json:"outerAddresses,omitempty"`

	Member int `json:"member"`
	// key is pod name
	Members map[string]Member `json:"members,omitempty"`

	Db *Db `json:"-"`
}

// Member describe a pd or tikv pod
type Member struct {
	ID    int `json:"id,omitempty"`
	State int `json:"state,omitempty"`
}

// Tikv 元数据存储模块
type Tikv struct {
	Spec              `json:",inline"`
	Member            int               `json:"member"`
	ReadyReplicas     int               `json:"readyReplicas"`
	AvailableReplicas int               `json:"availableReplicas"`
	Stores            map[string]*Store `json:"stores,omitempty"`

	cur string
	Db  *Db `json:"-"`
}

// Store tikv在tidb集群中的状态
type Store struct {
	// tikv info
	ID      int    `json:"id,omitempty"`
	Name    string `json:"name,omitempty"`
	Address string `json:"address,omitempty"`
	Node    string `json:"nodeName,omitempty"`
	State   int    `json:"state,omitempty"`
}
