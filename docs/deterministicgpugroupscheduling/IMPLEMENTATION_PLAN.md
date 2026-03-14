# Implementation Plan: GPUGroup & GPUGroupTemplate

## Phase 1: CRD Type Definitions & Code Generation

### 1.1 Define GPUGroup types

Create `pkg/apis/kai/v1alpha1/gpugroup_types.go`:

- `GPUGroup` and `GPUGroupList` structs with kubebuilder markers (`+genclient`, `+kubebuilder:object:root=true`, `+kubebuilder:subresource:status`)
- `GPUGroupSpec`: `GPUCount int32`, `MaxAttachedPods *int32`
- `GPUGroupStatus`: `Phase GPUGroupPhase`, `NodeName string`, `GPUSUUIDs []string`, `AttachedPodsNames []string`, `UniqueMemberIDs []string`, `Conditions []metav1.Condition`
- `GPUGroupPhase` type with constants: `Accepted`, `Allocated`, `Failed`
- Place in `kai/v1alpha1` alongside `Topology` (same group `kai.scheduler`, same version)

### 1.2 Define GPUGroupTemplate types

Create `pkg/apis/kai/v1alpha1/gpugrouptemplate_types.go`:

- `GPUGroupTemplate` and `GPUGroupTemplateList` structs
- `GPUGroupTemplateSpec`: `Template GPUGroupTemplateData` (embeds GPUGroup metadata+spec)
- `GPUGroupTemplateStatus`: `TemplatedGPUGroupsNames []string`, `Conditions []metav1.Condition`

### 1.3 Run code generation

- `make generate` → deepcopy methods in `zz_generated.deepcopy.go`
- `make manifests` → CRD YAMLs in `deployments/kai-scheduler/crds/`
- `make clients` → clientset, informers, listers in `pkg/apis/client/`

**Files created:**

- `pkg/apis/kai/v1alpha1/gpugroup_types.go`
- `pkg/apis/kai/v1alpha1/gpugrouptemplate_types.go`

**Files modified:**

- `pkg/apis/kai/v1alpha1/zz_generated.deepcopy.go` (auto-generated)
- `deployments/kai-scheduler/crds/` (new CRD YAMLs auto-generated)
- `pkg/apis/client/` (auto-generated clientset, informers, listers)

---

## Phase 2: Admission Webhooks

### 2.1 GPUGroup validation webhook

Create `pkg/apis/kai/v1alpha1/gpugroup_webhook.go`:

- `SetupGPUGroupWebhookWithManager(mgr)` — follows the Queue/PodGroup webhook pattern
- `ValidateCreate`: `gpuCount >= 1`, `maxAttachedPods` if set must be `>= 1`
- `ValidateUpdate`: reject changes to `spec.gpuCount` (immutability); `spec.maxAttachedPods` can only be increased (reject decreases and reject setting to nil if previously set)
- `ValidateDelete`: no-op

### 2.2 GPUGroupTemplate validation webhook

Create `pkg/apis/kai/v1alpha1/gpugrouptemplate_webhook.go`:

- Validate the embedded template spec (same rules as GPUGroup create validation)

### 2.3 Pod admission plugin for GPUGroup references

Create `pkg/admission/webhook/v1alpha1/gpugroup/gpu_group.go`:

- Implements the existing `Plugin` interface (`Name()`, `Validate()`, `Mutate()`)
- `Validate`: if pod has `kai.scheduler/gpu-group` label, verify the referenced GPUGroup exists in the same namespace and queue
- `Mutate`: if pod references a GPUGroup (via `kai.scheduler/gpu-group` or `kai.scheduler/gpu-group-template` label), mutate the pod's target containers to add `envFrom` referencing the `gpu-sharing` ConfigMap — same mutation the existing admission webhook performs for fractional GPU pods. The `kai.scheduler/gpu-group-attached-container-names` annotation determines which containers are mutated (defaults to first container)
- Register in `cmd/admission/app/app.go` alongside existing plugins

**Files created:**

- `pkg/apis/kai/v1alpha1/gpugroup_webhook.go`
- `pkg/apis/kai/v1alpha1/gpugrouptemplate_webhook.go`
- `pkg/admission/webhook/v1alpha1/gpugroup/gpu_group.go`

**Files modified:**

- `cmd/admission/app/app.go` — register new pod admission plugin + CRD webhooks

---

## Phase 3: GPUGroup Controller

A new controller manages GPUGroup lifecycle: reservation pod creation, status updates, node failure detection.

### 3.1 Controller structure

Create `pkg/gpugroupcontroller/` with:

