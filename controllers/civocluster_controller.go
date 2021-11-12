/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"

	"github.com/civo/civogo"
	"github.com/go-logr/logr"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/cluster-api/util/patch"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	infrastructurev1beta1 "github.com/null-channel/cluster-api-provider-civo/api/v1beta1"
)

const (
	civoClusterFinalizer = "civocluster.infrastructure.cluster.x-k8s.io/v1beta1"
)

// CivoClusterReconciler reconciles a CivoCluster object
type CivoClusterReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	CivoClient *civogo.Client
}

//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=civoclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=civoclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=infrastructure.cluster.x-k8s.io,resources=civoclusters/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.8.3/pkg/reconcile
func (r *CivoClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	logger.Info("Starting CivoCluster reconcilation")

	civoCluster := &infrastructurev1beta1.CivoCluster{}
	if err := r.Client.Get(ctx, req.NamespacedName, civoCluster); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Persist any changes to CivoCluster.
	h, err := patch.NewHelper(civoCluster, r.Client)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("init patch helper: %w", err)
	}
	defer func() {
		if e := h.Patch(ctx, civoCluster); e != nil {
			if err != nil {
				err = fmt.Errorf("%s: %w", e.Error(), err)
			}
			err = fmt.Errorf("patch: %w", e)
		}
	}()

	// Check if owner Cluster resource exists
	cluster, err := util.GetOwnerCluster(ctx, r.Client, civoCluster.ObjectMeta)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get OwnerCluster: %w", err)
	}
	if cluster == nil {
		logger.Info("Waiting for cluster controller to set OwnerRef to CivoCluster")
		return ctrl.Result{Requeue: true}, nil
	}

	// Check if cluster is paused
	if annotations.IsPaused(cluster, civoCluster) {
		logger.Info("reconcilation is paused for this object")
		return ctrl.Result{Requeue: true}, nil
	}

	if !civoCluster.ObjectMeta.DeletionTimestamp.IsZero() {
		r.delete(ctx, logger, civoCluster)
	}

	return r.reconcile(ctx, logger, civoCluster)
}

// TODO Cluster deletion
func (r *CivoClusterReconciler) delete(ctx context.Context, logger logr.Logger, civoCluster *infrastructurev1beta1.CivoCluster) (ctrl.Result, error) {
	logger = log.FromContext(ctx)
	logger.Info("Deleting civo cluster")
	_, err := r.CivoClient.DeleteKubernetesCluster(*civoCluster.Spec.ID)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to delete cluster: %w", err)
	}
	return ctrl.Result{}, nil
}

func (r *CivoClusterReconciler) reconcile(ctx context.Context, logger logr.Logger, civoCluster *infrastructurev1beta1.CivoCluster) (ctrl.Result, error) {
	controllerutil.AddFinalizer(civoCluster, civoClusterFinalizer)

	// Create cluster
	kc, err := r.CivoClient.NewKubernetesClusters(&civoCluster.Spec.Config)
	civoCluster.Spec.ID = &kc.ID
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to create cluster: %w", err)
	}
	if kc == nil {
		return ctrl.Result{}, fmt.Errorf("not able to retrieve Contol Plane IP")
	}

	civoCluster.Spec.ControlPlaneEndpoint.Host = kc.APIEndPoint
	civoCluster.Status.Ready = true

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *CivoClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1beta1.CivoCluster{}).
		Complete(r)
}
