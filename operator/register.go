package operator

import (
	"github.com/ffan/tidb-operator/pkg/spec"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var (
	schemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)

	// AddToScheme register type of Db and DbList to schema
	AddToScheme = schemeBuilder.AddToScheme
)

// addKnownTypes adds the set of types defined in this package to the supplied scheme.
func addKnownTypes(s *runtime.Scheme) error {
	s.AddKnownTypeWithName(spec.SchemeGroupVersion.WithKind("Tidb"), &Db{})
	s.AddKnownTypes(spec.SchemeGroupVersion,
		&DbList{},
	)
	metav1.AddToGroupVersion(s, spec.SchemeGroupVersion)
	return nil
}
