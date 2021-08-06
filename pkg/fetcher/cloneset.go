package fetcher

import (
	"context"
	"fmt"
	kruiseappsv1alpha1 "github.com/openkruise/kruise-api/apps/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/klog/v2"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func GetResourceInCache(ns, name string, obj runtime.Object, cl client.Reader) (bool, error) {
	err := cl.Get(context.TODO(), types.NamespacedName{Namespace: ns, Name: name}, obj)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

func ListResourceInCache(ns string, obj runtime.Object, cl client.Reader, ls labels.Selector) error {
	return cl.List(context.TODO(), obj, client.InNamespace(ns))
}

func GetCloneSetInCache(ns, name string, cl client.Reader) (*kruiseappsv1alpha1.CloneSet, bool, error) {
	cs := &kruiseappsv1alpha1.CloneSet{}
	found, err := GetResourceInCache(ns, name, cs, cl)
	if err != nil || !found {
		cs = nil
	}

	return cs, found, err
}

func GetPodsOwnedByCloneSet(ns, name string, cr client.Reader) (*corev1.PodList, error) {
	cs, found, err := GetCloneSetInCache(ns, name, cr)
	if err != nil || !found {
		klog.Error(err)
		return nil, fmt.Errorf("failed to retrieve CloneSet %s: %s", name, err.Error())
	}

	// List the Pods matching the PodTemplate Labels
	pods := &corev1.PodList{}
	err = cr.List(context.TODO(), pods, client.InNamespace(ns), client.MatchingLabels(cs.Spec.Template.Labels))
	if err != nil || !found {
		pods = nil
	}

	return pods, err
}
