// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"fmt"
	"net/http"

	capsulev1beta2 "github.com/clastix/capsule/api/v1beta2"
	goerr "github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/clastix/capsule-addon-cloudcasa/internal/cloudcasa/annotations"
	cloudcasa "github.com/clastix/capsule-addon-cloudcasa/internal/cloudcasa/oapi"
)

type Manager struct {
	client    client.Client
	cloudCasa *cloudcasa.ClientWithResponses
	extractor annotations.Annotations
}

func (m *Manager) SetupWithManager(serverURL, token string, mgr manager.Manager) error {
	cc, err := cloudcasa.NewClientWithResponses(serverURL, cloudcasa.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))

		return nil
	}))
	if err != nil {
		return err
	}

	m.cloudCasa = cc
	m.extractor = &annotations.Extractor{}

	return ctrl.NewControllerManagedBy(mgr).
		For(&capsulev1beta2.Tenant{}, builder.WithPredicates(predicate.Funcs{
			CreateFunc: func(event event.CreateEvent) (ok bool) {
				_, ok = m.extractor.ClusterID(event.Object)
				if !ok {
					return false
				}

				return true
			},
			UpdateFunc: func(updateEvent event.UpdateEvent) (ok bool) {
				_, ok = m.extractor.ClusterID(updateEvent.ObjectNew)
				if !ok {
					return false
				}

				return ok
			},
		})).
		Complete(m)
}

func (m *Manager) InjectClient(client client.Client) error {
	m.client = client

	return nil
}

func (m *Manager) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("Starting reconciliation")

	tenant := &capsulev1beta2.Tenant{}

	if err := m.client.Get(ctx, request.NamespacedName, tenant); err != nil {
		if k8serr.IsNotFound(err) {
			return reconcile.Result{}, nil
		}

		logger.Error(err, "cannot retrieve *capsulev1beta2.Tenant")

		return reconcile.Result{}, err
	}

	if err := m.ensureUserGroup(ctx, tenant); err != nil {
		logger.Error(err, "CloudCasa UserGroup for the given tenant does not exist")

		return reconcile.Result{}, err
	}

	for _, owner := range tenant.Spec.Owners {
		if err := m.ensureUser(ctx, owner, tenant); err != nil {
			logger.Error(err, fmt.Sprintf("failed ensuring user %s for Tenant %s", owner.Name, tenant.GetName()))
		}
	}

	if err := m.ensureKubernetesNamespaces(ctx, tenant); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (m *Manager) ensureUser(ctx context.Context, owner capsulev1beta2.OwnerSpec, tenant *capsulev1beta2.Tenant) error {
	_, userGroup, err := m.retrieveUserGroup(ctx, tenant)

	email := m.extractor.OwnerEmail(tenant, owner)

	invitationStatus, err := m.getUserInvitation(ctx, email)
	if err != nil {
		return err
	}

	switch {
	case invitationStatus == nil:
		err = m.createInvitation(ctx, email, *userGroup.Id, tenant.GetName())
	default:
		err = nil
	}

	return err
}

func (m *Manager) getUserInvitation(ctx context.Context, email string) (*cloudcasa.OrginviteState, error) {
	where := cloudcasa.QueryWhere(fmt.Sprintf(`{"email": %q}`, email))

	res, err := m.cloudCasa.Getv1orginvitesWithResponse(ctx, &cloudcasa.Getv1orginvitesParams{Where: &where})
	if err != nil {
		return nil, goerr.Wrap(err, "cannot retrieve list of OrgInvites for the current user")
	}

	if resErr := res.JSONDefault; resErr != nil {
		return nil, m.formatCloudCasaError(resErr)
	}

	items := res.JSON200.Items

	switch {
	case len(*items) == 0:
		return nil, nil
	case len(*items) == 1:
		return (*items)[0].State, nil
	}

	return nil, fmt.Errorf("cannot retrieve OrgInvite for the current user, multiple entries")
}

func (m *Manager) formatCloudCasaError(err *cloudcasa.Error) error {
	return fmt.Errorf(fmt.Sprintf("%s (%d)", *err.Error.Message, *err.Error.Code))
}

func (m *Manager) retrieveUserGroupByID(ctx context.Context, id string) (string, *cloudcasa.Usergroup, error) {
	res, err := m.cloudCasa.GetUsergroupItemWithResponse(ctx, cloudcasa.UsergroupId(id))
	if err != nil {
		return "", nil, goerr.Wrap(err, "cannot create request for CloudCasa UserGroup retrieval")
	}

	switch {
	case res.JSON200 != nil:
		return res.HTTPResponse.Header.Get("etag"), res.JSON200, nil
	case res.JSONDefault != nil:
		return "", nil, m.formatCloudCasaError(res.JSONDefault)
	default:
		return "", nil, fmt.Errorf("unhandled error for CloudCasa UserGroup retrieval")
	}
}

