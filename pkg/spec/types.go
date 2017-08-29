package spec

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	// CRDGroup all crd resources group
	CRDGroup = "tidb.ffan.com"
	// CRDKindMetadata CRD metadata kind
	CRDKindMetadata = "Metadata"
	// CRDKindTidb CRD tidb kind
	CRDKindTidb = "Tidb"
	// CRDKindEvent CRD event kind
	CRDKindEvent = "Event"
	// CRDVersion current version is beta for REST API: /apis/<group>/<version>
	CRDVersion = "v1beta2"
)

var (
	SchemeGroupVersion = schema.GroupVersion{Group: CRDGroup, Version: CRDVersion}
)

// Resource tpr
type Resource struct {
	metav1.TypeMeta `json:",inline"`
	Metadata        metav1.ObjectMeta `json:"metadata,omitempty"`
}
