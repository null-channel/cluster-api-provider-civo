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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"regexp"
	"net/url"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	"github.com/civo/civogo"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"github.com/go-logr/logr"
	"sigs.k8s.io/cluster-api/util"
	"sigs.k8s.io/cluster-api/util/annotations"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	capiv1beta1 "sigs.k8s.io/cluster-api/api/v1beta1"
	capiremote "sigs.k8s.io/cluster-api/controllers/remote"

	infrastructurev1beta1 "github.com/null-channel/cluster-api-provider-civo/api/v1beta1"
)

const (
	civoClusterFinalizer = "civocluster.infrastructure.cluster.x-k8s.io/v1beta1"
)

// CivoClusterReconciler reconciles a CivoCluster object
type CivoClusterReconciler struct {
	client.Client
	Scheme     		 *runtime.Scheme
	CivoClient 		 *civogo.Client
	ClusterTracker   *capiremote.ClusterCacheTracker
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

	defer func() {
		r.Client.Update(ctx, civoCluster)
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

	return r.reconcile(ctx, logger, cluster, civoCluster)
}

func (r *CivoClusterReconciler) delete(ctx context.Context, logger logr.Logger, civoCluster *infrastructurev1beta1.CivoCluster) (ctrl.Result, error) {
	logger = log.FromContext(ctx)
	logger.Info("Deleting civo cluster")
	_, err := r.CivoClient.DeleteKubernetesCluster(*civoCluster.Spec.ID)
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to delete cluster: %w", err)
	}
	controllerutil.RemoveFinalizer(civoCluster, civoClusterFinalizer)
	return ctrl.Result{}, nil
}

func (r *CivoClusterReconciler) reconcile(
	ctx context.Context, logger logr.Logger, cluster *capiv1beta1.Cluster, civoCluster *infrastructurev1beta1.CivoCluster,
) (ctrl.Result, error) {
	controllerutil.AddFinalizer(civoCluster, civoClusterFinalizer)

	if civoCluster.Spec.ID == nil {
		// Create cluster
		kc, err := r.CivoClient.NewKubernetesClusters(infrastructurev1beta1.ToCivoKubernetesStruct(&civoCluster.Spec.Config))
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to create cluster: %w", err)
		}
		if kc == nil {
			return ctrl.Result{}, fmt.Errorf("not able to retrieve Contol Plane IP")
		}

		fmt.Println("===========================================================")
		fmt.Println(kc.ID)

		civoCluster.Spec.ID = &kc.ID
		return ctrl.Result{Requeue: true, RequeueAfter: time.Minute * 1}, nil
	}

	// Update cluster
	kc, err := r.CivoClient.UpdateKubernetesCluster(*civoCluster.Spec.ID, infrastructurev1beta1.ToCivoKubernetesStruct(&civoCluster.Spec.Config))
	if err != nil {
		return ctrl.Result{Requeue: true, RequeueAfter: time.Minute * 1}, fmt.Errorf("failed to update cluster: %w", err)
	}
	if kc == nil {
		return ctrl.Result{}, fmt.Errorf("not able to retrieve Contol Plane IP")
	}

	if !kc.Ready {
		kc, _ := r.CivoClient.GetKubernetesCluster(*civoCluster.Spec.ID)

		if kc.Ready && civoCluster.Spec.ControlPlaneEndpoint.Host == "" {
			node, err := r.getNode(ctx, cluster, kc)
			if err != nil {
				return ctrl.Result{Requeue: true, RequeueAfter: time.Second * 10}, nil
			}

			re := regexp.MustCompile(`(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)(\.(25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)){3}`)
			civoCluster.Spec.ControlPlaneEndpoint.Host = re.FindString(kc.APIEndPoint)
			civoCluster.Spec.ControlPlaneEndpoint.Port = 6443

			if err := r.createControlPlaneMachine(ctx, cluster, civoCluster, node); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to create control plane: %w", err)
			}

			return ctrl.Result{Requeue: true}, nil
		} else {
			host, port := stringToHostPort(kc.APIEndPoint)
			civoCluster.Spec.ControlPlaneEndpoint.Host = host
			civoCluster.Spec.ControlPlaneEndpoint.Port = port
			civoCluster.Status.Ready = true
			r.Client.Status().Update(ctx, civoCluster)
		}

		return ctrl.Result{Requeue: true, RequeueAfter: time.Minute * 1}, nil
	} else {
		fmt.Println("Cluster Ready.")
		host, port := stringToHostPort(kc.APIEndPoint)
		civoCluster.Spec.ControlPlaneEndpoint.Host = host
		civoCluster.Spec.ControlPlaneEndpoint.Port = port
		civoCluster.Status.Ready = true
		r.Client.Status().Update(ctx, civoCluster)
	}

	return ctrl.Result{}, nil
}