- `controllers/gpugroup_controller.go` — `GPUGroupReconciler` using controller-runtime
- Watches: `GPUGroup`, `Pod` (filtered by `kai.scheduler/gpu-group` label)
- Field indexers: pods by `metadata.labels.kai.scheduler/gpu-group`

### 3.2 Reconciliation logic

On each reconcile:

1. List pods referencing this GPUGroup (via indexer)
2. If no consumer pods exist and `gpu-reservation` pod exists → delete reservation pod, set phase `Accepted`
3. If consumer pods exist and phase is `Allocated` → update `status.attachedPodsNames` and `status.uniqueMemberIDs`, update reservation pod's owner references
4. If phase is `Allocated` and gpu-reservation pod is unhealthy → set phase `Failed`; remains in `Failed` until the gpu-reservation pod becomes healthy again

### 3.3 App entry point

Create `cmd/gpugroupcontroller/`:

- `main.go` and `app/app.go` following the pattern from `cmd/podgroupcontroller/`
- Manager setup with leader election, cache filtering

### 3.4 Operator integration

- Create `pkg/operator/operands/gpugroup_controller/` operand
- Register in `ConfigReconcilerOperands` in `pkg/operator/controller/config_controller.go`

**Files created:**

- `pkg/gpugroupcontroller/controllers/gpugroup_controller.go`
- `cmd/gpugroupcontroller/main.go`
- `cmd/gpugroupcontroller/app/app.go`
- `pkg/operator/operands/gpugroup_controller/` (operand files)

**Files modified:**

- `pkg/operator/controller/config_controller.go` — add operand

---

## Phase 4: GPUGroupTemplate Controller

### 4.1 Controller structure

Create `pkg/gpugrouptemplatecontroller/`:

- `controllers/gpugrouptemplate_controller.go`
- Watches: `GPUGroupTemplate`, `GPUGroup` (owned by template)

### 4.2 Reconciliation logic

On reconcile:

1. List GPUGroups owned by this template
2. Update `status.templatedGPUGroupsNames` with owned GPUGroup names
3. No automatic GPUGroup creation here — GPUGroups are created on-demand by the scheduler when a pod references the template and no existing GPUGroup can accept it

### 4.3 App entry point & operator integration

Same pattern as Phase 3.

**Files created:**

- `pkg/gpugrouptemplatecontroller/controllers/gpugrouptemplate_controller.go`
- `cmd/gpugrouptemplatecontroller/main.go`
- `cmd/gpugrouptemplatecontroller/app/app.go`
- `pkg/operator/operands/gpugrouptemplate_controller/`

**Files modified:**

- `pkg/operator/controller/config_controller.go` — add operand

---

## Phase 5: Scheduler Integration

### 5.1 Cache — load GPUGroups into ClusterInfo

- Add `GPUGroupInfos map[string]*GPUGroupInfo` to `ClusterInfo` (`pkg/scheduler/api/cluster_info.go`)
- Create `pkg/scheduler/api/gpugroup_info/gpugroup_info.go` — internal scheduler representation
- Add GPUGroup informer+lister to `DataLister` (`pkg/scheduler/cache/cluster_info/data_lister/`)
- Add `snapshotGPUGroups()` to cluster snapshot flow (`pkg/scheduler/cache/cluster_info/cluster_info.go`)

### 5.2 Scheduler plugin — `gpugroup`

Create `pkg/scheduler/plugins/gpugroup/gpugroup.go`:

- `**PrePredicateFn`**: If pod references a `GPUGroup`:
  - Look up GPUGroup in `ClusterInfo.GPUGroupInfos`
  - If not found → return error (Unschedulable)
  - If phase is `Accepted` → trigger reservation flow (see 5.3), return error (Pending)
  - If phase is `Failed` → Skip this `GPUGroup`
  - If phase is `Allocated` → check `maxAttachedPods` and `uniqueMemberIDs` constraints, restrict to the GPUGroup's node
- `**PredicateFn`**: If pod references an `Allocated` GPUGroup → only allow the node matching `status.nodeName`, given that all other scheduling constraints also allow this node, otherwise mark the Pod as `Unschedulable`. This means consumer pods with scheduling constraints incompatible with the GPUGroup's node (e.g., mismatched nodeSelector, affinity, tolerations) will fail here
- `**BindRequestMutateFn`**: Inject the GPUGroup's GPU UUIDs into `SelectedGPUGroups` on the BindRequest
- Register in `pkg/scheduler/plugins/factory.go`

### 5.3 Reservation pod creation from scheduler

When the scheduler encounters a pod referencing a GPUGroup in `Accepted`/`Failed` phase:

