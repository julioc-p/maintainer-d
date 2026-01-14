/*
Copyright 2025.

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

package kcp

import (
	"context"
	"fmt"
	"time"

	"github.com/kcp-dev/logicalcluster/v3"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	// AnnotationBindingLastSynced tracks when the ClusterRoleBinding was last synced
	AnnotationBindingLastSynced = "kdp-workspaces.cncf.io/last-synced"

	// AnnotationBindingStaffCount tracks the number of staff members in the binding
	AnnotationBindingStaffCount = "kdp-workspaces.cncf.io/staff-count"

	// AnnotationBindingManagedBy identifies the operator managing this binding
	AnnotationBindingManagedBy = "kdp-workspaces.cncf.io/managed-by"

	// AnnotationBindingSourceNamespace identifies the source namespace for staff members
	AnnotationBindingSourceNamespace = "kdp-workspaces.cncf.io/source-namespace"

	// LabelManagedBy is the label key for identifying managed resources
	LabelManagedBy = "managed-by"

	// StaffAccessBindingName is the default name for staff access ClusterRoleBindings
	StaffAccessBindingName = "cncf-staff-access"

	// StaffRoleName is the ClusterRole to bind staff members to
	StaffRoleName = "kdp:owner"

	// OperatorName identifies this operator
	OperatorName = "kdp-ws-operator"
)

// CreateOrUpdateStaffBinding creates or updates a ClusterRoleBinding in the specified workspace
// with the given subjects. The binding grants the kdp:owner role to all subjects.
func (c *Client) CreateOrUpdateStaffBinding(ctx context.Context,
	workspaceName string, subjects []rbacv1.Subject) error {

	if workspaceName == "" {
		return fmt.Errorf("workspace name cannot be empty")
	}

	// Build workspace path (e.g., root:projectname)
	baseWorkspacePath := c.config.WorkspacePath
	if baseWorkspacePath == "" {
		baseWorkspacePath = "root"
	}
	workspacePath := logicalcluster.NewPath(baseWorkspacePath).Join(workspaceName)

	// Create clientset from kubeconfig
	restConfig, err := clientcmd.RESTConfigFromKubeConfig(c.config.KubeconfigData)
	if err != nil {
		return fmt.Errorf("failed to parse kubeconfig: %w", err)
	}

	// Set cluster path in the REST config
	// KCP uses a special URL parameter to target specific workspaces
	restConfig.Host = fmt.Sprintf("%s/clusters/%s", restConfig.Host, workspacePath.String())

	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	// Build ClusterRoleBinding with annotations
	binding := &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: StaffAccessBindingName,
			Annotations: map[string]string{
				AnnotationBindingLastSynced:      time.Now().Format(time.RFC3339),
				AnnotationBindingStaffCount:      fmt.Sprintf("%d", len(subjects)),
				AnnotationBindingManagedBy:       OperatorName,
				AnnotationBindingSourceNamespace: "maintainerd",
			},
			Labels: map[string]string{
				LabelManagedBy: OperatorName,
			},
		},
		Subjects: subjects,
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     StaffRoleName,
		},
	}

	// Try to get existing binding
	existingBinding, err := clientset.RbacV1().ClusterRoleBindings().Get(
		ctx, StaffAccessBindingName, metav1.GetOptions{},
	)

	if err != nil {
		if errors.IsNotFound(err) {
			// Binding doesn't exist, create it
			_, err = clientset.RbacV1().ClusterRoleBindings().Create(
				ctx, binding, metav1.CreateOptions{},
			)
			if err != nil {
				return fmt.Errorf("failed to create ClusterRoleBinding in workspace %s: %w",
					workspaceName, err)
			}
			return nil
		}
		return fmt.Errorf("failed to get ClusterRoleBinding in workspace %s: %w",
			workspaceName, err)
	}

	// Binding exists, update it
	existingBinding.Subjects = subjects
	existingBinding.Annotations[AnnotationBindingLastSynced] = time.Now().Format(time.RFC3339)
	existingBinding.Annotations[AnnotationBindingStaffCount] = fmt.Sprintf("%d", len(subjects))

	_, err = clientset.RbacV1().ClusterRoleBindings().Update(
		ctx, existingBinding, metav1.UpdateOptions{},
	)
	if err != nil {
		return fmt.Errorf("failed to update ClusterRoleBinding in workspace %s: %w",
			workspaceName, err)
	}

	return nil
}
