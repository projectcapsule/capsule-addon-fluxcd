// Copyright 2020-2024 Project Capsule Authors.
// SPDX-License-Identifier: Apache-2.0

package serviceaccount

import (
	"context"
	"fmt"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *ServiceAccountReconciler) ensureRoles(ctx context.Context, saName, saNamespace string) error {
	subjects := []rbacv1.Subject{{Kind: "ServiceAccount", Name: saName, Namespace: saNamespace}}

	// Ensure the Service Account Namespace cluster-admin RoleBinding.
	homeAdminRB := &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      saName,
			Namespace: saNamespace,
		},
		Subjects: subjects,
		RoleRef: rbacv1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "cluster-admin",
		},
	}

	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, homeAdminRB, func() error {
		if homeAdminRB.Subjects == nil {
			homeAdminRB.Subjects = subjects
		}

		if homeAdminRB.RoleRef.Name != "cluster-admin" ||
			homeAdminRB.RoleRef.Kind != "ClusterRole" ||
			homeAdminRB.RoleRef.APIGroup != "rbac.authorization.k8s.io" {
			homeAdminRB.RoleRef = rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "cluster-admin",
			}
		}

		return nil
	}); err != nil {
		return err
	}

	// Ensure the Service Account impersonator ClusterRole.
	impersonatorName := fmt.Sprintf("%s-%s-impersonator", saNamespace, saName)

	impersonatorRules := []rbacv1.PolicyRule{{
		APIGroups:     []string{""},
		Verbs:         []string{"impersonate"},
		Resources:     []string{"users"},
		ResourceNames: []string{fmt.Sprintf("system:serviceaccount:%s:%s", saNamespace, saName)},
	}}

	impersonatorCR := &rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{Name: impersonatorName},
		Rules:      impersonatorRules,
	}

	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, impersonatorCR, func() error {
		if len(impersonatorCR.Rules) == 0 {
			impersonatorCR.Rules = impersonatorRules
		}
		if impersonatorCR.Rules[0].Verbs == nil {
			impersonatorCR.Rules[0].Verbs = []string{"impersonate"}
		}
		if impersonatorCR.Rules[0].Resources == nil {
			impersonatorCR.Rules[0].Resources = []string{"users"}
		}
		if impersonatorCR.Rules[0].ResourceNames == nil {
			impersonatorCR.Rules[0].ResourceNames = []string{
				fmt.Sprintf("system:serviceaccount:%s:%s", saNamespace, saName),
			}
		}

		return nil
	}); err != nil {
		return err
	}

	// Ensure the Service Account impersonator ClusterRoleBinding.
	impersonatorRoleRef := rbacv1.RoleRef{
		APIGroup: "rbac.authorization.k8s.io",
		Kind:     "ClusterRole",
		Name:     impersonatorName,
	}

	impersonatorCRB := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: impersonatorName},
		Subjects:   subjects,
		RoleRef:    impersonatorRoleRef,
	}

	if _, err := controllerutil.CreateOrUpdate(ctx, r.Client, impersonatorCRB, func() error {
		if impersonatorCRB.RoleRef.Kind != "ClusterRole" ||
			impersonatorCRB.RoleRef.APIGroup != "rbac.authorization.k8s.io" ||
			impersonatorCRB.RoleRef.Name != impersonatorName {
			impersonatorCRB.RoleRef = impersonatorRoleRef
		}
		if len(impersonatorCRB.Subjects) == 0 {
			impersonatorCRB.Subjects = subjects
		}

		return nil
	}); err != nil {
		return err
	}

	return nil
}
