/*
Copyright 2021 The Kruise Authors.
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package set

import (
	"fmt"

	internalclient "github.com/hantmac/kubectl-kruise/pkg/client"
	internalpolymorphichelpers "github.com/hantmac/kubectl-kruise/pkg/internal/polymorphichelpers"
	"github.com/spf13/cobra"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/klog"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	generateversioned "k8s.io/kubectl/pkg/generate/versioned"
	"k8s.io/kubectl/pkg/scheme"
	"k8s.io/kubectl/pkg/util/i18n"
	"k8s.io/kubectl/pkg/util/templates"
)

var (
	setresourcesResouces = `
	replicationcontroller (rc), deployment (deploy), daemonset (ds), job, replicaset (rs), statefulset, cloneset (cs), advanced statefulset (asts)`

	resourcesLong = templates.LongDesc(`
		Specify compute resource requirements (cpu, memory) for any resource that defines a pod template.  If a pod is successfully scheduled, it is guaranteed the amount of resource requested, but may burst up to its specified limits.

		for each compute resource, if a limit is specified and a request is omitted, the request will default to the limit.

		Possible resources include (case insensitive): ` + setresourcesResouces)

	resourcesExample = templates.Examples(`
		# Set a deployments nginx container cpu limits to "200m" and memory to "512Mi"
		kubectl-kruise set resources deployment nginx -c=nginx --limits=cpu=200m,memory=512Mi

		# Set a cloneset nginx container cpu limits to "200m" and memory to "512Mi" 
		kubectl-kruise set resources cloneset nginx -c=nginx --limits=cpu=200m,memory=512Mi

		# Set an advanced statefulset nginx container cpu limits to "200m" and memory to "512Mi"
		kubectl-kruise set resources asts nginx -c=nginx --limits=cpu=200m,memory=512Mi

		# Print the result (in yaml format) of updating nginx container limits from a local, without hitting the server
		kubectl-kruise set resources -f path/to/file.yaml --limits=cpu=200m,memory=512Mi --local -o yaml`)
)

// SetResourcesOptions is the start of the data required to perform the operation. As new fields are added, add them here instead of
// referencing the cmd.Flags
type SetResourcesOptions struct {
	resource.FilenameOptions

	PrintFlags  *genericclioptions.PrintFlags
	RecordFlags *genericclioptions.RecordFlags

	Infos             []*resource.Info
	Selector          string
	ContainerSelector string
	Output            string
	All               bool
	Local             bool

	DryRunStrategy cmdutil.DryRunStrategy

	PrintObj printers.ResourcePrinterFunc
	Recorder genericclioptions.Recorder

	Limits               string
	Requests             string
	ResourceRequirements v1.ResourceRequirements

	UpdatePodSpecForObject internalpolymorphichelpers.UpdatePodSpecForObjectFunc
	Resources              []string
	DryRunVerifier         *resource.DryRunVerifier

	genericclioptions.IOStreams
}

// NewResourcesOptions returns a ResourcesOptions indicating all containers in the selected
// pod templates are selected by default.
func NewResourcesOptions(streams genericclioptions.IOStreams) *SetResourcesOptions {
	return &SetResourcesOptions{
		PrintFlags:  genericclioptions.NewPrintFlags("resource requirements updated").WithTypeSetter(internalclient.Scheme),
		RecordFlags: genericclioptions.NewRecordFlags(),

		Recorder: genericclioptions.NoopRecorder{},

		ContainerSelector: "*",

		IOStreams: streams,
	}
}

// NewCmdResources returns initialized Command instance for the 'set resources' sub command
func NewCmdResources(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewResourcesOptions(streams)

	cmd := &cobra.Command{
		Use:                   "resources (-f FILENAME | TYPE NAME)  ([--limits=LIMITS & --requests=REQUESTS]",
		DisableFlagsInUseLine: true,
		Short:                 i18n.T("Update resource requests/limits on objects with pod templates"),
		Long:                  fmt.Sprintf(resourcesLong, cmdutil.SuggestAPIResources("kubectl")),
		Example:               resourcesExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, cmd, args))
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run())
		},
	}

	o.PrintFlags.AddFlags(cmd)
	o.RecordFlags.AddFlags(cmd)

	//usage := "Filename, directory, or URL to a file identifying the resource to get from the server"
	//kubectl.AddJsonFilenameFlag(cmd, &options.Filenames, usage)
	usage := "identifying the resource to get from a server."
	cmdutil.AddFilenameOptionFlags(cmd, &o.FilenameOptions, usage)
	cmd.Flags().BoolVar(&o.All, "all", o.All, "Select all resources, including uninitialized ones, in the namespace of the specified resource types")
	cmd.Flags().StringVarP(&o.Selector, "selector", "l", o.Selector, "Selector (label query) to filter on, not including uninitialized ones,supports '=', '==', and '!='.(e.g. -l key1=value1,key2=value2)")
	cmd.Flags().StringVarP(&o.ContainerSelector, "containers", "c", o.ContainerSelector, "The names of containers in the selected pod templates to change, all containers are selected by default - may use wildcards")
	cmd.Flags().BoolVar(&o.Local, "local", o.Local, "If true, set resources will NOT contact api-server but run locally.")
	cmdutil.AddDryRunFlag(cmd)
	cmd.Flags().StringVar(&o.Limits, "limits", o.Limits, "The resource requirement requests for this container.  For example, 'cpu=100m,memory=256Mi'.  Note that server side components may assign requests depending on the server configuration, such as limit ranges.")
	cmd.Flags().StringVar(&o.Requests, "requests", o.Requests, "The resource requirement requests for this container.  For example, 'cpu=100m,memory=256Mi'.  Note that server side components may assign requests depending on the server configuration, such as limit ranges.")
	return cmd
}

// Complete completes all required options
func (o *SetResourcesOptions) Complete(f cmdutil.Factory, cmd *cobra.Command, args []string) error {
	var err error

	o.RecordFlags.Complete(cmd)
	o.Recorder, err = o.RecordFlags.ToRecorder()
	if err != nil {
		return err
	}

	o.UpdatePodSpecForObject = internalpolymorphichelpers.UpdatePodSpecForObjectFn
	o.Output = cmdutil.GetFlagString(cmd, "output")
	o.DryRunStrategy, err = cmdutil.GetDryRunStrategy(cmd)
	if err != nil {
		return err
	}
	dynamicClient, err := f.DynamicClient()
	if err != nil {
		return err
	}
	discoveryClient, err := f.ToDiscoveryClient()
	if err != nil {
		return err
	}
	o.DryRunVerifier = resource.NewDryRunVerifier(dynamicClient, discoveryClient)

	cmdutil.PrintFlagsWithDryRunStrategy(o.PrintFlags, o.DryRunStrategy)
	printer, err := o.PrintFlags.ToPrinter()
	if err != nil {
		return err
	}
	o.PrintObj = printer.PrintObj

	cmdNamespace, enforceNamespace, err := f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	builder := f.NewBuilder().
		WithScheme(internalclient.Scheme, scheme.Scheme.PrioritizedVersionsAllGroups()...).
		LocalParam(o.Local).
		ContinueOnError().
		NamespaceParam(cmdNamespace).DefaultNamespace().
		FilenameParam(enforceNamespace, &o.FilenameOptions).
		Flatten()

	if !o.Local {
		builder.LabelSelectorParam(o.Selector).
			ResourceTypeOrNameArgs(o.All, args...).
			Latest()
	} else {
		// if a --local flag was provided, and a resource was specified in the form
		// <resource>/<name>, fail immediately as --local cannot query the api server
		// for the specified resource.
		// TODO: this should be in the builder - if someone specifies tuples, fail when
		// local is true
		if len(args) > 0 {
			return resource.LocalResourceError
		}
	}

	o.Infos, err = builder.Do().Infos()
	if err != nil {
		return err
	}
	return nil
}

// Validate makes sure that provided values in ResourcesOptions are valid
func (o *SetResourcesOptions) Validate() error {
	var err error
	if o.Local && o.DryRunStrategy == cmdutil.DryRunServer {
		return fmt.Errorf("cannot specify --local and --dry-run=server - did you mean --dry-run=client?")
	}
	if o.All && len(o.Selector) > 0 {
		return fmt.Errorf("cannot set --all and --selector at the same time")
	}
	if len(o.Limits) == 0 && len(o.Requests) == 0 {
		return fmt.Errorf("you must specify an update to requests or limits (in the form of --requests/--limits)")
	}

	o.ResourceRequirements, err = generateversioned.HandleResourceRequirementsV1(map[string]string{"limits": o.Limits, "requests": o.Requests})
	if err != nil {
		return err
	}

	return nil
}

// Run performs the execution of 'set resources' sub command
func (o *SetResourcesOptions) Run() error {
	allErrs := []error{}
	patches := CalculatePatches(o.Infos, scheme.DefaultJSONEncoder(), func(obj runtime.Object) ([]byte, error) {
		transformed := false
		_, err := o.UpdatePodSpecForObject(obj, func(spec *v1.PodSpec) error {
			containers, _ := selectContainers(spec.Containers, o.ContainerSelector)
			if len(containers) != 0 {
				for i := range containers {
					if len(o.Limits) != 0 && len(containers[i].Resources.Limits) == 0 {
						containers[i].Resources.Limits = make(v1.ResourceList)
					}
					for key, value := range o.ResourceRequirements.Limits {
						containers[i].Resources.Limits[key] = value
					}

					if len(o.Requests) != 0 && len(containers[i].Resources.Requests) == 0 {
						containers[i].Resources.Requests = make(v1.ResourceList)
					}
					for key, value := range o.ResourceRequirements.Requests {
						containers[i].Resources.Requests[key] = value
					}
					transformed = true
				}
			} else {
				allErrs = append(allErrs, fmt.Errorf("error: unable to find container named %s", o.ContainerSelector))
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
		if !transformed {
			return nil, nil
		}
		// record this change (for rollout history)
		if err := o.Recorder.Record(obj); err != nil {
			klog.V(4).Infof("error recording current command: %v", err)
		}

		return runtime.Encode(scheme.DefaultJSONEncoder(), obj)
	})

	for _, patch := range patches {
		info := patch.Info
		name := info.ObjectName()
		if patch.Err != nil {
			allErrs = append(allErrs, fmt.Errorf("error: %s %v\n", name, patch.Err))
			continue
		}

		//no changes
		if string(patch.Patch) == "{}" || len(patch.Patch) == 0 {
			continue
		}

		if o.Local || o.DryRunStrategy == cmdutil.DryRunClient {
			if err := o.PrintObj(info.Object, o.Out); err != nil {
				allErrs = append(allErrs, err)
			}
			continue
		}

		if o.DryRunStrategy == cmdutil.DryRunServer {
			if err := o.DryRunVerifier.HasSupport(info.Mapping.GroupVersionKind); err != nil {
				allErrs = append(allErrs, fmt.Errorf("failed to patch resources update to pod template %v", err))
				continue
			}
		}

		actual, err := resource.
			NewHelper(info.Client, info.Mapping).
			DryRun(o.DryRunStrategy == cmdutil.DryRunServer).
			Patch(info.Namespace, info.Name, types.MergePatchType, patch.Patch, nil)
		if err != nil {
			allErrs = append(allErrs, fmt.Errorf("failed to patch resources update to pod template %v", err))
			continue
		}

		if err := o.PrintObj(actual, o.Out); err != nil {
			allErrs = append(allErrs, err)
		}
	}
	return utilerrors.NewAggregate(allErrs)
}
