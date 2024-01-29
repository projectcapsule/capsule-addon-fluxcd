// Copyright 2020-2024 Project Capsule Authors.
// SPDX-License-Identifier: Apache-2.0

package serviceaccount

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// ensureSATokenSecret ensures that a token Secret is present for the Service Account of which the name and the namespace
// are specified as arguments.
func (r *ServiceAccountReconciler) ensureSATokenSecret(ctx context.Context, name, namespace string) error {
	// If the token does not exist, create it.
	if _, err := r.getSATokenSecret(ctx, name, namespace); err != nil {
		if errors.Is(err, ErrServiceAccountTokenNotFound) {
			tokenSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s%s", name, SecretNameSuffixToken),
					Namespace: namespace,
					Annotations: map[string]string{
						corev1.ServiceAccountNameKey: name,
					},
				},
				Type: corev1.SecretTypeServiceAccountToken,
			}
			if _, err = controllerutil.CreateOrUpdate(ctx, r.Client, tokenSecret, func() error {
				return nil
			}); err != nil {
				return err
			}

			return nil
		}

		return err
	}

	return nil
}

// getSATokenSecret returns, if exists, the token Secret of the Service Account of which the name and the namespace
// are specified as arguments.
func (r *ServiceAccountReconciler) getSATokenSecret(ctx context.Context, saName, saNamespace string) (*corev1.Secret, error) {
	saTokenList := new(corev1.SecretList)
	if err := r.Client.List(ctx, saTokenList); err != nil {
		return nil, ErrServiceAccountTokenNotFound
	}

	if len(saTokenList.Items) == 0 {
		return nil, ErrServiceAccountTokenNotFound
	}

	var tokenSecret *corev1.Secret

	for _, v := range saTokenList.Items {
		v := v
		if v.Type == corev1.SecretTypeServiceAccountToken {
			if v.Namespace == saNamespace && v.Annotations[corev1.ServiceAccountNameKey] == saName {
				return &v, nil
			}
		}
	}

	if tokenSecret == nil {
		return nil, ErrServiceAccountTokenNotFound
	}

	return tokenSecret, nil
}