func (m *Manager) retrieveUserGroup(ctx context.Context, tenant *capsulev1beta2.Tenant) (etag string, userGroup *cloudcasa.Usergroup, err error) {
	id, ok := m.extractor.UserGroupID(tenant)
	if ok {
		return m.retrieveUserGroupByID(ctx, id)
	}

	return m.retrieveUserGroupFromAPI(ctx, tenant)
}

func (m *Manager) retrieveUserGroupFromAPI(ctx context.Context, tenant *capsulev1beta2.Tenant) (string, *cloudcasa.Usergroup, error) {
	where := cloudcasa.QueryWhere(fmt.Sprintf(`{"name": %q}`, tenant.GetName()))

	res, err := m.cloudCasa.Getv1usergroupsWithResponse(ctx, &cloudcasa.Getv1usergroupsParams{Where: &where})
	if err != nil {
		return "", nil, err
	}

	switch {
	case res.JSONDefault != nil:
		return "", nil, m.formatCloudCasaError(res.JSONDefault)
	case res.JSON200 != nil && len(*(res.JSON200).Items) > 1:
		return "", nil, fmt.Errorf("multiple UserGroup with the same Tenant Name, force one using the annotation %s", annotations.OrganizationAnnotation)
	case res.JSON200 != nil && len(*(res.JSON200).Items) == 1:
		userGroup := (*res.JSON200.Items)[0]

		return m.retrieveUserGroupByID(ctx, *userGroup.Id)
	case res.JSON200 != nil && len(*(res.JSON200).Items) == 0:
		return m.createUserGroup(ctx, tenant)
	default:
		return "", nil, fmt.Errorf("unhandled condition for UserGroup retrieval")
	}
}

func (m *Manager) createUserGroup(ctx context.Context, tenant *capsulev1beta2.Tenant) (string, *cloudcasa.Usergroup, error) {
	res, err := m.cloudCasa.Postv1usergroupsWithResponse(ctx, cloudcasa.Postv1usergroupsJSONRequestBody{
		Id: nil,
		Acls: &[]cloudcasa.UserGroupACL{
			{
				Permissions: &[]string{
					"policies.create",
					"kubebackups.create",
					"kuberestores.create",
					"kubeclusters.backup",
					"kubeclusters.restore",
				},
				Resource: "allresources",
			},
		},
		Name:  tenant.GetName(),
		Tags:  nil,
		Users: nil,
	})
	if err != nil {
		return "", nil, err
	}

	if resErr := res.JSONDefault; err != nil {
		return "", nil, m.formatCloudCasaError(resErr)
	}

	return m.retrieveUserGroupFromAPI(ctx, tenant)
}

func (m *Manager) ensureUserGroup(ctx context.Context, tenant *capsulev1beta2.Tenant) error {
	_, userGroup, err := m.retrieveUserGroup(ctx, tenant)
	if err != nil {
		return err
	}

	if userGroup != nil {
		return nil
	}

	return err
}

func (m *Manager) createInvitation(ctx context.Context, email, userGroupID, tenant string) error {
	res, err := m.cloudCasa.Postv1orginvitesWithResponse(ctx, cloudcasa.Postv1orginvitesJSONRequestBody{
		Acls: &[]struct {
			Permissions *[]string `json:"permissions,omitempty"`
			Resource    string    `json:"resource"`
			ResourceIds *[]string `json:"resourceIds,omitempty"`
			Roles       *[]struct {
				Id   *string `json:"_id,omitempty"`
				Name *string `json:"name,omitempty"`
				Type *string `json:"type,omitempty"`
			} `json:"roles,omitempty"`
		}{
			{
				Resource: "allresources",
			},
		},
		Email:     email,
		FirstName: email,
		LastName:  tenant,
		Name:      email,
		Tags: &map[string]interface{}{
			"capsule-clastix-io-tenant": tenant,
		},
		Usergroups: &[]string{
			userGroupID,
		},
	})
	if err != nil {
		return goerr.Wrap(err, "cannot create CloudCasa invitation for user")
	}

	if jsonErr := res.JSONDefault; jsonErr.Status != "OK" {
		return m.formatCloudCasaError(jsonErr)
	}

	return nil
}

