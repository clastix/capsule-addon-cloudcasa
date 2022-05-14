// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package annotations

import (
	capsulev1beta1 "github.com/clastix/capsule/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Annotations interface {
	OwnerEmail(tenant *capsulev1beta1.Tenant, owner capsulev1beta1.OwnerSpec) string
	UserGroupID(object client.Object) (string, bool)
	ClusterID(object client.Object) (string, bool)
	OrganizationID(tenant *capsulev1beta1.Tenant) string
}
