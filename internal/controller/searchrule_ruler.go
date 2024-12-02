package controller

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/log"

	"k8s.io/apimachinery/pkg/types"
	"prosimcorp.com/SearchRuler/api/v1alpha1"

	"github.com/tidwall/gjson"
)

const (
	ruleHealthyState         = "healthy"
	ruleFiringState          = "firing"
	rulePendingFiringState   = "pending_firing"
	rulePendingResolvedState = "pending_resolved"

	conditionGreaterThan        = "greaterThan"
	conditionGreaterThanOrEqual = "greaterThanOrEqual"
	conditionLessThan           = "lessThan"
	conditionLessThanOrEqual    = "lessThanOrEqual"
	conditionEqual              = "equal"
)

var (
	queryConnectorCreds *Credentials
	credsExists         bool
)

// DeleteAlertFromPool deletes the rule from the pool
func (r *SearchRuleReconciler) DeleteRuleFromPool(ctx context.Context, resource *v1alpha1.SearchRule) (err error) {
	alertKey := fmt.Sprintf("%s/%s", resource.Namespace, resource.Name)
	_, alertExists := SearchRulePool.Get(alertKey)
	if alertExists {
		SearchRulePool.Delete(alertKey)
	}
	return nil
}

// evaluateCondition evaluates the conditionField with the operator and threshold
func evaluateCondition(value float64, operator string, threshold string) (bool, error) {

	// Parse threshold to float
	floatThreshold, err := strconv.ParseFloat(threshold, 64)
	if err != nil {
		return false, fmt.Errorf("configured threshold is not a valid float: %v", threshold)
	}

	// Evaluate condition
	switch operator {
	case conditionGreaterThan:
		return value > floatThreshold, nil
	case conditionGreaterThanOrEqual:
		return value >= floatThreshold, nil
	case conditionLessThan:
		return value < floatThreshold, nil
	case conditionLessThanOrEqual:
		return value <= floatThreshold, nil
	case conditionEqual:
		return value == floatThreshold, nil
	default:
		return false, fmt.Errorf("unknown configured operator: %q", operator)
	}
}

