// Copyright 2020-2024 Project Capsule Authors.
// SPDX-License-Identifier: Apache-2.0

package indexer

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"github.com/projectcapsule/capsule/pkg/indexer/tenant"
	"github.com/projectcapsule/capsule/pkg/utils"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func AddToManager(ctx context.Context, log logr.Logger, mgr manager.Manager) error {
	indexer := tenant.OwnerReference{}

	if err := mgr.GetFieldIndexer().IndexField(ctx, indexer.Object(), indexer.Field(), indexer.Func()); err != nil {
		if utils.IsUnsupportedAPI(err) {
			log.Info(fmt.Sprintf("skipping setup of Indexer %T for object %T", indexer, indexer.Object()), "error", err.Error())
		}

		return err
	}

	return nil
}
