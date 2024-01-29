// Copyright 2020-2024 Project Capsule Authors.
// SPDX-License-Identifier: Apache-2.0

package serviceaccount

import "github.com/pkg/errors"

var (
	ErrServiceAccountTokenNotFound    = errors.New("service account token not found")
	ErrGetServiceAccountToken         = errors.New("error getting service account token")
	ErrServiceAccountTokenSecretEmpty = errors.New("the service account token secret is empty")
)
