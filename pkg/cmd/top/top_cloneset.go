package top

import (
	"context"
	"errors"
	"fmt"
	internalclient "github.com/hantmac/kubectl-kruise/pkg/client"
	"github.com/hantmac/kubectl-kruise/pkg/fetcher"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/discovery"
	corev1client "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/klog/v2"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/metricsutil"
	"k8s.io/kubectl/pkg/util/i18n"
	"k8s.io/kubectl/pkg/util/templates"
	metricsapi "k8s.io/metrics/pkg/apis/metrics"
	metricsv1beta1api "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type TopCloneSetOptions struct {
	ResourceName       string
	Namespace          string
	Selector           string
	SortBy             string
	AllNamespaces      bool
	PrintContainers    bool
	NoHeaders          bool
	UseProtocolBuffers bool

	CloneSetClient  client.Reader
	PodClient       corev1client.PodsGetter
	Printer         *metricsutil.TopCmdPrinter
	DiscoveryClient discovery.DiscoveryInterface
	MetricsClient   metricsclientset.Interface

	genericclioptions.IOStreams
}

var (
	topCloneSetLong = templates.LongDesc(i18n.T(`
		Display Resource (CPU/Memory) usage of clonesets.

		The top-clone command allows you to see the resource consumption of clonesets.`))

	topCloneSetExample = templates.Examples(i18n.T(`
		  # Show metrics for all clonesets
		  kubectl top clone

		  # Show metrics for a given cloneset
		  kubectl top clone CLONESET_NAME`))
)

func NewCmdTopClone(f cmdutil.Factory, o *TopCloneSetOptions, streams genericclioptions.IOStreams) *cobra.Command {
	if o == nil {
		o = &TopCloneSetOptions{
			IOStreams: streams,
		}
	}

	cmd := &cobra.Command{
		Use:                   "clone [NAME | -l label]",
		DisableFlagsInUseLine: true,
		Short:                 i18n.T("Display Resource (CPU/Memory) usage of nodes"),
		Long:                  topCloneSetLong,
		Example:               topCloneSetExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, cmd, args))
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.RunTopCloneSet())
		},
		Aliases: []string{"clonesets", "clone"},
	}
	cmd.Flags().StringVarP(&o.Selector, "selector", "l", o.Selector, "Selector (label query) to filter on, supports '=', '==', and '!='.(e.g. -l key1=value1,key2=value2)")
	cmd.Flags().StringVar(&o.SortBy, "sort-by", o.Selector, "If non-empty, sort nodes list using specified field. The field can be either 'cpu' or 'memory'.")
	cmd.Flags().BoolVarP(&o.AllNamespaces, "all-namespaces", "A", o.AllNamespaces, "If present, list the requested object(s) across all namespaces. Namespace in current context is ignored even if specified with --namespace.")
	cmd.Flags().BoolVar(&o.NoHeaders, "no-headers", o.NoHeaders, "If present, print output without headers")
	cmd.Flags().BoolVar(&o.UseProtocolBuffers, "use-protocol-buffers", o.UseProtocolBuffers, "If present, protocol-buffers will be used to request metrics.")

	return cmd
}

func (o *TopCloneSetOptions) Complete(f cmdutil.Factory, cmd *cobra.Command, args []string) error {
	var err error
	if len(args) == 1 {
		o.ResourceName = args[0]
	} else if len(args) > 1 {
		return cmdutil.UsageErrorf(cmd, "%s", cmd.Use)
	}

	clientset, err := f.KubernetesClientSet()
	if err != nil {
		return err
	}
	o.Namespace, _, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	o.DiscoveryClient = clientset.DiscoveryClient

	config, err := f.ToRESTConfig()
	if err != nil {
		return err
	}
	if o.UseProtocolBuffers {
		config.ContentType = "application/vnd.kubernetes.protobuf"
	} else {
		klog.Warning("Using json format to get metrics. Next release will switch to protocol-buffers, switch early by passing --use-protocol-buffers flag")
	}
	o.MetricsClient, err = metricsclientset.NewForConfig(config)
	if err != nil {
		return err
	}

	o.CloneSetClient = internalclient.NewManager().GetAPIReader()

	o.Printer = metricsutil.NewTopCmdPrinter(o.Out)
	return nil
}

