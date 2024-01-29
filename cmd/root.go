package cmd

import (
	"github.com/projectcapsule/capsule-addon-flux/cmd/manager"
	"github.com/spf13/cobra"
)

func New() *cobra.Command {
	cmd := &cobra.Command{
		Use: commandName,
	}

	cmd.AddCommand(manager.New())

	return cmd
}

func Execute() error {
	cmd := New()
	return cmd.Execute()
}
