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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/klog/v2"
	clusterv1 "sigs.k8s.io/cluster-api/api/v1beta1"
	"sigs.k8s.io/cluster-api/util/conditions"
	"sigs.k8s.io/cluster-api/util/patch"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

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

	// Ignore deleted LVMCluster, this can happen when foregroundDeletion
	// is enabled
	if !cluster.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	err = r.reconcile(ctx, cluster)
	if err != nil {
		log.Error(err, "Failed to reconcile LVMCluster")
		// r.recorder.Eventf(cluster, corev1.EventTypeWarning, "ReconcileError", "%v", err)
	}
	return ctrl.Result{}, err
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

func (r *LVMClusterReconciler) reconcile(ctx context.Context, cluster *storagev1.LVMCluster) error {
	log := ctrl.LoggerFrom(ctx)
	log.V(4).Info("Reconcile LVMCluster")

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *LVMClusterReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&storagev1.LVMCluster{}).
		Complete(r)
}
