package spec

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	// TPRKindMetadata metadata TPR schema
	TPRKindMetadata = "Metadata"
	// TPRKindTidb tidb TPR schema
	TPRKindTidb = "Tidb"
	// TPRKindEvent event TPR schema
	TPRKindEvent = "Event"
	// TPRGroup all resources group
	TPRGroup = "tidb.ffan.com"
	// TPRVersion current version is beta
	TPRVersion = "v1beta1"
	// TPRDescription a trp desc
	TPRDescription = "Manage tidb cluster"
	// APIVersion tpr api version
	APIVersion = TPRGroup + "/" + TPRVersion
)

// Resource tpr
type Resource struct {
	metav1.TypeMeta `json:",inline"`
	Metadata        metav1.ObjectMeta `json:"metadata,omitempty"`
}

var (
	// SchemeGroupVersion all tpr schema group
	SchemeGroupVersion = schema.GroupVersion{
		Group:   TPRGroup,
		Version: TPRVersion,
	}
)
