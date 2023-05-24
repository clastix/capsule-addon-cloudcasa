// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package annotations

import (
	capsulev1beta2 "github.com/clastix/capsule/api/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Annotations interface {
	OwnerEmail(tenant *capsulev1beta2.Tenant, owner capsulev1beta2.OwnerSpec) string
	UserGroupID(object client.Object) (string, bool)
	ClusterID(object client.Object) (string, bool)
	OrganizationID(tenant *capsulev1beta2.Tenant) string
}
