# E2E Testing Guide for StaffMember Controller

This guide documents manual E2E testing procedures for the StaffMember controller. These tests require actual KCP cluster access and validate cross-cluster behavior that cannot be tested in unit tests.

## Prerequisites

- Access to KDP (KCP) cluster with kubeconfig
- Service cluster with operator deployed
- `maintainerd` namespace with StaffMember CRDs
- kubectl configured with both cluster contexts

## Test Configuration

```bash
# Environment setup
export SERVICE_CLUSTER_CONTEXT="context-cdv2c4jfn5q"
export KDP_CLUSTER_KUBECONFIG="$(git rev-parse --show-toplevel)/tmp/kdp-cluster-cncf/kubeconfig-admin"
export TEST_NAMESPACE="maintainerd-e2e-test"

# Timeouts
WORKSPACE_READY_TIMEOUT=5m
RECONCILIATION_TIMEOUT=2m
BINDING_UPDATE_TIMEOUT=30s
```

## Test Namespace Setup

Create an isolated namespace for E2E tests to avoid conflicts with production resources:

```bash
# Create test namespace
kubectl --context=$SERVICE_CLUSTER_CONTEXT create namespace $TEST_NAMESPACE

# Label for easy identification and cleanup
kubectl --context=$SERVICE_CLUSTER_CONTEXT label namespace $TEST_NAMESPACE \
  purpose=e2e-testing \
  test=kdp-workspaces
```

**Important:** For E2E testing, you have two options:

1. **Dedicated test operator deployment** (Recommended for CI/CD):
   Deploy a separate operator instance configured with `--staff-namespace=$TEST_NAMESPACE`

2. **Configure existing operator** (Quick local testing):
   Update the operator deployment to watch the test namespace:
   ```bash
   kubectl --context=$SERVICE_CLUSTER_CONTEXT edit deployment kdp-workspaces-controller-manager -n kdp-workspaces-system
   # Add: --staff-namespace=$TEST_NAMESPACE
   # Or watch all namespaces: --staff-namespace=""
   ```

**Verify test isolation:**
```bash
# Verify test namespace exists and is labeled
kubectl --context=$SERVICE_CLUSTER_CONTEXT get namespace $TEST_NAMESPACE -o yaml

# Verify no test resources in production namespace
kubectl --context=$SERVICE_CLUSTER_CONTEXT get projects,staffmembers -n maintainerd -l test=e2e
# Expected: No resources found (test resources are in $TEST_NAMESPACE)
```

**Note on annotations:** The ClusterRoleBinding annotation `kdp-workspaces.cncf.io/source-namespace` will reflect the actual namespace being watched (e.g., `$TEST_NAMESPACE` for test resources).

## Test Priority 1: Workspace Creation + Staff Access (HIGH VALUE)

**Purpose:** Validate core happy path - staff members get access when workspaces are created.

**Isolation:** All test resources (Projects, StaffMembers) are created in the isolated `$TEST_NAMESPACE` namespace, ensuring no interference with production `maintainerd` resources.

### Test Steps

1. **Create test Project**
   ```bash
   kubectl --context=$SERVICE_CLUSTER_CONTEXT apply -f - <<EOF
   apiVersion: maintainer-d.cncf.io/v1alpha1
   kind: Project
   metadata:
     name: test-staff-e2e
     namespace: $TEST_NAMESPACE
     labels:
       test: e2e
   spec:
     displayName: "E2E Test Project"
     gitHubOrg: "test-org"
   EOF
   ```

2. **Wait for workspace to be Ready**
   ```bash
   kubectl --context=$SERVICE_CLUSTER_CONTEXT get project test-staff-e2e -n $TEST_NAMESPACE -w

   # Wait for annotations:
   #   kdp-workspaces.cncf.io/workspace-phase: Ready
   ```

3. **Create test StaffMembers**
   ```bash
   for i in 1 2 3; do
     kubectl --context=$SERVICE_CLUSTER_CONTEXT apply -f - <<EOF
   apiVersion: maintainer-d.cncf.io/v1alpha1
   kind: StaffMember
   metadata:
     name: test-staff-$i
     namespace: $TEST_NAMESPACE
     labels:
       test: e2e
   spec:
     displayName: "Test Staff $i"
     primaryEmail: "test-staff-$i@example.com"
   EOF
   done
   ```

4. **Verify ClusterRoleBinding created**
   ```bash
   KUBECONFIG=$KDP_CLUSTER_KUBECONFIG kubectl get clusterrolebinding cncf-staff-access

   # Expected output:
   # NAME                 ROLE                  AGE
   # cncf-staff-access    ClusterRole/kdp:owner 30s
   ```

