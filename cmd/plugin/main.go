package main

import (
	"os"

	"github.com/hantmac/kubectl-kruise/pkg/cmd"
	"github.com/spf13/pflag"
)

func main() {
	flags := pflag.NewFlagSet("kubectl-kruise", pflag.ExitOnError)
	pflag.CommandLine = flags

	root := cmd.NewDefaultKubectlCommand()
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
