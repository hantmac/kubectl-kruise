package client

import (
	kruiseappsv1alpha1 "github.com/openkruise/kruise-api/apps/v1alpha1"
	"k8s.io/kubectl/pkg/scheme"
	"sync"

	"github.com/pkg/errors"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
)

var managerOnce sync.Once
var mgr ctrl.Manager
var Scheme = scheme.Scheme

func init() {
	_ = clientgoscheme.AddToScheme(Scheme)
	_ = kruiseappsv1alpha1.AddToScheme(Scheme)
}

func NewManager() ctrl.Manager {
	managerOnce.Do(func() {
		var err error
		mgr, err = ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
			Scheme:             Scheme,
			MetricsBindAddress: "0",
		})
		if err != nil {
			panic(errors.Wrap(err, "unable to start manager"))
		}
	})
	return mgr
}
