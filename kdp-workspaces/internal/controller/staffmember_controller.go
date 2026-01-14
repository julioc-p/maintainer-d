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

package controller

import (
	"context"
	"fmt"
	"time"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/cncf/maintainer-d/kdp-workspaces/internal/kcp"
)

const (
	// AnnotationStaffLastSynced tracks when this staff member was last synced to workspaces
	AnnotationStaffLastSynced = "kdp-workspaces.cncf.io/last-synced"

	// AnnotationStaffSyncStatus tracks sync status: "success", "partial", "error"
	AnnotationStaffSyncStatus = "kdp-workspaces.cncf.io/sync-status"

	// AnnotationStaffWorkspaceCount tracks number of workspaces this member has access to
	AnnotationStaffWorkspaceCount = "kdp-workspaces.cncf.io/workspace-count"
)

// StaffMemberReconciler reconciles maintainer-d StaffMember objects and manages staff access across workspaces
type StaffMemberReconciler struct {
	client.Client
	Scheme *runtime.Scheme

	// KCP configuration
	KCPConfigMapName      string
	KCPConfigMapNamespace string
	KCPSecretName         string
	KCPSecretNamespace    string

	// StaffMember namespace
	StaffMemberNamespace string
}

// +kubebuilder:rbac:groups=maintainer-d.cncf.io,resources=staffmembers,verbs=get;list;watch;update;patch
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch

// Reconcile handles the reconciliation of StaffMember resources
func (r *StaffMemberReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling StaffMember change", "staffmember", req.Name)

	// Load kcp configuration from ConfigMap and Secret
	kcpConfig, err := kcp.LoadConfigFromCluster(
		ctx,
		r.Client,
		r.KCPConfigMapName,
		r.KCPSecretName,
		r.KCPConfigMapNamespace,
	)
	if err != nil {
		logger.Error(err, "Failed to load kcp configuration")
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, err
	}

	// Create kcp client
	kcpClient, err := kcp.NewClient(kcpConfig)
	if err != nil {
		logger.Error(err, "Failed to create kcp client")
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, err
	}

	// List all managed workspaces
	workspaces, err := kcpClient.ListManagedWorkspaces(ctx)
	if err != nil {
		logger.Error(err, "Failed to list managed workspaces")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, err
	}

	logger.Info("Found managed workspaces", "count", len(workspaces))

	// Get current list of ALL staff members
	staffList := &unstructured.UnstructuredList{}
	staffList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "maintainer-d.cncf.io",
		Version: "v1alpha1",
		Kind:    "StaffMemberList",
	})

	if err := r.List(ctx, staffList,
		client.InNamespace(r.StaffMemberNamespace)); err != nil {
		logger.Error(err, "Failed to list staff members")
		return ctrl.Result{}, fmt.Errorf("failed to list staff members: %w", err)
	}

	logger.Info("Found staff members", "count", len(staffList.Items))

	// Build subjects list from ALL staff members
	subjects := []rbacv1.Subject{}
	for _, staff := range staffList.Items {
		spec, ok := staff.Object["spec"].(map[string]any)
		if !ok {
			logger.Info("Staff member has no spec, skipping", "name", staff.GetName())
			continue
		}

		email, ok := spec["primaryEmail"].(string)
		if !ok || email == "" {
			logger.Info("Staff member has no primaryEmail, skipping", "name", staff.GetName())
			continue
		}

		subjects = append(subjects, rbacv1.Subject{
			Kind: rbacv1.UserKind,
			Name: fmt.Sprintf("oidc:%s", email),
		})
	}

	logger.Info("Built subjects list", "subjectCount", len(subjects))

	// Update binding in all workspaces
	var reconcileErrors []error
	successCount := 0
	for _, ws := range workspaces {
		logger.Info("Updating staff binding in workspace", "workspace", ws.Name)
		if err := kcpClient.CreateOrUpdateStaffBinding(ctx, ws.Name, subjects); err != nil {
			logger.Error(err, "Failed to update staff binding", "workspace", ws.Name)
			reconcileErrors = append(reconcileErrors, err)
		} else {
			successCount++
		}
	}

	// Update StaffMember annotations with sync status
	syncStatus := "success"
	if len(reconcileErrors) > 0 {
		syncStatus = "partial"
		if successCount == 0 {
			syncStatus = "error"
		}
	}

	// Update the StaffMember that triggered this reconciliation
	if err := r.updateStaffMemberAnnotations(ctx, req.Name,
		successCount, syncStatus); err != nil {
		logger.Error(err, "Failed to update StaffMember annotations",
			"staffmember", req.Name)
		// Don't fail reconciliation if annotation update fails
	}

	if len(reconcileErrors) > 0 {
		logger.Error(fmt.Errorf("failed to update %d workspaces", len(reconcileErrors)),
			"Reconciliation completed with errors")
		return ctrl.Result{RequeueAfter: 30 * time.Second},
			fmt.Errorf("failed to update %d workspaces", len(reconcileErrors))
	}

	logger.Info("Reconciliation completed successfully",
		"workspacesUpdated", successCount)
	return ctrl.Result{}, nil
}

// updateStaffMemberAnnotations updates sync metadata annotations on a StaffMember
func (r *StaffMemberReconciler) updateStaffMemberAnnotations(ctx context.Context,
	staffMemberName string, workspaceCount int, syncStatus string) error {

	// Fetch the StaffMember
	staffMember := &unstructured.Unstructured{}
	staffMember.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "maintainer-d.cncf.io",
		Version: "v1alpha1",
		Kind:    "StaffMember",
	})

	if err := r.Get(ctx, client.ObjectKey{
		Name:      staffMemberName,
		Namespace: r.StaffMemberNamespace,
	}, staffMember); err != nil {
		if errors.IsNotFound(err) {
			// StaffMember was deleted, nothing to update
			return nil
		}
		return err
	}

	// Create patch
	patch := client.MergeFrom(staffMember.DeepCopy())

	// Update annotations
	annotations := staffMember.GetAnnotations()
	if annotations == nil {
		annotations = make(map[string]string)
	}
	annotations[AnnotationStaffLastSynced] = time.Now().Format(time.RFC3339)
	annotations[AnnotationStaffSyncStatus] = syncStatus
	annotations[AnnotationStaffWorkspaceCount] = fmt.Sprintf("%d", workspaceCount)
	staffMember.SetAnnotations(annotations)

	// Apply patch
	return r.Patch(ctx, staffMember, patch)
}

// SetupWithManager sets up the controller with the Manager
func (r *StaffMemberReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Create a metadata-only object for watching
	staffMember := &metav1.PartialObjectMetadata{}
	staffMember.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "maintainer-d.cncf.io",
		Version: "v1alpha1",
		Kind:    "StaffMember",
	})

	return ctrl.NewControllerManagedBy(mgr).
		For(staffMember).
		Complete(r)
}