func (r *CivoClusterReconciler) createControlPlaneMachine(
	ctx context.Context, cluster *capiv1beta1.Cluster, civoCluster *infrastructurev1beta1.CivoCluster, node *corev1.Node,
) error  {
	civoMachine := &infrastructurev1beta1.CivoMachine{
		ObjectMeta: metav1.ObjectMeta{
			Name: "control-plane-" + civoCluster.Name,
			Namespace: civoCluster.Namespace,
		},
		Spec: infrastructurev1beta1.CivoMachineSpec{},
	}

	if err := r.Create(ctx, civoMachine); err != nil {
		return fmt.Errorf("failed to create CivoMachine: %w", err)
	}
	civoMachine.Status.Ready = true
	r.Status().Update(ctx, civoMachine)

	emptySecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: "empty-secret-" + cluster.Name,
			Namespace: cluster.Namespace,
		},
	}
	if err := r.Create(ctx, emptySecret); err != nil {
		return fmt.Errorf("failed to create bootstrap secret: %w", err)
	}

	machine := &capiv1beta1.Machine{
		ObjectMeta: metav1.ObjectMeta{
			Name: "control-plane-" + civoCluster.Name,
			Namespace: civoCluster.Namespace,
			Labels: map[string]string{
				capiv1beta1.ClusterLabelName: cluster.Name,
				capiv1beta1.MachineControlPlaneLabelName: "true",
			},
		},
		Spec: capiv1beta1.MachineSpec{
				ClusterName: cluster.Name,
				InfrastructureRef: corev1.ObjectReference{
					Kind: "CivoMachine",
					Name: civoMachine.Name,
					APIVersion: "infrastructure.cluster.x-k8s.io/v1beta1",
				},
				Bootstrap: capiv1beta1.Bootstrap{
					DataSecretName: &emptySecret.Name,
				},
				ProviderID: &node.Spec.ProviderID,
		},
	}
	if err := r.Create(ctx, machine); err != nil {
		return fmt.Errorf("failed to create Machine: %w", err)
	}
	machine.Status.BootstrapReady = true
	machine.Status.InfrastructureReady = true
	machine.Status.NodeRef = &corev1.ObjectReference{
		Kind: "Node",
		Namespace: node.Namespace,
		Name: node.Name,
	}
	r.Status().Update(ctx, machine)

	return nil
}

func (r *CivoClusterReconciler) getNode(ctx context.Context, cluster *capiv1beta1.Cluster, config *civogo.KubernetesCluster) (*corev1.Node, error) {
	if err := r.Get(ctx, types.NamespacedName{Namespace: cluster.Namespace, Name: cluster.Name + "-kubeconfig"}, &corev1.Secret{}); err != nil {
		if apierrors.IsNotFound(err) {
			kubeconfigSecret := &corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name: cluster.Name + "-kubeconfig",
					Namespace: cluster.Namespace,
				},
				Data: map[string][]byte{"value": []byte(config.KubeConfig)},
			}
			if err := r.Create(ctx, kubeconfigSecret); err != nil {
				return nil, fmt.Errorf("failed to create kubeconfig secret: %w", err)
			}
		} else {
			return nil, err
		}
	}

	remoteClient, err := r.ClusterTracker.GetClient(ctx, types.NamespacedName{Namespace: cluster.Namespace, Name: cluster.Name})
	if err != nil {
		return nil, err
	}

	nodeList := corev1.NodeList{}
	if err := remoteClient.List(ctx, &nodeList); err != nil {
		return nil, err
	}
	if len(nodeList.Items) == 0 {
		return nil, fmt.Errorf("didn't found any nodes")
	}

	return &nodeList.Items[0], nil
}
  
func stringToHostPort(hostPort string) (string, int32) {
	v, err := url.Parse(hostPort)
	if err != nil {
		return "", 0
	}
	i, err := strconv.Atoi(v.Port())
	if err != nil {
		return "", 0
	}
	return v.Hostname(), int32(i)
}

// SetupWithManager sets up the controller with the Manager.
func (r *CivoClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&infrastructurev1beta1.CivoCluster{}).
		Complete(r)
}
