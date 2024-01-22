// Copyright 2020-2024 Project Capsule Authors.
// SPDX-License-Identifier: Apache-2.0

package serviceaccount

import (
	"context"

	capsulev1beta2 "github.com/projectcapsule/capsule/api/v1beta2"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// Set the Tenant owner reference on the Namespace specified.
func (r *ServiceAccountReconciler) setNamespaceOwnerRef(ctx context.Context, ns *corev1.Namespace, tnt *capsulev1beta2.Tenant) error {
	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, ns, func() error {
		if err := controllerutil.SetControllerReference(tnt, ns, r.Client.Scheme()); err != nil {
			return err
		}

		return nil
	}); err != nil {
		return err
	}

	return nil
}
