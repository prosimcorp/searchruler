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

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"prosimcorp.com/SearchRuler/internal/pools"
)

var (
	resourceNamespace string
	resourceName      string
	secretName        string
	secretNamespace   string
	secretKeyUsername string
	secretKeyPassword string
)

// Sync function is used to synchronize the QueryConnector resource with the credentials. Adds the credentials to the
// credentials pool to be used in SearchRule resources. Just executed when the resource has a secretRef defined.
func (r *QueryConnectorReconciler) Sync(ctx context.Context, eventType watch.EventType, resource *CompoundQueryConnectorResource, resourceType string) (err error) {

	// Get the resource values depending on the resourceType
	switch resourceType {
	case ClusterQueryConnectorResourceType:
		resourceNamespace = ""
		resourceName = resource.ClusterQueryConnectorResource.Name
		secretName = resource.ClusterQueryConnectorResource.Spec.Credentials.SecretRef.Name
		secretNamespace = resource.ClusterQueryConnectorResource.Spec.Credentials.SecretRef.Namespace
		if secretNamespace == "" {
			secretNamespace = "default"
		}
		secretKeyUsername = resource.ClusterQueryConnectorResource.Spec.Credentials.SecretRef.KeyUsername
		secretKeyPassword = resource.ClusterQueryConnectorResource.Spec.Credentials.SecretRef.KeyPassword
	case QueryConnectorResourceType:
		resourceNamespace = resource.QueryConnectorResource.Namespace
		resourceName = resource.QueryConnectorResource.Name
		secretName = resource.QueryConnectorResource.Spec.Credentials.SecretRef.Name
		secretNamespace = resource.QueryConnectorResource.Spec.Credentials.SecretRef.Namespace
		if secretNamespace == "" {
			secretNamespace = resourceNamespace
		}
		secretKeyUsername = resource.QueryConnectorResource.Spec.Credentials.SecretRef.KeyUsername
		secretKeyPassword = resource.QueryConnectorResource.Spec.Credentials.SecretRef.KeyPassword
	}

	// If the eventType is Deleted, remove the credentials from the pool
	// In other cases get the credentials from the secret and add them to the pool
	if eventType == watch.Deleted {
		credentialsKey := fmt.Sprintf("%s_%s", resourceNamespace, resourceName)
		r.CredentialsPool.Delete(credentialsKey)
		return nil
	}

	// Get credentials for the queryConnector in the secret associated
	// First get secret with the credentials. The secret must be in the same
	// namespace as the QueryConnector resource.
	QueryConnectorCredsSecret := &v1.Secret{}
	namespacedName := types.NamespacedName{
		Namespace: secretNamespace,
		Name:      secretName,
	}
	err = r.Get(ctx, namespacedName, QueryConnectorCredsSecret)
	if err != nil {
		// Updates status to NoCredsFound
		r.UpdateConditionNoCredsFound(resource, resourceType)
		return fmt.Errorf(SecretNotFoundErrorMessage, namespacedName, err)
	}

	// Get username and password from the secret data
	username := string(QueryConnectorCredsSecret.Data[secretKeyUsername])
	password := string(QueryConnectorCredsSecret.Data[secretKeyPassword])

	// If username or password are empty, return an error
	if username == "" || password == "" {
		// Updates status to NoCredsFound
		r.UpdateConditionNoCredsFound(resource, resourceType)
		return fmt.Errorf(MissingCredentialsMessage, namespacedName)
	}

	// Save credentials in the credentials pool
	key := fmt.Sprintf("%s_%s", resourceNamespace, resourceName)
	r.CredentialsPool.Set(key, &pools.Credentials{
		Username: username,
		Password: password,
	})

	// Updates status to Success
	r.UpdateStateSuccess(resource, resourceType)
	return nil
}
