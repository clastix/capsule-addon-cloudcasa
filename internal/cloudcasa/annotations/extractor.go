// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package annotations

import (
	"fmt"
	"strings"

	capsulev1beta1 "github.com/clastix/capsule/api/v1beta1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Extractor struct{}

func (e Extractor) OrganizationID(tenant *capsulev1beta1.Tenant) string {
	annotations := tenant.GetAnnotations()

	if annotations == nil {
		return ""
	}

	v, ok := annotations[OrganizationAnnotation]
	if !ok {
		return ""
	}

	return v
}

func (e Extractor) ClusterID(object client.Object) (string, bool) {
	annotations := object.GetAnnotations()

	if annotations == nil {
		return "", false
	}

	v, ok := annotations[ClusterIDAnnotation]

	return v, ok
}

func (e Extractor) UserGroupID(object client.Object) (string, bool) {
	annotations := object.GetAnnotations()

	if annotations == nil {
		return "", false
	}

	v, ok := annotations[UserGroupAnnotation]

	return v, ok
}

func (e Extractor) OwnerEmail(tenant *capsulev1beta1.Tenant, owner capsulev1beta1.OwnerSpec) string {
	email := owner.Name

	annotations := tenant.GetAnnotations()
	if annotations == nil {
		return email
	}

	overrideAnnotation := fmt.Sprintf("%s/%s.%s", UserEmailOverridePattern, strings.ToLower(owner.Kind.String()), strings.ToLower(owner.Name))
	v, ok := annotations[overrideAnnotation]
	if !ok {
		return email
	}

	return v
}
