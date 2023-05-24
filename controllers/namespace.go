// Copyright 2022 Clastix Labs
// SPDX-License-Identifier: Apache-2.0

package controllers

import (
	"context"
	"fmt"

	capsulev1beta2 "github.com/clastix/capsule/api/v1beta2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type Namespace struct {
	client       client.Client
	capsuleLabel string
}

func (n *Namespace) SetupWithManager(mgr manager.Manager) error {
	capsuleLabel, err := capsulev1beta2.GetTypeLabel(&capsulev1beta2.Tenant{})
	if err != nil {
		return err
	}

	n.capsuleLabel = capsuleLabel

	return ctrl.NewControllerManagedBy(mgr).
		Watches(source.NewKindWithCache(&corev1.Namespace{}, mgr.GetCache()), handler.Funcs{
			CreateFunc: n.createFunc,
			DeleteFunc: n.deleteFunc,
			UpdateFunc: n.updateFunc,
		}, builder.WithPredicates(predicate.NewPredicateFuncs(func(object client.Object) bool {
			tenantName, _ := n.extractTenantFromNamespace(object)

			return len(tenantName) > 0
		}))).
		For(&capsulev1beta2.Tenant{}).
		Complete(n)
}

func (n *Namespace) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("Reconciliation started")

	tnt := &capsulev1beta2.Tenant{}
	if err := n.client.Get(ctx, request.NamespacedName, tnt); err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}

		return reconcile.Result{}, err
	}

	namespaceList := &corev1.NamespaceList{}

	if err := n.client.List(ctx, namespaceList, client.MatchingLabels(map[string]string{n.capsuleLabel: tnt.GetName()})); err != nil {
		return reconcile.Result{}, err
	}

	for _, ns := range namespaceList.Items {
		logger.Info(fmt.Sprintf("Reconciling Namespace %s", ns.GetName()))

		if err := n.reconcileNamespace(ctx, &ns, tnt); err != nil {
			logger.Error(err, "reconciliation failed")

			continue
		}
	}

	logger.Info("Reconciliation completed")

	return reconcile.Result{}, nil
}

func (n *Namespace) extractTenantFromNamespace(namespace client.Object) (string, error) {
	labels := namespace.GetLabels()
	if labels == nil {
		return "", nil
	}

	tenantName, ok := labels[n.capsuleLabel]

	if !ok {
		return "", nil
	}

	return tenantName, nil
}

func (n *Namespace) createFunc(event event.CreateEvent, queue workqueue.RateLimitingInterface) {
	tenantName, err := n.extractTenantFromNamespace(event.Object)
	if err != nil {
		return
	}

	n.addToQueue(queue, tenantName)
}

func (n *Namespace) deleteFunc(event event.DeleteEvent, queue workqueue.RateLimitingInterface) {
	tenantName, err := n.extractTenantFromNamespace(event.Object)
	if err != nil {
		return
	}

	n.addToQueue(queue, tenantName)
}

func (n *Namespace) updateFunc(event event.UpdateEvent, queue workqueue.RateLimitingInterface) {
	tenantName, err := n.extractTenantFromNamespace(event.ObjectNew)
	if err != nil {
		return
	}

	n.addToQueue(queue, tenantName)
}

func (n *Namespace) addToQueue(queue workqueue.RateLimitingInterface, tenantName string) {
	queue.Add(reconcile.Request{
		NamespacedName: types.NamespacedName{
			Name: tenantName,
		},
	})
}

func (n *Namespace) reconcileNamespace(ctx context.Context, ns *corev1.Namespace, tnt *capsulev1beta2.Tenant) error {
	_, err := controllerutil.CreateOrUpdate(ctx, n.client, ns, func() error {
		labels := ns.GetLabels()
		labels["restored"] = "true"

		return controllerutil.SetControllerReference(tnt, ns, n.client.Scheme())
	})

	return err
}

func (n *Namespace) InjectClient(client client.Client) error {
	n.client = client

	return nil
}
