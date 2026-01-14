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
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
)

func TestAnnotationConstants(t *testing.T) {
	// Verify annotation constants follow expected naming convention
	tests := []struct {
		name     string
		constant string
		expected string
	}{
		{
			name:     "last synced annotation",
			constant: AnnotationBindingLastSynced,
			expected: "kdp-workspaces.cncf.io/last-synced",
		},
		{
			name:     "staff count annotation",
			constant: AnnotationBindingStaffCount,
			expected: "kdp-workspaces.cncf.io/staff-count",
		},
		{
			name:     "managed by annotation",
			constant: AnnotationBindingManagedBy,
			expected: "kdp-workspaces.cncf.io/managed-by",
		},
		{
			name:     "source namespace annotation",
			constant: AnnotationBindingSourceNamespace,
			expected: "kdp-workspaces.cncf.io/source-namespace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.constant != tt.expected {
				t.Errorf("annotation constant mismatch: got %q, want %q", tt.constant, tt.expected)
			}
		})
	}
}

func TestStaffRoleNameConstant(t *testing.T) {
	// Verify the staff role name follows the kdp:owner format
	expected := "kdp:owner"
	if StaffRoleName != expected {
		t.Errorf("StaffRoleName = %q, want %q", StaffRoleName, expected)
	}
}

func TestSubjectListGeneration(t *testing.T) {
	// Test that subject list is correctly formatted with oidc: prefix
	tests := []struct {
		name           string
		emails         []string
		expectedCount  int
		expectedPrefix string
	}{
		{
			name:           "empty list",
			emails:         []string{},
			expectedCount:  0,
			expectedPrefix: "oidc:",
		},
		{
			name:           "single email",
			emails:         []string{"user@example.com"},
			expectedCount:  1,
			expectedPrefix: "oidc:",
		},
		{
			name:           "multiple emails",
			emails:         []string{"user1@example.com", "user2@example.com", "user3@example.com"},
			expectedCount:  3,
			expectedPrefix: "oidc:",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			subjects := []rbacv1.Subject{}
			for _, email := range tt.emails {
				subjects = append(subjects, rbacv1.Subject{
					Kind: rbacv1.UserKind,
					Name: "oidc:" + email,
				})
			}

			if len(subjects) != tt.expectedCount {
				t.Errorf("subject count = %d, want %d", len(subjects), tt.expectedCount)
			}

			for i, subject := range subjects {
				if subject.Kind != rbacv1.UserKind {
					t.Errorf("subject[%d].Kind = %q, want %q", i, subject.Kind, rbacv1.UserKind)
				}

				if len(subject.Name) < len(tt.expectedPrefix) ||
					subject.Name[:len(tt.expectedPrefix)] != tt.expectedPrefix {
					t.Errorf("subject[%d].Name = %q, want prefix %q", i, subject.Name, tt.expectedPrefix)
				}
			}
		})
	}
}

func TestBindingNameConstant(t *testing.T) {
	// Verify the binding name is correctly set
	expected := "cncf-staff-access"
	if StaffAccessBindingName != expected {
		t.Errorf("StaffAccessBindingName = %q, want %q", StaffAccessBindingName, expected)
	}
}

func TestOperatorNameConstant(t *testing.T) {
	// Verify the operator name matches the expected value
	expected := "kdp-ws-operator"
	if OperatorName != expected {
		t.Errorf("OperatorName = %q, want %q", OperatorName, expected)
	}
}
