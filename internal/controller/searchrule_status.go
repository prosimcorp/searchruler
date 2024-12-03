package controller

import (
	"prosimcorp.com/SearchRuler/internal/globals"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1alpha1 "prosimcorp.com/SearchRuler/api/v1alpha1"
)

func (r *SearchRuleReconciler) UpdateConditionSuccess(searchRule *v1alpha1.SearchRule) {

	//
	condition := globals.NewCondition(globals.ConditionTypeResourceSynced, metav1.ConditionTrue,
		globals.ConditionReasonTargetSynced, globals.ConditionReasonTargetSyncedMessage)

	globals.UpdateCondition(&searchRule.Status.Conditions, condition)
}

func (r *SearchRuleReconciler) UpdateConditionKubernetesApiCallFailure(searchRule *v1alpha1.SearchRule) {

	//
	condition := globals.NewCondition(globals.ConditionTypeResourceSynced, metav1.ConditionTrue,
		globals.ConditionReasonKubernetesApiCallErrorType, globals.ConditionReasonKubernetesApiCallErrorMessage)

	globals.UpdateCondition(&searchRule.Status.Conditions, condition)
}

func (r *SearchRuleReconciler) UpdateConditionAlertFiring(searchRule *v1alpha1.SearchRule, conditionReasonAlertFiringMessage string) {

	//
	condition := globals.NewCondition(globals.ConditionTypeState, metav1.ConditionTrue,
		globals.ConditionReasonAlertFiring, conditionReasonAlertFiringMessage)

	globals.UpdateCondition(&searchRule.Status.Conditions, condition)
}

func (r *SearchRuleReconciler) UpdateConditionAlertResolved(searchRule *v1alpha1.SearchRule, conditionReasonAlertResolvedMessage string) {

	//
	condition := globals.NewCondition(globals.ConditionTypeState, metav1.ConditionTrue,
		globals.ConditionReasonAlertResolved, conditionReasonAlertResolvedMessage)

	globals.UpdateCondition(&searchRule.Status.Conditions, condition)
}
