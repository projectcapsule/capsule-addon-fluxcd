// Copyright 2020-2024 Project Capsule Authors.
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"github.com/spf13/cobra"

	"github.com/projectcapsule/capsule-addon-flux/cmd/manager"
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
