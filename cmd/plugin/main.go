package main

import (
	"github.com/hantmac/kubectl-kruise/pkg/cmd"
	"github.com/spf13/pflag"
	"os"
)

func main() {
	flags := pflag.NewFlagSet("kubectl-kruise", pflag.ExitOnError)
	pflag.CommandLine = flags

	root := cmd.NewDefaultKubectlCommand()
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
