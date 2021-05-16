package client

import (
	"sync"

	kruiseappsv1alpha1 "github.com/openkruise/kruise-api/apps/v1alpha1"
	"github.com/pkg/errors"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
)

var managerOnce sync.Once
var mgr ctrl.Manager
var scheme = runtime.NewScheme()

func init() {
	_ = clientgoscheme.AddToScheme(scheme)
	_ = kruiseappsv1alpha1.AddToScheme(scheme)
}

func NewManager() ctrl.Manager {
	managerOnce.Do(func() {
		var err error
		mgr, err = ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
			Scheme:             scheme,
			MetricsBindAddress: "0",
		})
		if err != nil {
			panic(errors.Wrap(err, "unable to start manager"))
		}
	})
	return mgr
}