1. Pick a node using normal scoring (respecting pod scheduling constraints)
2. Create the `gpu-reservation` pod on that node via the existing resource reservation service pattern
3. Update GPUGroup status: `phase=Allocated`, `nodeName`, `gpus` (after reservation pod is Ready)
4. The consumer pod remains Pending until the next scheduling cycle when the GPUGroup is `Allocated`

This logic lives in the `gpugroup` plugin's `PrePredicateFn` or as a separate pre-scheduling step.

### 5.4 GPUGroupTemplate resolution in scheduler

When a pod references a `GPUGroupTemplate` (via label `kai.scheduler/gpu-group-template`):

1. Look up the template in cache
2. Iterate existing GPUGroups created from this template
3. For each, check if the pod can attach (maxAttachedPods, uniqueMemberID constraints)
4. If a valid GPUGroup is found → set the `kai.scheduler/gpu-group` label on the pod to point to that GPUGroup, then treat the pod as a direct GPUGroup consumer from this point on
5. If none found → create a new GPUGroup from the template spec, leave pod Pending
6. On binding failure, the scheduler may override the `kai.scheduler/gpu-group` label to retry with a different GPUGroup from the same template. This label override is only permitted for the GPUGroupTemplate use case (pods that also have the `kai.scheduler/gpu-group-template` label)

This can be a separate `gpugrouptemplate` plugin or part of the `gpugroup` plugin.

**Files created:**

- `pkg/scheduler/api/gpugroup_info/gpugroup_info.go`
- `pkg/scheduler/plugins/gpugroup/gpugroup.go`

**Files modified:**

- `pkg/scheduler/api/cluster_info.go` — add GPUGroupInfos map
- `pkg/scheduler/cache/cluster_info/data_lister/interface.go` — add ListGPUGroups
- `pkg/scheduler/cache/cluster_info/data_lister/kubernetes_lister.go` — implement ListGPUGroups
- `pkg/scheduler/cache/cluster_info/cluster_info.go` — add snapshotGPUGroups()
- `pkg/scheduler/cache/cache.go` — add GPUGroup informer
- `pkg/scheduler/plugins/factory.go` — register gpugroup plugin

---

## Phase 6: Binder Integration

### 6.1 GPU UUID injection

Modify `pkg/binder/binding/binder.go`:

- When binding a pod that references a GPUGroup, read the GPUGroup's `status.gpusUUIDs` UUIDs
- Inject them into the `gpu-sharing` ConfigMap (same mechanism as fractional GPUs)
- Use the existing `gpusharing` binder plugin path

### 6.2 Owner reference updates

Modify the binder or GPUGroup controller to update the `gpu-reservation` pod's owner references when a new consumer pod is bound.

**Files modified:**

- `pkg/binder/binding/binder.go` — GPUGroup-aware binding
- `pkg/binder/plugins/gpusharing/gpu_sharing.go` — handle GPUGroup GPU injection

---

## Phase 7: RBAC & Helm Chart

### 7.1 RBAC

- Scheduler: needs `get;list;watch` on `gpugroups` and `gpugrouptemplates`, plus `update;patch` on `gpugroups/status`
- Binder: needs `get;list;watch` on `gpugroups`, `update;patch` on `gpugroups/status`
- GPUGroup controller: needs `get;list;watch;update;patch` on `gpugroups`, `gpugroups/status`, `pods`
- GPUGroupTemplate controller: needs `get;list;watch;update;patch` on `gpugrouptemplates`, `gpugrouptemplates/status`, `gpugroups`
- Admission: needs `get;list` on `gpugroups`

### 7.2 Helm chart updates

- `deployments/kai-scheduler/templates/rbac/` — new RBAC files for the GPUGroup controller
- `deployments/kai-scheduler/templates/services/` — service account for GPUGroup controller
- CRD YAMLs auto-included via `deployments/kai-scheduler/crds/embed.go`

---

## Phase 8: Tests

- **Unit tests**: webhook validation, controller reconciliation logic, scheduler plugin predicate/scoring logic
- **Integration tests**: GPUGroup lifecycle (create → pod arrives → reservation → binding), GPUGroupTemplate → GPUGroup creation
- **E2E tests**: in `test/e2e/suites/gpugroup/` — full flow from GPUGroup creation to pod scheduling

---

## Suggested Implementation Order

1. **Phase 1** (CRD types + codegen) — foundation, unblocks everything
2. **Phase 2** (webhooks) — validation early prevents bad data
3. **Phase 3** (GPUGroup controller) — core lifecycle management
4. **Phase 5** (scheduler integration) — the main scheduling logic
5. **Phase 6** (binder integration) — completes the binding path
6. **Phase 4** (GPUGroupTemplate controller) — builds on GPUGroup
7. **Phase 7** (RBAC + Helm) — deployment concerns
8. **Phase 8** (tests) — should be written alongside each phase

