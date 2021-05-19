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

// ResumeOptions is the start of the data required to perform the operation.  As new fields are added, add them here instead of
// referencing the cmd.Flags()
type ResumeOptions struct {
	PrintFlags *genericclioptions.PrintFlags
	ToPrinter  func(string) (printers.ResourcePrinter, error)

	Resources []string

	Builder          func() *resource.Builder
	Resumer          internalpolymorphichelpers.ObjectResumerFunc
	Namespace        string
	EnforceNamespace bool

	resource.FilenameOptions
	genericclioptions.IOStreams
}

var (
	resumeLong = templates.LongDesc(`
		Resume a paused resource

		Paused resources will not be reconciled by a controller. By resuming a
		resource, we allow it to be reconciled again.
		Currently deployments, cloneset support being resumed.`)

	resumeExample = templates.Examples(`
		# Resume an already paused deployment
		
		kubectl-kruise rollout resume cloneset/nginx
		kubectl-kruise rollout resume deployment/nginx`)
)

// NewRolloutResumeOptions returns an initialized ResumeOptions instance
func NewRolloutResumeOptions(streams genericclioptions.IOStreams) *ResumeOptions {
	return &ResumeOptions{
		PrintFlags: genericclioptions.NewPrintFlags("resumed").WithTypeSetter(internalclient.Scheme),
		IOStreams:  streams,
	}
}

// NewCmdRolloutResume returns a Command instance for 'rollout resume' sub command
func NewCmdRolloutResume(f cmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewRolloutResumeOptions(streams)

	validArgs := []string{"deployment", "cloneset"}

	cmd := &cobra.Command{
		Use:                   "resume RESOURCE",
		DisableFlagsInUseLine: true,
		Short:                 i18n.T("Resume a paused resource"),
		Long:                  resumeLong,
		Example:               resumeExample,
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(f, cmd, args))
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.RunResume())
		},
		ValidArgs: validArgs,
	}

	usage := "identifying the resource to get from a server."
	cmdutil.AddFilenameOptionFlags(cmd, &o.FilenameOptions, usage)
	o.PrintFlags.AddFlags(cmd)
	return cmd
}

// Complete completes all the required options
func (o *ResumeOptions) Complete(f cmdutil.Factory, cmd *cobra.Command, args []string) error {
	o.Resources = args

	o.Resumer = internalpolymorphichelpers.ObjectResumerFn

	var err error
	o.Namespace, o.EnforceNamespace, err = f.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}

	o.ToPrinter = func(operation string) (printers.ResourcePrinter, error) {
		o.PrintFlags.NamePrintFlags.Operation = operation
		return o.PrintFlags.ToPrinter()
	}

	o.Builder = f.NewBuilder

	return nil
}

func (o *ResumeOptions) Validate() error {
	if len(o.Resources) == 0 && cmdutil.IsFilenameSliceEmpty(o.Filenames, o.Kustomize) {
		return fmt.Errorf("required resource not specified")
	}
	return nil
}

// RunResume performs the execution of 'rollout resume' sub command
func (o ResumeOptions) RunResume() error {
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

	allErrs := []error{}
	infos, err := r.Infos()
	if err != nil {
		// restore previous command behavior where
		// an error caused by retrieving infos due to
		// at least a single broken object did not result
		// in an immediate return, but rather an overall
		// aggregation of errors.
		allErrs = append(allErrs, err)
	}

	for _, patch := range set.CalculatePatches(infos, scheme.DefaultJSONEncoder(), set.PatchFn(o.Resumer)) {
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
			printer, err := o.ToPrinter("already resumed")
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
		printer, err := o.ToPrinter("resumed")
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
