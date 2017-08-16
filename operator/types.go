package operator

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// StoreStatus tikv store status
type StoreStatus int

const (
	// StoreOnline the store is available
	StoreOnline StoreStatus = iota
	// StoreOffline mark the store offline, but what will not be deleted
	StoreOffline
	// StoreTombstone the store'pod will be deleted
	StoreTombstone
	// StoreUnknown maybe start failure, etc
	StoreUnknown
)

const (
	// PodRunning pod status is runnng
	PodRunning = iota
	// PodFailed pod status is failed
	PodFailed
)

const (
	upgrading     = "Upgrading"
	upgradeOk     = "True"
	upgradeFailed = "False"
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
	metav1.TypeMeta `json:",inline"`
	Metadata        metav1.ObjectMeta `json:"metadata,omitempty"`

	Owner    Owner  `json:"owner,omitempty"`
	Schema   Schema `json:"schema"`
	Pd       *Pd    `json:"pd"`
	Tikv     *Tikv  `json:"tikv"`
	Tidb     *Tidb  `json:"tidb"`
	Operator string `json:"operator"`
	Status   Status `json:"status"`
	Volume   string `json:"tidbdata_volume,omitempty"`
}

// Tidb tidb module
type Tidb struct {
	Spec    `json:",inline"`
	Members []*Member `json:"members,omitempty"`
	// number of available tidb
	AvailableReplicas int `json:"availableReplicas"`

	cur string
	Db  *Db `json:"-"`
}

// Owner creater
type Owner struct {
	ID     string `json:"userId"` //user
	Name   string `json:"userName"`
	Desc   string `json:"desc"`
	Reason string `json:"reason"`
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
	Mount    string `json:"mount,omitempty"`
	Capatity int    `json:"capatity,omitempty"`
}

// Status tidb status
type Status struct {
	Available bool   `json:"available"`
	Phase     Phase  `json:"phase"`
	Reason    string `json:"reason"`

	MigrateState      string `json:"migrateState"`
	MigrateRetryCount int    `json:"migrateRetryCount,omitempty"`

	UpgradeState string `json:"upgradeState"`
	ScaleState   int    `json:"scaleState"`
	ScaleCount   int    `json:"scaleCount"`
	Message      string `json:"message"`

	OuterAddresses       []string `json:"outerAddresses,omitempty"`
	OuterStatusAddresses []string `json:"outerStatusAddresses,omitempty"`
}

// Phase tidb runing status
type Phase int

// Pd describe a pd cluster
type Pd struct {
	Spec `json:",inline"`

	InnerAddresses []string `json:"innerAddresses,omitempty"`
	OuterAddresses []string `json:"outerAddresses,omitempty"`

	Member  int       `json:"member"`
	Members []*Member `json:"members,omitempty"`

	// default is new
	initialClusterState string
	// true: join exist cluser, false:  init cluster
	join bool
	Db   *Db `json:"-"`
}

// Member describe a pd or tikv pod
type Member struct {
	Name  string `json:"name,omitempty"`
	State int    `json:"state,omitempty"`
}

// Tikv describe a tikv cluster
type Tikv struct {
	Spec   `json:",inline"`
	Member int `json:"member"`
	// number of tikv pods
	ReadyReplicas int `json:"readyReplicas"`
	// all tikvs
	Stores map[string]*Store `json:"stores,omitempty"`
	// number of available tikv
	AvailableReplicas int `json:"availableReplicas"`

	cur string
	Db  *Db `json:"-"`
}

// Store a tikv
type Store struct {
	ID        int         `json:"id,omitempty"`
	Name      string      `json:"name,omitempty"`
	Address   string      `json:"address,omitempty"`
	Node      string      `json:"nodeName,omitempty"`
	State     StoreStatus `json:"state"`
	DownTimes int         `json:"downTimes"`
}

const (
	// PhaseRefuse user apply create a tidb
	PhaseRefuse Phase = iota - 2
	// PhaseAuditing wait admin to auditing user apply
	PhaseAuditing
	// PhaseUndefined undefined
	PhaseUndefined
	// PhasePdPending pd pods is starting
	PhasePdPending
	// PhasePdStartFailed fail to start all pod pods
	PhasePdStartFailed
	// PhasePdStarted pd pods started
	PhasePdStarted
	// PhaseTikvPending tikv pods is starting
	PhaseTikvPending
	// PhaseTikvStartFailed fail to start all tikv pods
	PhaseTikvStartFailed
	// PhaseTikvStarted tikv pods started
	PhaseTikvStarted
	// PhaseTidbPending tidb pods is starting
	PhaseTidbPending
	// PhaseTidbStartFailed fail to start all tidb pods
	PhaseTidbStartFailed
	// PhaseTidbStarted tidb pods started
	PhaseTidbStarted
	// PhaseTidbInitFailed fail to init tidb schema and privilage
	PhaseTidbInitFailed
	// PhaseTidbInited tidb aviliable
	PhaseTidbInited
	// PhaseTidbUninstalling being uninstall tidb
	PhaseTidbUninstalling
)

// GetName ...
func (db *Db) GetName() string {
	return db.Metadata.Name
}
