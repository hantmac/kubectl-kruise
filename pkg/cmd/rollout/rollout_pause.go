package rollout

import (
	"fmt"
	internalclient "github.com/hantmac/kubectl-kruise/pkg/client"
	internalpolymorphichelpers "github.com/hantmac/kubectl-kruise/pkg/internal/polymorphichelpers"
	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/types"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/printers"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/kubectl/pkg/cmd/set"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/scheme"
	"k8s.io/kubectl/pkg/util/i18n"
	"k8s.io/kubectl/pkg/util/templates"
)

// PauseOptions is the start of the data required to perform the operation.  As new fields are added, add them here instead of
// referencing the cmd.Flags()
type PauseOptions struct {
	PrintFlags *genericclioptions.PrintFlags
	ToPrinter  func(string) (printers.ResourcePrinter, error)

	Pauser           internalpolymorphichelpers.ObjectPauserFunc
	Builder          func() *resource.Builder
	Namespace        string
	EnforceNamespace bool
	Resources        []string

	resource.FilenameOptions
	genericclioptions.IOStreams
}

var (
	pauseLong = templates.LongDesc(`
		Mark the provided resource as paused

		Paused resources will not be reconciled by a controller.
		Use "kubectl rollout resume" to resume a paused resource.
		Currently  deployments, clonesets, advanced statefulsets support being paused.`)

	pauseExample = templates.Examples(`
		# Mark the nginx deployment as paused. Any current state of
		# the deployment will continue its function, new updates to the deployment will not
		# have an effect as long as the deployment is paused.

		kubectl-kruise rollout pause cloneset/nginx

		kubectl-kruise rollout pause deployment/nginx`)
)

// NewCmdRolloutPause returns a Command instance for 'rollout pause' sub command
func NewCmdRolloutPause(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := &PauseOptions{
		PrintFlags: genericclioptions.NewPrintFlags("paused").WithTypeSetter(internalclient.Scheme),
		IOStreams:  streams,
	}

	validArgs := []string{"deployment", "cloneset", "advancedstatefulset"}

	cmd := &cobra.Command{
		Use:                   "pause RESOURCE",
		DisableFlagsInUseLine: true,
		Short:                 i18n.T("Mark the provided resource as paused"),
		Long:                  pauseLong,
		Example:               pauseExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, cmd, args))
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.RunPause())
		},
		ValidArgs: validArgs,
	}

	o.PrintFlags.AddFlags(cmd)

	usage := "identifying the resource to get from a server."
	cmdutil.AddFilenameOptionFlags(cmd, &o.FilenameOptions, usage)
	return cmd
}

// Complete completes all the required options
func (o *PauseOptions) Complete(f cmdutil.Factory, cmd *cobra.Command, args []string) error {
	o.Pauser = internalpolymorphichelpers.ObjectPauserFn

	var err error
	o.Namespace, o.EnforceNamespace, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	o.Resources = args
	o.Builder = f.NewBuilder

	o.ToPrinter = func(operation string) (printers.ResourcePrinter, error) {
		o.PrintFlags.NamePrintFlags.Operation = operation
		return o.PrintFlags.ToPrinter()
	}

	return nil
}

func (o *PauseOptions) Validate() error {
	if len(o.Resources) == 0 && cmdutil.IsFilenameSliceEmpty(o.Filenames, o.Kustomize) {
		return fmt.Errorf("required resource not specified")
	}
	return nil
}

// RunPause performs the execution of 'rollout pause' sub command
func (o *PauseOptions) RunPause() error {
	r := o.Builder().
		WithScheme(internalclient.Scheme, scheme.Scheme.PrioritizedVersionsAllGroups()...).
		NamespaceParam(o.Namespace).DefaultNamespace().
		FilenameParam(o.EnforceNamespace, &o.FilenameOptions).
		ResourceTypeOrNameArgs(true, o.Resources...).
		ContinueOnError().
		Latest().
		Flatten().
		Do()
	if err := r.Err(); err != nil {
		return err
	}

	var allErrs []error
	infos, err := r.Infos()
	if err != nil {
		// restore previous command behavior where
		// an error caused by retrieving infos due to
		// at least a single broken object did not result
		// in an immediate return, but rather an overall
		// aggregation of errors.
		allErrs = append(allErrs, err)
	}

	for _, patch := range set.CalculatePatches(infos, scheme.DefaultJSONEncoder(), set.PatchFn(o.Pauser)) {
		info := patch.Info

		if patch.Err != nil {
			resourceString := info.Mapping.Resource.Resource
			if len(info.Mapping.Resource.Group) > 0 {
				resourceString = resourceString + "." + info.Mapping.Resource.Group
			}
			allErrs = append(allErrs, fmt.Errorf("error: %s %q %v", resourceString, info.Name, patch.Err))
			continue
		}

		if string(patch.Patch) == "{}" || len(patch.Patch) == 0 {
			printer, err := o.ToPrinter("already paused")
			if err != nil {
				allErrs = append(allErrs, err)
				continue
			}
			if err = printer.PrintObj(info.Object, o.Out); err != nil {
				allErrs = append(allErrs, err)
			}
			continue
		}

		obj, err := resource.NewHelper(info.Client, info.Mapping).Patch(info.Namespace, info.Name, types.MergePatchType, patch.Patch, nil)
		if err != nil {
			allErrs = append(allErrs, fmt.Errorf("failed to patch: %v", err))
			continue
		}

		info.Refresh(obj, true)
		printer, err := o.ToPrinter("paused")
		if err != nil {
			allErrs = append(allErrs, err)
			continue
		}
		if err = printer.PrintObj(info.Object, o.Out); err != nil {
			allErrs = append(allErrs, err)
		}
	}

	return utilerrors.NewAggregate(allErrs)
}