// CheckRule execute the query to the elasticsearch and evaluate the condition. Then trigger the action
func (r *SearchRuleReconciler) CheckRule(ctx context.Context, resource *v1alpha1.SearchRule) (err error) {

	logger := log.FromContext(ctx)

	// Get QueryConnector associated to the rule
	QueryConnectorResource := &v1alpha1.QueryConnector{}
	QueryConnectorNamespacedName := types.NamespacedName{
		Namespace: resource.Namespace,
		Name:      resource.Spec.QueryConnectorRef.Name,
	}
	err = r.Get(ctx, QueryConnectorNamespacedName, QueryConnectorResource)
	if QueryConnectorResource.Name == "" {
		return fmt.Errorf("QueryConnector %s not found in the resource namespace %s", resource.Spec.QueryConnectorRef.Name, resource.Namespace)
	}

	// Get credentials for QueryConnector attached
	if QueryConnectorResource.Spec.Credentials.SecretRef.Name != "" {
		key := fmt.Sprintf("%s/%s", resource.Namespace, QueryConnectorResource.Name)
		queryConnectorCreds, credsExists = QueryConnectorCredentialsPool.Get(key)
		if !credsExists {
			return fmt.Errorf("credentials not found for %s", key)
		}
	}

	// Check if query is defined
	if resource.Spec.Elasticsearch.Query == nil {
		return fmt.Errorf("query not defined")
	}

	// Get elasticsearch query to execute from resource
	elasticQuery, err := json.Marshal(resource.Spec.Elasticsearch.Query)
	if err != nil {
		return fmt.Errorf("error marshalling query body: %v", err)
	}

	// Make http client for elasticsearch connection
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: QueryConnectorResource.Spec.TlsSkipVerify,
			},
		},
	}

	// Generate URL for search to elastic
	searchURL := fmt.Sprintf("%s/%s/_search", QueryConnectorResource.Spec.URL, resource.Spec.Elasticsearch.Index)
	req, err := http.NewRequest("POST", searchURL, bytes.NewBuffer(elasticQuery))
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}

	// Add headers and custom headers for elasticsearch queries
	req.Header.Set("Content-Type", "application/json")
	for key, value := range QueryConnectorResource.Spec.Headers {
		req.Header.Set(key, value)
	}

	// Add authentication if set for elasticsearch queries
	if QueryConnectorResource.Spec.Credentials.SecretRef.Name != "" {
		req.SetBasicAuth(queryConnectorCreds.Username, queryConnectorCreds.Password)
	}

	// Make request to elasticsearch
	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("error executing request %s: %v", string(elasticQuery), err)
	}
	defer resp.Body.Close()

	// Read response and check if it is ok
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response body: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("error response from Elasticsearch executing request %s: %s", string(elasticQuery), string(responseBody))
	}

	// Extract conditionField from the response field for elasticsearch
	conditionValue := gjson.Get(string(responseBody), resource.Spec.Elasticsearch.ConditionField)
	if !conditionValue.Exists() {
		return fmt.Errorf("conditionField %s not found in the response: %s", resource.Spec.Elasticsearch.ConditionField, string(responseBody))
	}

	// Evaluate condition and check if the alert is firing or not
	firing, err := evaluateCondition(conditionValue.Float(), resource.Spec.Condition.Operator, resource.Spec.Condition.Threshold)
	if err != nil {
		return fmt.Errorf("error evaluating condition: %v", err)
	}

	// Get ruleKey for the pool <namespace>/<name> and get it from the pool if exists
	// If not, create a default skeleton rule and save it to the pool
	ruleKey := fmt.Sprintf("%s/%s", resource.Namespace, resource.Name)
	rule, ruleInPool := SearchRulePool.Get(ruleKey)
	if !ruleInPool {
		rule = &Rule{
			firingTime:    time.Time{},
			state:         ruleHealthyState,
			resolvingTime: time.Time{},
		}
		SearchRulePool.Set(ruleKey, rule)
	}

	// Get `for` duration for the rules firing. When rule is firing during this for time,
	// then the rule is really ocurring and must be an alert
	forDuration, err := time.ParseDuration(resource.Spec.Condition.For)
	if err != nil {
		return fmt.Errorf("error parsing `for` time: %v", err)
	}

	// If rule is firing right now
	if firing {

		// If rule is not set as firing in the pool, set start fireTime and firing as true
		if rule.state == ruleHealthyState || rule.state == rulePendingResolvedState {
			rule.firingTime = time.Now()
			rule.state = rulePendingFiringState
			SearchRulePool.Set(ruleKey, rule)
		}

		// If rule is firing the For time and it is not notified yet, do it
		if time.Since(rule.firingTime) > forDuration && rule.state == rulePendingFiringState {
			rule.state = ruleFiringState
			SearchRulePool.Set(ruleKey, rule)

			// Log and update the rule status
			r.UpdateConditionAlertFiring(resource, "Rule is in firing state. Alert created. Current value is "+fmt.Sprintf("%v", conditionValue))
			logger.Info(fmt.Sprintf("Rule is in firing state. Alert created. Current value is %v", conditionValue))

			// TODO trigger firing action

		}

		return nil
	}

	// If alert is not firing right now and it is not in healthy state
	if !firing && rule.state != ruleHealthyState {

		// If rule is not marked as resolving in the pool, do it and set start resolvingTime now
		if rule.state != rulePendingResolvedState {
			rule.state = rulePendingResolvedState
			rule.resolvingTime = time.Now()
			SearchRulePool.Set(ruleKey, rule)
		}

		// If rule stay in resoliving state during the for time or it is not notified, mark as resolved
		if time.Since(rule.resolvingTime) > forDuration {

			// Log and update the rule status
			r.UpdateConditionAlertResolved(resource, "Rule is in resolved state. Alert resolved. Current value is "+fmt.Sprintf("%v", conditionValue))
			logger.Info(fmt.Sprintf("Rule is in resolved state. Alert resolved. Current value is %v", conditionValue))

			// TODO remove from alert pool

			// Restore rule to default values
			rule = &Rule{
				firingTime:    time.Time{},
				state:         ruleHealthyState,
				resolvingTime: time.Time{},
			}
			SearchRulePool.Set(ruleKey, rule)
		}
	}

	return nil
}