func (o *TopCloneSetOptions) Validate() error {
	if len(o.SortBy) > 0 {
		if o.SortBy != sortByCPU && o.SortBy != sortByMemory {
			return errors.New("--sort-by accepts only cpu or memory")
		}
	}
	if len(o.ResourceName) > 0 && len(o.Selector) > 0 {
		return errors.New("only one of NAME or --selector can be provided")
	}
	return nil
}

func (o TopCloneSetOptions) RunTopCloneSet() error {

	var err error
	selector := labels.Everything()
	if len(o.Selector) > 0 {
		selector, err = labels.Parse(o.Selector)
		if err != nil {
			return err
		}
	}

	apiGroups, err := o.DiscoveryClient.ServerGroups()
	if err != nil {
		return err
	}

	metricsAPIAvailable := SupportedMetricsAPIVersionAvailable(apiGroups)

	if !metricsAPIAvailable {
		return errors.New("Metrics API not available")
	}
	pods, err := fetcher.GetPodsOwnedByCloneSet(o.Namespace, o.ResourceName, o.CloneSetClient)

	if pods == nil {
		return fmt.Errorf("CloneSet %s has no pods", o.ResourceName)
	}

	metrics, err := getPodOfCloneMetricsFromMetricsAPI(pods, o.MetricsClient, o.Namespace, o.ResourceName, o.AllNamespaces, selector)
	if err != nil {
		return err
	}

	// First we check why no metrics have been received.
	if len(metrics.Items) == 0 {
		// If the API server query is successful but all the pods are newly created,
		// the metrics are probably not ready yet, so we return the error here in the first place.
		err := verifyCloneSetEmptyMetrics(o, selector)
		if err != nil {
			return err
		}

		// if we had no errors, be sure we output something.
		if o.AllNamespaces {
			fmt.Fprintln(o.ErrOut, "No resources found")
		} else {
			fmt.Fprintf(o.ErrOut, "No resources found in %s namespace.\n", o.Namespace)
		}
	}

	return o.Printer.PrintPodMetrics(metrics.Items, o.PrintContainers, o.AllNamespaces, o.NoHeaders, o.SortBy)
}

func getPodOfCloneMetricsFromMetricsAPI(podList *corev1.PodList, metricsClient metricsclientset.Interface, namespace, resourceName string, allNamespaces bool, selector labels.Selector) (*metricsapi.PodMetricsList, error) {
	var err error
	ns := metav1.NamespaceAll
	if !allNamespaces {
		ns = namespace
	}
	versionedMetrics := &metricsv1beta1api.PodMetricsList{}
	if podList != nil {
		for _, v := range podList.Items {

			m, err := metricsClient.MetricsV1beta1().PodMetricses(ns).Get(context.TODO(), v.Name, metav1.GetOptions{})
			if err != nil {
				return nil, err
			}
			versionedMetrics.Items = append(versionedMetrics.Items, *m)
		}

	} else {
		versionedMetrics, err = metricsClient.MetricsV1beta1().PodMetricses(ns).List(context.TODO(), metav1.ListOptions{LabelSelector: selector.String()})
		if err != nil {
			return nil, err
		}
	}
	metrics := &metricsapi.PodMetricsList{}
	err = metricsv1beta1api.Convert_v1beta1_PodMetricsList_To_metrics_PodMetricsList(versionedMetrics, metrics, nil)
	if err != nil {
		return nil, err
	}
	return metrics, nil
}

func verifyCloneSetEmptyMetrics(o TopCloneSetOptions, selector labels.Selector) error {
	if len(o.ResourceName) > 0 {
		pod, err := o.PodClient.Pods(o.Namespace).Get(context.TODO(), o.ResourceName, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if err := checkPodAge(pod); err != nil {
			return err
		}
	} else {
		pods, err := o.PodClient.Pods(o.Namespace).List(context.TODO(), metav1.ListOptions{
			LabelSelector: selector.String(),
		})
		if err != nil {
			return err
		}
		if len(pods.Items) == 0 {
			return nil
		}
		for _, pod := range pods.Items {
			if err := checkPodAge(&pod); err != nil {
				return err
			}
		}
	}
	return errors.New("metrics not available yet")
}
