/*
Copyright 2024.

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

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	storagev1 "storage.domain/disko/api/v1"
)

// LVMClusterReconciler reconciles a LVMCluster object
type LVMClusterReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=storage.storage.domain,resources=lvmclusters,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=storage.storage.domain,resources=lvmclusters/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=storage.storage.domain,resources=lvmclusters/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the LVMCluster object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.2/pkg/reconcile
func (r *LVMClusterReconciler) Reconcile(ctx context.Context, req ctrl.Request) (_ ctrl.Result, reterr error) {
	log := ctrl.LoggerFrom(ctx)

	cluster := &storagev1.LVMCluster{}
	if err := r.Client.Get(ctx, req.NamespacedName, cluster); err != nil {
		if apierrors.IsNotFound(err) {
			// Object not found, return.  Created objects are automatically garbage collected.
			// For additional cleanup logic use finalizers.
			return ctrl.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return ctrl.Result{}, err
	}

	log = log.WithValues("Cluster", klog.KRef(cluster.Namespace, cluster.Name))
	ctx = ctrl.LoggerInto(ctx, log)

	if cluster.Spec.Paused {
		log.Info("Reconciliation is paused for this object")
		return ctrl.Result{}, nil
	}

	// Initialize the patch helper
	patchHelper, err := patch.NewHelper(cluster, r.Client)
	if err != nil {
		return ctrl.Result{}, err
	}

	defer func() {
		// Always attempt to patch the object and status after each reconciliation.
		// Patch ObservedGeneration only if the reconciliation completed successfully
		// Having ObservedGeneration in the Status allows controllers and agents to ensure they're working with data that has been successfully reconciled by the owning controller.
		// For instance, if .metadata.generation is currently 12, but the .status.observedGeneration is 11, the object has yet to be reconciled successfully.
		patchOpts := []patch.Option{}
		if reterr == nil {
			patchOpts = append(patchOpts, patch.WithStatusObservedGeneration{})
		}
		if err := patchLVMCluster(ctx, patchHelper, cluster, patchOpts...); err != nil {
			reterr = kerrors.NewAggregate([]error{reterr, err})
		}
	}()

	// Handle deletion reconciliation loop.
	if !cluster.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, cluster)
	}

	// Add finalizer first if not set to avoid the race condition between init and delete.
	// Note: Finalizers in general can only be added when the deletionTimestamp is not set.
	if !controllerutil.ContainsFinalizer(cluster, storagev1.LVMClusterFinalizer) {
		controllerutil.AddFinalizer(cluster, storagev1.LVMClusterFinalizer)
		return ctrl.Result{}, nil
	}

	return r.reconcileNormal(ctx, cluster)
}

func (r *LVMClusterReconciler) reconcileNormal(ctx context.Context, cluster *storagev1.LVMCluster) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(4).Info("Reconcile LVMCluster")

	// fetch list of nodes
	nodes, err := fetchAllNodes(ctx, r.Client)
	if err != nil {
		log.Error(err, "Failed to fetch nodes")
		return ctrl.Result{}, err
	}

	// fetch list of LVMNodes
	lvmNodes, err := getLVMNodes(ctx, r.Client, cluster)
	if err != nil {
		log.Error(err, "Failed to fetch LVMNodes")
		return ctrl.Result{}, err
	}

	// Loop through all the Nodes
	created := false
	for _, node := range nodes {
		// Check if a matching LVMNode exists
		if findMatchingLVMNode(node, lvmNodes) == nil {
			// Create a new LVMNode
			err := r.createLVMNode(ctx, cluster, node)
			if err != nil {
				log.Error(err, "Failed to create LVMNode")
				return ctrl.Result{}, err
			}
			created = true
		}
	}
	if created {
		// If we created one or more LVMNode, then let's requeue
		log.Info("Requeueing LVMCluster")
		return ctrl.Result{Requeue: true}, nil
	}

	// update all LVMNodes
	for _, lvmNode := range lvmNodes {
		err := r.updateLVMNode(ctx, cluster, lvmNode)
		if err != nil {
			log.Error(err, "Failed to update LVMNode", "LVMNode", lvmNode.Name)
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *LVMClusterReconciler) reconcileDelete(ctx context.Context, cluster *storagev1.LVMCluster) (ctrl.Result, error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(4).Info("Reconcile delete LVMCluster")

	// fetch list of LVMNodes
	lvmNodes, err := getLVMNodes(ctx, r.Client, cluster)
	if err != nil {
		log.Error(err, "Failed to fetch LVMNodes")
		return ctrl.Result{}, err
	}

	// Delete all LVMNodes
	for _, lvmNode := range lvmNodes {
		err := r.deleteLVMNode(ctx, lvmNode)
		if err != nil {
			log.Error(err, "Failed to delete LVMNode", "LVMNode", lvmNode.Name)
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

func (r *LVMClusterReconciler) deleteLVMNode(ctx context.Context, lvmNode *storagev1.LVMNode) error {
	log := ctrl.LoggerFrom(ctx)
	log.V(4).Info("Deleting LVMNode", "LVMNode", lvmNode.Name)

	err := r.Client.Delete(ctx, lvmNode)
	if err != nil {
		return err
	}
	log.Info(fmt.Sprintf("Deleted LVMNode: %s", lvmNode.Name))
	return nil
}

func (r *LVMClusterReconciler) updateLVMNode(ctx context.Context, cluster *storagev1.LVMCluster, lvmNode *storagev1.LVMNode) error {
	log := ctrl.LoggerFrom(ctx)
	log.V(4).Info("Updating LVMNode", "LVMNode", lvmNode.Name)

	// Update lvmNode
	patchHelper, err := patch.NewHelper(lvmNode, r.Client)
	if err != nil {
		return err
	}

	lvmNode.Spec.LVMClusterName = cluster.Name
	lvmNode.Spec.Disks = cluster.Spec.Disks
	lvmNode.Spec.VolumeGroupName = cluster.Spec.VolumeGroupName

	if err := patchHelper.Patch(ctx, lvmNode); err != nil {
		return err
	}
	log.Info(fmt.Sprintf("Updated lvmNode: %s", lvmNode.Name))

	return nil
}

func findMatchingLVMNode(node corev1.Node, lvmNodes []*storagev1.LVMNode) *storagev1.LVMNode {
	for _, lvmNode := range lvmNodes {
		if lvmNode.Spec.NodeRef != nil && lvmNode.Spec.NodeRef.Name == node.Name {
			return lvmNode
		}
	}
	return nil
}

func (r *LVMClusterReconciler) createLVMNode(ctx context.Context, cluster *storagev1.LVMCluster, node corev1.Node) error {
	log := ctrl.LoggerFrom(ctx)
	log.V(4).Info("Creating LVMNode")
	lvmNode := &storagev1.LVMNode{
		ObjectMeta: metav1.ObjectMeta{
			Name:      node.Name,
			Namespace: cluster.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				{
					APIVersion: cluster.APIVersion,
					Kind:       cluster.Kind,
					Name:       cluster.Name,
					UID:        cluster.UID,
				},
			},
		},
		Spec: storagev1.LVMNodeSpec{
			LVMClusterName: cluster.Name,
			NodeRef: &corev1.ObjectReference{
				APIVersion: corev1.SchemeGroupVersion.String(),
				Kind:       node.Kind,
				Name:       node.Name,
				UID:        node.UID,
			},
		},
	}
	// Create lvmNode in k8s
	// TODO: Consider using the CAPI approach for Machine create.
	if err := r.Client.Create(ctx, lvmNode); err != nil {
		log.Error(err, "Failed to create LVMNode")
		return err
	}
	return nil
}

func getLVMNodes(ctx context.Context, c client.Client, cluster *storagev1.LVMCluster) ([]*storagev1.LVMNode, error) {
	log := ctrl.LoggerFrom(ctx)
	log.V(4).Info("Fetching LVMNodes for LVMCluster")

	lvmNodeList := &storagev1.LVMNodeList{}
	if err := c.List(ctx, lvmNodeList, client.InNamespace(cluster.Namespace)); err != nil {
		return nil, err
	}

	var lvmNodes []*storagev1.LVMNode
	for i := range lvmNodeList.Items {
		lvmNode := &lvmNodeList.Items[i]
		if lvmNode.OwnerReferences != nil && lvmNode.OwnerReferences[0].UID == cluster.UID {
			lvmNodes = append(lvmNodes, lvmNode)
		}
	}

	return lvmNodes, nil
}

// fetchAllNodes fetches all Nodes in a Kubernetes cluster.
func fetchAllNodes(ctx context.Context, c client.Client) ([]corev1.Node, error) {
	nodeList := &corev1.NodeList{}
	if err := c.List(ctx, nodeList); err != nil {
		return nil, err
	}

	var nodes []corev1.Node
	for _, node := range nodeList.Items {
		nodes = append(nodes, node)
	}

	return nodes, nil
}

func patchLVMCluster(ctx context.Context, patchHelper *patch.Helper, cluster *storagev1.LVMCluster, options ...patch.Option) error {
	// Always update the readyCondition by summarizing the state of other conditions.
	conditions.SetSummary(cluster,
		conditions.WithConditions(
			storagev1.LVMAvailableCondition,
		),
	)

	// Patch the object, ignoring conflicts on the conditions owned by this controller.
	options = append(options,
		patch.WithOwnedConditions{Conditions: []clusterv1.ConditionType{
			storagev1.ReadyCondition,
		}},
	)
	return patchHelper.Patch(ctx, cluster, options...)
}

// SetupWithManager sets up the controller with the Manager.
func (r *LVMClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&storagev1.LVMCluster{}).
		Complete(r)
}
