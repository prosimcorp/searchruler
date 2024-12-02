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

package controller

import (
	"context"
	"fmt"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"prosimcorp.com/SearchRuler/api/v1alpha1"
	searchrulerv1alpha1 "prosimcorp.com/SearchRuler/api/v1alpha1"
)

// SearchRuleReconciler reconciles a SearchRule object
type SearchRuleReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=searchruler.prosimcorp.com,resources=searchrules,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=searchruler.prosimcorp.com,resources=searchrules/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=searchruler.prosimcorp.com,resources=searchrules/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the SearchRule object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.19.0/pkg/reconcile
func (r *SearchRuleReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {
	logger := log.FromContext(ctx)

	// 1. Get the content of the Patch
	searchRuleResource := &v1alpha1.SearchRule{}
	err = r.Get(ctx, req.NamespacedName, searchRuleResource)

	// 2. Check existence on the cluster
	if err != nil {

		// 2.1 It does NOT exist: manage removal
		if err = client.IgnoreNotFound(err); err == nil {
			logger.Info(fmt.Sprintf(resourceNotFoundError, SearchRuleResourceType, req.NamespacedName))
			return result, err
		}

		// 2.2 Failed to get the resource, requeue the request
		logger.Info(fmt.Sprintf(resourceSyncTimeRetrievalError, SearchRuleResourceType, req.NamespacedName, err.Error()))
		return result, err
	}

	// 3. Check if the SearchRule instance is marked to be deleted: indicated by the deletion timestamp being set
	if !searchRuleResource.DeletionTimestamp.IsZero() {
		if controllerutil.ContainsFinalizer(searchRuleResource, resourceFinalizer) {
			// Remove the finalizers on Patch CR
			controllerutil.RemoveFinalizer(searchRuleResource, resourceFinalizer)
			err = r.Update(ctx, searchRuleResource)
			if err != nil {
				logger.Info(fmt.Sprintf(resourceFinalizersUpdateError, SearchRuleResourceType, req.NamespacedName, err.Error()))
			}
		}
		// Delete credentials from pool
		err = r.DeleteRuleFromPool(ctx, searchRuleResource)
		if err != nil {
			logger.Info(fmt.Sprintf("error deleting rule from pool: %v", err.Error()))
		}

		result = ctrl.Result{}
		err = nil
		return result, err
	}

	// 4. Add finalizer to the SearchRule CR
	if !controllerutil.ContainsFinalizer(searchRuleResource, resourceFinalizer) {
		controllerutil.AddFinalizer(searchRuleResource, resourceFinalizer)
		err = r.Update(ctx, searchRuleResource)
		if err != nil {
			return result, err
		}
	}

	// 5. Update the status before the requeue
	defer func() {
		err = r.Status().Update(ctx, searchRuleResource)
		if err != nil {
			logger.Info(fmt.Sprintf(resourceConditionUpdateError, SearchRuleResourceType, req.NamespacedName, err.Error()))
		}
	}()

	// 6. Schedule periodical request
	RequeueTime, err := time.ParseDuration(searchRuleResource.Spec.CheckInterval)
	if err != nil {
		logger.Info(fmt.Sprintf(resourceSyncTimeRetrievalError, SearchRuleResourceType, req.NamespacedName, err.Error()))
		return result, err
	}
	result = ctrl.Result{
		RequeueAfter: RequeueTime,
	}

	// 7. Check the rule
	err = r.CheckRule(ctx, searchRuleResource)
	if err != nil {
		r.UpdateConditionKubernetesApiCallFailure(searchRuleResource)
		logger.Info(fmt.Sprintf(syncTargetError, SearchRuleResourceType, req.NamespacedName, err.Error()))
		return result, err
	}

	// 8. Success, update the status
	r.UpdateConditionSuccess(searchRuleResource)

	return result, err

}

// SetupWithManager sets up the controller with the Manager.
func (r *SearchRuleReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&searchrulerv1alpha1.SearchRule{}).
		Named("searchrule").
		Complete(r)
}
