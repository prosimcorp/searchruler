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
	"prosimcorp.com/SearchRuler/internal/globals"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1alpha1 "prosimcorp.com/SearchRuler/api/v1alpha1"
)

// UpdateConditionSuccess updates the status of the QueryConnector resource with a success condition
func (r *QueryConnectorReconciler) UpdateConditionSuccess(QueryConnector *v1alpha1.QueryConnector) {

	// Create the new condition with the success status
	condition := globals.NewCondition(globals.ConditionTypeResourceSynced, metav1.ConditionTrue,
		globals.ConditionReasonTargetSynced, globals.ConditionReasonTargetSyncedMessage)

	// Update the status of the QueryConnector resource
	globals.UpdateCondition(&QueryConnector.Status.Conditions, condition)
}

// UpdateConditionKubernetesApiCallFailure updates the status of the QueryConnector resource with a failure condition
func (r *QueryConnectorReconciler) UpdateConditionKubernetesApiCallFailure(QueryConnector *v1alpha1.QueryConnector) {

	// Create the new condition with the failure status
	condition := globals.NewCondition(globals.ConditionTypeResourceSynced, metav1.ConditionTrue,
		globals.ConditionReasonKubernetesApiCallErrorType, globals.ConditionReasonKubernetesApiCallErrorMessage)

	// Update the status of the QueryConnector resource
	globals.UpdateCondition(&QueryConnector.Status.Conditions, condition)
}

// UpdateStateSuccess updates the status of the QueryConnector resource with a Success condition
func (r *QueryConnectorReconciler) UpdateStateSuccess(QueryConnector *v1alpha1.QueryConnector) {

	// Create the new condition with the success status
	condition := globals.NewCondition(globals.ConditionTypeState, metav1.ConditionTrue,
		globals.ConditionReasonStateSuccessType, globals.ConditionReasonStateSuccessMessage)

	// Update the status of the QueryConnector resource
	globals.UpdateCondition(&QueryConnector.Status.Conditions, condition)
}

// UpdateConditionNoCredsFound updates the status of the QueryConnector resource with a NoCreds condition
func (r *QueryConnectorReconciler) UpdateConditionNoCredsFound(QueryConnector *v1alpha1.QueryConnector) {

	// Create the new condition with the success status
	condition := globals.NewCondition(globals.ConditionTypeState, metav1.ConditionTrue,
		globals.ConditionReasonNoCredsFoundType, globals.ConditionReasonNoCredsFoundMessage)

	// Update the status of the QueryConnector resource
	globals.UpdateCondition(&QueryConnector.Status.Conditions, condition)
}
