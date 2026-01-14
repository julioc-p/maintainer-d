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
	"errors"
	"fmt"
	"time"

	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/cncf/maintainer-d/kdp-workspaces/internal/kcp"
)

const (
	// AnnotationStaffLastSynced tracks when this staff member was last synced to workspaces
	AnnotationStaffLastSynced = "kdp-workspaces.cncf.io/last-synced"

	// AnnotationStaffSyncStatus tracks sync status: "success", "partial", "error"
	AnnotationStaffSyncStatus = "kdp-workspaces.cncf.io/sync-status"

	// AnnotationStaffWorkspaceCount tracks number of workspaces this member has access to
	AnnotationStaffWorkspaceCount = "kdp-workspaces.cncf.io/workspace-count"

	// workspaceTrigger is a special name for reconciliation requests triggered by a project/workspace change
	workspaceTrigger = "workspace-trigger"
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
// +kubebuilder:rbac:groups=maintainer-d.cncf.io,resources=projects,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch
// +kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch

// Reconcile handles the reconciliation of StaffMember resources
func (r *StaffMemberReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconciling", "request", req.Name)

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
	var workspaceErrors []error
	var failedWorkspaces []string
	successCount := 0
	totalWorkspaces := len(workspaces)

	for _, ws := range workspaces {
		logger.Info("Updating staff binding in workspace", "workspace", ws.Name)
		if err := kcpClient.CreateOrUpdateStaffBinding(ctx, ws.Name, subjects); err != nil {
			logger.Error(err, "Failed to update staff binding", "workspace", ws.Name)
			workspaceErrors = append(workspaceErrors, fmt.Errorf("workspace %s: %w", ws.Name, err))
			failedWorkspaces = append(failedWorkspaces, ws.Name)
		} else {
			successCount++
		}
	}

	// Update StaffMember annotations with sync status, but only if this was not a workspace-triggered reconcile
	if req.Name != workspaceTrigger {
		syncStatus := "success"
		if len(workspaceErrors) > 0 {
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
	}

	// Aggregate and return errors if any workspace updates failed
	if len(workspaceErrors) > 0 {
		aggregatedErr := errors.Join(workspaceErrors...)
		logger.Error(aggregatedErr, "Reconciliation completed with errors",
			"successCount", successCount,
			"failedCount", len(workspaceErrors),
			"totalWorkspaces", totalWorkspaces,
			"failedWorkspaces", failedWorkspaces)

		// Return error for controller-runtime to handle requeue with exponential backoff
		return ctrl.Result{}, aggregatedErr
	}

	logger.Info("Reconciliation completed successfully",
		"workspacesUpdated", successCount,
		"totalWorkspaces", totalWorkspaces)
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
		if apierrors.IsNotFound(err) {
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
	// Create a metadata-only object for watching StaffMembers
	staffMember := &metav1.PartialObjectMetadata{}
	staffMember.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "maintainer-d.cncf.io",
		Version: "v1alpha1",
		Kind:    "StaffMember",
	})

	// Create a metadata-only object for watching Projects
	project := &metav1.PartialObjectMetadata{}
	project.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "maintainer-d.cncf.io",
		Version: "v1alpha1",
		Kind:    "Project",
	})

	// projectToRequests is a mapper function that enqueues a reconciliation request
	// when a project's workspace becomes ready.
	projectToRequests := func(ctx context.Context, obj client.Object) []reconcile.Request {
		logger := log.FromContext(ctx)
		project, ok := obj.(*metav1.PartialObjectMetadata)
		if !ok {
			logger.Error(fmt.Errorf("unexpected type %T, expected PartialObjectMetadata for Project", obj),
				"failed to map project event")
			return nil
		}

		// Only trigger if project's workspace is ready
		ann := project.GetAnnotations()
		if ann == nil || ann["kdp-workspaces.cncf.io/workspace-phase"] != "Ready" {
			return nil
		}

		logger.Info("Project's workspace is ready, triggering reconciliation", "project", project.GetName())
		// Trigger a single, fixed-name reconciliation for all staff members
		return []reconcile.Request{{
			NamespacedName: types.NamespacedName{
				Name:      workspaceTrigger,
				Namespace: r.StaffMemberNamespace, // Use the configured staff namespace
			},
		}}
	}

	// Create predicate to ignore annotation-only updates to prevent reconciliation loops
	ignoreSyncAnnotations := predicate.Funcs{
		UpdateFunc: func(e event.TypedUpdateEvent[client.Object]) bool {
			// Get old and new annotations
			oldAnnotations := e.ObjectOld.GetAnnotations()
			newAnnotations := e.ObjectNew.GetAnnotations()

			// Create copies without our sync annotations
			oldFiltered := make(map[string]string)
			newFiltered := make(map[string]string)

			for k, v := range oldAnnotations {
				if k != AnnotationStaffLastSynced &&
					k != AnnotationStaffSyncStatus &&
					k != AnnotationStaffWorkspaceCount {
					oldFiltered[k] = v
				}
			}

			for k, v := range newAnnotations {
				if k != AnnotationStaffLastSynced &&
					k != AnnotationStaffSyncStatus &&
					k != AnnotationStaffWorkspaceCount {
					newFiltered[k] = v
				}
			}

			// Check if anything besides sync annotations changed
			// Compare filtered annotations
			if len(oldFiltered) != len(newFiltered) {
				return true
			}
			for k, v := range oldFiltered {
				if newFiltered[k] != v {
					return true
				}
			}

			// Check if spec changed (only primaryEmail matters for us)
			oldSpec, _ := e.ObjectOld.(*metav1.PartialObjectMetadata)
			newSpec, _ := e.ObjectNew.(*metav1.PartialObjectMetadata)

			// If generation changed, spec was modified
			if oldSpec != nil && newSpec != nil && oldSpec.Generation != newSpec.Generation {
				return true
			}

			// Only sync annotations changed, ignore this update
			return false
		},
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(staffMember).
		WithEventFilter(ignoreSyncAnnotations).
		Watches(
			project,
			handler.EnqueueRequestsFromMapFunc(projectToRequests),
		).
		Complete(r)
}