5. **Verify subjects list**
   ```bash
   KUBECONFIG=$KDP_CLUSTER_KUBECONFIG kubectl get clusterrolebinding cncf-staff-access -o jsonpath='{.subjects[*].name}'

   # Expected output (includes oidc: prefix):
   # oidc:test-staff-1@example.com oidc:test-staff-2@example.com oidc:test-staff-3@example.com
   ```

6. **Verify ClusterRoleBinding annotations**
   ```bash
   KUBECONFIG=$KDP_CLUSTER_KUBECONFIG kubectl get clusterrolebinding cncf-staff-access -o jsonpath='{.metadata.annotations}'

   # Expected annotations:
   # {
   #   "kdp-workspaces.cncf.io/last-synced": "<timestamp>",
   #   "kdp-workspaces.cncf.io/managed-by": "kdp-ws-operator",
   #   "kdp-workspaces.cncf.io/source-namespace": "maintainerd-e2e-test",  # Reflects TEST_NAMESPACE
   #   "kdp-workspaces.cncf.io/staff-count": "3"
   # }
   ```

7. **Sample verify StaffMember annotations**
   ```bash
   kubectl --context=$SERVICE_CLUSTER_CONTEXT get staffmember test-staff-1 -n $TEST_NAMESPACE -o jsonpath='{.metadata.annotations}'

   # Expected annotations:
   # {
   #   "kdp-workspaces.cncf.io/last-synced": "<timestamp>",
   #   "kdp-workspaces.cncf.io/sync-status": "success",
   #   "kdp-workspaces.cncf.io/workspace-count": "1"
   # }
   ```

### Success Criteria
- ✅ ClusterRoleBinding exists in workspace
- ✅ All 3 staff members in subjects list with `oidc:` prefix
- ✅ Annotations present: `managed-by`, `staff-count`
- ✅ Timestamp in `last-synced` is recent (< 2 minutes)
- ✅ StaffMember `sync-status` = "success"

---

## Test Priority 2: Staff Member Addition (MEDIUM-HIGH VALUE)

**Purpose:** Validate incremental changes - new staff members are added to all workspaces.

### Test Steps

1. **Prerequisites:** Complete Test Priority 1 (workspace + 3 staff members exist)

2. **Create additional StaffMember**
   ```bash
   kubectl --context=$SERVICE_CLUSTER_CONTEXT apply -f - <<EOF
   apiVersion: maintainer-d.cncf.io/v1alpha1
   kind: StaffMember
   metadata:
     name: test-staff-4
     namespace: $TEST_NAMESPACE
     labels:
       test: e2e
   spec:
     displayName: "Test Staff 4"
     primaryEmail: "test-staff-4@example.com"
   EOF
   ```

3. **Wait for reconciliation**
   ```bash
   sleep 30  # Allow time for reconciliation
   ```

4. **Verify new subject added**
   ```bash
   KUBECONFIG=$KDP_CLUSTER_KUBECONFIG kubectl get clusterrolebinding cncf-staff-access -o jsonpath='{.subjects[*].name}' | grep "test-staff-4"

   # Expected: oidc:test-staff-4@example.com appears in list
   ```

5. **Verify existing subjects NOT removed**
   ```bash
   KUBECONFIG=$KDP_CLUSTER_KUBECONFIG kubectl get clusterrolebinding cncf-staff-access -o jsonpath='{.subjects[*].name}'

   # Expected: All 4 staff members present
   # oidc:test-staff-1@example.com ... oidc:test-staff-4@example.com
   ```

6. **Verify staff-count updated** (Optional - low value)
   ```bash
   KUBECONFIG=$KDP_CLUSTER_KUBECONFIG kubectl get clusterrolebinding cncf-staff-access -o jsonpath='{.metadata.annotations.kdp-workspaces\.cncf\.io/staff-count}'

   # Expected: "4"
   ```

### Success Criteria
- ✅ New staff member appears in subjects list
- ✅ Existing staff members NOT removed
- ✅ No errors in operator logs

---

## Test Priority 3: Staff Member Removal (MEDIUM VALUE)

**Purpose:** Validate deletion path - removed staff members lose access.

### Test Steps

1. **Prerequisites:** Complete Test Priority 2 (workspace + 4 staff members exist)

2. **Delete one StaffMember**
   ```bash
   kubectl --context=$SERVICE_CLUSTER_CONTEXT delete staffmember test-staff-2 -n $TEST_NAMESPACE
   ```

3. **Wait for reconciliation**
   ```bash
   sleep 30  # Allow time for reconciliation
   ```

