// Copyright 2020-2024 Project Capsule Authors.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"

	"github.com/projectcapsule/capsule-addon-flux/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		//nolint:forbidigo
		fmt.Println(err)
		os.Exit(1)
	}
}