func (m *Manager) ensureKubernetesNamespaces(ctx context.Context, tenant *capsulev1beta2.Tenant) error {
	clusterID, ok := m.extractor.ClusterID(tenant)
	if !ok {
		return fmt.Errorf("missing CloudCasa Cluster ID annotation")
	}

	ids := []string{}

	for _, namespace := range tenant.Status.Namespaces {
		ns := &corev1.Namespace{}

		if err := m.client.Get(ctx, types.NamespacedName{Name: namespace}, ns); err != nil {
			return goerr.Wrap(err, "cannot retrieve Namespace for CloudCasa")
		}

		id, err := m.ensureKubernetesNamespace(ctx, ns.GetName())
		if err != nil {
			log.FromContext(ctx).Error(err, "cannot ensure Tenant Namespaces in CloudCasa")

			continue
		}

		ids = append(ids, id)
	}

	etag, userGroup, err := m.retrieveUserGroup(ctx, tenant)
	if err != nil {
		return goerr.Wrap(err, "cannot retrieve CloudCasa UserGroup for update")
	}

	if len(*userGroup.Acls) == 0 {
		return fmt.Errorf("no ACLs for the current user group")
	}

	organizationID := m.extractor.OrganizationID(tenant)
	if len(organizationID) == 0 {
		organizationID, err = m.retrieveFirstOrganizationID(ctx)
		if err != nil {
			return fmt.Errorf("cannot retrieve the correct Organization ID : %w", err)
		}
	}

	res, err := m.cloudCasa.UpdateUserGroupACLWithResponse(ctx, cloudcasa.UsergroupId(*userGroup.Id), &cloudcasa.UpdateUserGroupACLParams{IfMatch: cloudcasa.IfMatch(etag)}, cloudcasa.UpdateUserGroupACLJSONRequestBody{
		Acls: &[]cloudcasa.ACL{
			{
				Permissions: &[]string{
					"policies.create",
					"kubebackups.create",
					"kuberestores.create",
				},
				Resource: "allresources",
			},
			{
				Permissions: &[]string{
					"kubeclusters.backup",
					"kubeclusters.restore",
				},
				Resource:    "kubeclusters",
				ResourceIds: &[]string{clusterID},
			},
			{
				Permissions: &[]string{"kubenamespaces.read"},
				Resource:    "kubenamespaces",
				ResourceIds: &ids,
			},
		},
	})
	if err != nil {
		return fmt.Errorf("cannot update UserGroup ACL : %w", err)
	}

	if res.JSON200 == nil {
		return fmt.Errorf("expected successful operation from UserGroup ACL")
	}

	return nil
}

func (m *Manager) retrieveKubernetesNamespace(ctx context.Context, name string) (id string, err error) {
	where := cloudcasa.QueryWhere(fmt.Sprintf(`{"name": %q}`, name))

	res, err := m.cloudCasa.Getv1kubenamespacesWithResponse(ctx, &cloudcasa.Getv1kubenamespacesParams{
		Where: &where,
	})
	if err != nil {
		return "", goerr.Wrap(err, "cannot create request for Kubernetes Namespace retrieval on CloudCasa")
	}

	if jsonErr := res.JSONDefault; jsonErr != nil {
		return "", m.formatCloudCasaError(jsonErr)
	}

	items := *res.JSON200.Items

	if len(items) == 0 {
		return "", nil
	}

	return *items[0].Id, nil
}

func (m *Manager) ensureKubernetesNamespace(ctx context.Context, name string) (id string, err error) {
	id, err = m.retrieveKubernetesNamespace(ctx, name)
	if err != nil {
		return "", err
	}
	// Kubernetes Namespace exists on CloudCasa
	if len(id) > 0 {
		return id, nil
	}
	return "", fmt.Errorf("Kubernetes Namespace still not present in CloudCasa, enquing back the request")
}

func (m *Manager) retrieveFirstOrganizationID(ctx context.Context) (string, error) {
	res, err := m.cloudCasa.Getv1orgsWithResponse(ctx, &cloudcasa.Getv1orgsParams{})
	if err != nil {
		return "", err
	}

	response := res.JSON200
	switch {
	case response == nil:
		return "", fmt.Errorf("missing respone")
	case response.Items != nil && len(*response.Items) > 1:
		return "", fmt.Errorf("cannot pick the correct Organization, define it in the Tenant annotation using the key %s", annotations.OrganizationAnnotation)
	case response.Items != nil && len(*response.Items) == 1:
		return string(*(*response.Items)[0].Id), nil
	default:
		return "", fmt.Errorf("unknown condition")
	}
}