4. **Verify subject removed**
   ```bash
   KUBECONFIG=$KDP_CLUSTER_KUBECONFIG kubectl get clusterrolebinding cncf-staff-access -o jsonpath='{.subjects[*].name}' | grep "test-staff-2"

   # Expected: NO output (test-staff-2 NOT present)
   ```

5. **Verify other subjects remain**
   ```bash
   KUBECONFIG=$KDP_CLUSTER_KUBECONFIG kubectl get clusterrolebinding cncf-staff-access -o jsonpath='{.subjects[*].name}'

   # Expected: 3 staff members present (1, 3, 4)
   # oidc:test-staff-1@example.com oidc:test-staff-3@example.com oidc:test-staff-4@example.com
   ```

### Success Criteria
- ✅ Deleted staff member NOT in subjects list
- ✅ Other staff members remain in list
- ✅ No errors in operator logs

---

## Test Priority 4: Multi-Workspace Validation (OPTIONAL)

**Purpose:** Validate staff access propagates to all workspaces.

### Test Steps

1. **Create 3 test Projects**
   ```bash
   for i in 1 2 3; do
     kubectl --context=$SERVICE_CLUSTER_CONTEXT apply -f - <<EOF
   apiVersion: maintainer-d.cncf.io/v1alpha1
   kind: Project
   metadata:
     name: test-multi-$i
     namespace: $TEST_NAMESPACE
     labels:
       test: e2e-multi
   spec:
     displayName: "Multi Test Project $i"
     gitHubOrg: "test-org-$i"
   EOF
   done
   ```

2. **Wait for all workspaces Ready**
   ```bash
   kubectl --context=$SERVICE_CLUSTER_CONTEXT get projects -n $TEST_NAMESPACE -l test=e2e-multi -w
   ```

3. **Create StaffMember**
   ```bash
   kubectl --context=$SERVICE_CLUSTER_CONTEXT apply -f - <<EOF
   apiVersion: maintainer-d.cncf.io/v1alpha1
   kind: StaffMember
   metadata:
     name: test-multi-staff
     namespace: $TEST_NAMESPACE
     labels:
       test: e2e-multi
   spec:
     displayName: "Multi Test Staff"
     primaryEmail: "test-multi-staff@example.com"
   EOF
   ```

4. **Sample verify binding in 2 workspaces**
   ```bash
   # Check workspace 1
   KUBECONFIG=$KDP_CLUSTER_KUBECONFIG kubectl --context=test-multi-1 get clusterrolebinding cncf-staff-access -o jsonpath='{.subjects[*].name}' | grep "test-multi-staff"

   # Check workspace 3
   KUBECONFIG=$KDP_CLUSTER_KUBECONFIG kubectl --context=test-multi-3 get clusterrolebinding cncf-staff-access -o jsonpath='{.subjects[*].name}' | grep "test-multi-staff"
   ```

5. **Verify workspace count annotation**
   ```bash
   kubectl --context=$SERVICE_CLUSTER_CONTEXT get staffmember test-multi-staff -n $TEST_NAMESPACE -o jsonpath='{.metadata.annotations.kdp-workspaces\.cncf\.io/workspace-count}'

   # Expected: "3" (or higher if other workspaces exist)
   ```

### Success Criteria
- ✅ Staff member appears in sampled workspaces
- ✅ Workspace count annotation reflects actual count

---

## Test Priority 5: Error Handling - Partial Sync Failure (HIGH VALUE)

**Purpose:** Validate resilience when some workspaces are unreachable.

### Test Steps

1. **Prerequisites:** Workspace + staff members exist

2. **Simulate workspace unreachability**
   ```bash
   # Option 1: Delete KCP kubeconfig secret temporarily
   kubectl --context=$SERVICE_CLUSTER_CONTEXT delete secret kdp-workspaces -n kdp-workspaces-system

   # Option 2: Network policy to block KCP access
   # (implementation depends on cluster setup)
   ```

3. **Create new StaffMember**
   ```bash
   kubectl --context=$SERVICE_CLUSTER_CONTEXT apply -f - <<EOF
   apiVersion: maintainer-d.cncf.io/v1alpha1
   kind: StaffMember
   metadata:
     name: test-error-staff
     namespace: $TEST_NAMESPACE
   spec:
     displayName: "Error Test Staff"
     primaryEmail: "test-error-staff@example.com"
   EOF
   ```

4. **Wait for reconciliation attempts**
   ```bash
   sleep 60
   ```

5. **Verify sync-status annotation**
   ```bash
   kubectl --context=$SERVICE_CLUSTER_CONTEXT get staffmember test-error-staff -n $TEST_NAMESPACE -o jsonpath='{.metadata.annotations.kdp-workspaces\.cncf\.io/sync-status}'

   # Expected: "error" or "partial"
   ```

