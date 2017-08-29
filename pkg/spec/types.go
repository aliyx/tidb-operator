package spec

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

const (
	// crdGroup all crd resources group
	crdGroup = "tidb.ffan.com"
	// crdVersion current version is beta for REST API: /apis/<group>/<version>
	crdVersion = "v1beta2"
	// CRDKindMetadata CRD metadata kind
	CRDKindMetadata = "Metadata"
	// CRDKindTidb CRD tidb kind
	CRDKindTidb = "Tidb"
	// CRDKindEvent CRD event kind
	CRDKindEvent = "Event"
)

var (
	SchemeGroupVersion = schema.GroupVersion{Group: crdGroup, Version: crdVersion}
)

// Resource tpr
type Resource struct {
	metav1.TypeMeta `json:",inline"`
	Metadata        metav1.ObjectMeta `json:"metadata,omitempty"`
}
