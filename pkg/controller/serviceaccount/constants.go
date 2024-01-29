// Copyright 2020-2024 Project Capsule Authors.
// SPDX-License-Identifier: Apache-2.0

package serviceaccount

const (
	ManagerName = "capsule-addon-fluxcd"

	GlobalTenantResourceSuffix = "-kubeconfig"

	// #nosec G101
	SecretNameSuffixKubeconfig = "-kubeconfig"
	SecretNameSuffixToken      = "-token"
	SecretKeyKubeconfig        = "kubeconfig"

	ServiceAccountAddonAnnotationKey   = "capsule.addon.fluxcd/enabled"
	ServiceAccountAddonAnnotationValue = "true"

	ServiceAccountGlobalAnnotationKey   = "capsule.addon.fluxcd/kubeconfig-global"
	ServiceAccountGlobalAnnotationValue = "true"

	KubeconfigClusterName = "default"
	KubeconfigUserName    = "default"
	KubeconfigContextName = "default"
)