6. **Check operator logs for errors**
   ```bash
   kubectl --context=$SERVICE_CLUSTER_CONTEXT logs -n kdp-workspaces-system deployment/kdp-workspaces-controller-manager | grep "Failed to"

   # Expected: Error messages about KCP connection or workspace access
   ```

7. **Restore access and verify recovery**
   ```bash
   # Restore KCP secret or remove network policy
   kubectl --context=$SERVICE_CLUSTER_CONTEXT apply -f config/kdp-secret.yaml

   # Wait for reconciliation
   sleep 60

   # Verify sync-status now "success"
   kubectl --context=$SERVICE_CLUSTER_CONTEXT get staffmember test-error-staff -n $TEST_NAMESPACE -o jsonpath='{.metadata.annotations.kdp-workspaces\.cncf\.io/sync-status}'
   ```

### Success Criteria
- ✅ StaffMember `sync-status` = "error" or "partial" during failure
- ✅ Operator logs show appropriate error messages
- ✅ StaffMember `sync-status` = "success" after recovery
- ✅ No operator crash or restart

---

## Cleanup

After completing all tests:

```bash
# Delete test StaffMembers
kubectl --context=$SERVICE_CLUSTER_CONTEXT delete staffmembers -n $TEST_NAMESPACE -l test=e2e
kubectl --context=$SERVICE_CLUSTER_CONTEXT delete staffmembers -n $TEST_NAMESPACE -l test=e2e-multi

# Delete test Projects
kubectl --context=$SERVICE_CLUSTER_CONTEXT delete projects -n $TEST_NAMESPACE -l test=e2e
kubectl --context=$SERVICE_CLUSTER_CONTEXT delete projects -n $TEST_NAMESPACE -l test=e2e-multi

# Verify workspaces cleaned up in KDP cluster
KUBECONFIG=$KDP_CLUSTER_KUBECONFIG kubectl get workspaces | grep test-

# Delete test namespace (this will delete all remaining resources)
kubectl --context=$SERVICE_CLUSTER_CONTEXT delete namespace $TEST_NAMESPACE
```

**Note:** Deleting the test namespace will cascade delete all Projects and StaffMembers in it, which will trigger the operator to clean up the corresponding workspaces in the KDP cluster.

---

## Troubleshooting

### Reconciliation not happening

```bash
# Check operator is running
kubectl --context=$SERVICE_CLUSTER_CONTEXT get pods -n kdp-workspaces-system

# Check operator logs
kubectl --context=$SERVICE_CLUSTER_CONTEXT logs -n kdp-workspaces-system deployment/kdp-workspaces-controller-manager -f

# Check StaffMember events
kubectl --context=$SERVICE_CLUSTER_CONTEXT describe staffmember <name> -n $TEST_NAMESPACE
```

### ClusterRoleBinding not created

```bash
# Verify workspace is Ready
kubectl --context=$SERVICE_CLUSTER_CONTEXT get project <name> -n $TEST_NAMESPACE -o yaml | grep -A5 annotations

# Verify KCP config is correct
kubectl --context=$SERVICE_CLUSTER_CONTEXT get configmap kdp-workspaces -n kdp-workspaces-system -o yaml
kubectl --context=$SERVICE_CLUSTER_CONTEXT get secret kdp-workspaces -n kdp-workspaces-system

# Manually test KCP access
KUBECONFIG=$KDP_CLUSTER_KUBECONFIG kubectl get workspaces
```

### Subjects list incorrect

```bash
# Verify StaffMember has primaryEmail
kubectl --context=$SERVICE_CLUSTER_CONTEXT get staffmember <name> -n $TEST_NAMESPACE -o jsonpath='{.spec.primaryEmail}'

# Check if StaffMember has empty email (will be skipped)
kubectl --context=$SERVICE_CLUSTER_CONTEXT get staffmembers -n $TEST_NAMESPACE -o json | jq '.items[] | select(.spec.primaryEmail == "")'
```

---

## Test Execution Time Estimates

| Test | Expected Duration |
|------|-------------------|
| Priority 1: Workspace Creation + Staff Access | ~8-10 minutes |
| Priority 2: Staff Member Addition | ~2-3 minutes |
| Priority 3: Staff Member Removal | ~2-3 minutes |
| Priority 4: Multi-Workspace | ~10-12 minutes |
| Priority 5: Error Handling | ~5-7 minutes |
| **Total** | ~30-40 minutes |

---

## Automated E2E Tests (Future Work)

These tests can be automated using Ginkgo/Gomega when:
- KCP cluster is available in CI/CD
- Test fixtures and cleanup are standardized
- Cross-cluster kubeconfig management is implemented

See `test/e2e/e2e_test.go` for existing automated test patterns.
