package rollout

import (
	kruiseappsv1alpha1 "github.com/openkruise/kruise-api/apps/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
)

var Scheme = runtime.NewScheme()

func init() {
	_ = kruiseappsv1alpha1.AddToScheme(Scheme)
	_ = clientgoscheme.AddToScheme(Scheme)
}
