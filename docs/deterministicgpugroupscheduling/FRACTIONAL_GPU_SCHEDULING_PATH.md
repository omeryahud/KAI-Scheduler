# Fractional GPU Pod Scheduling Path

End-to-end code path for scheduling a pod that requests a fractional GPU (e.g. 0.5 GPU).

---

## 1. Scheduling Cycle Starts

**`Scheduler.runOnce()`** — `pkg/scheduler/scheduler.go` line 112

The scheduler runs periodically. Each cycle:

1. Generates a session ID
2. Opens a new session via `framework.OpenSession()` — snapshots cluster state
3. Executes actions sequentially (allocate, preempt, reclaim, etc.)

**`openSession()`** — `pkg/scheduler/framework/session.go` line 341

Creates a `Session` containing:

- `ClusterInfo` — snapshot of pods, nodes, queues, job groups
- Plugin function registries (predicates, order functions, bind request mutators, etc.)

---

## 2. Allocate Action Picks a Task

**`allocateAction.Execute()`** — `pkg/scheduler/actions/allocate/allocate.go` line 46

Iterates jobs ordered by queue priority, calls into common allocation logic for each.

**`AllocateJob()`** — `pkg/scheduler/actions/common/allocate.go` line 20

For each pending task in the job, calls `allocateTask()`.

**`allocateTask()`** — `pkg/scheduler/actions/common/allocate.go` line 121

1. Runs **PrePredicateFn** on the task (early rejection before node iteration)
2. Gets ordered candidate nodes via `OrderedNodesByTask()`
3. For each node, calls `FittingNode()` which runs **PredicateFn**
4. If a node fits, calls `allocateTaskToNode()`

---

## 3. Fractional GPU Detection & Routing

**`allocateTaskToNode()`** — `pkg/scheduler/actions/common/allocate.go` line 165

Checks `task.IsFractionRequest()` (`pkg/scheduler/api/pod_info/pod_info.go` line 308). If true, routes to fractional GPU allocation path:

**`AllocateFractionalGPUTaskToNode()`** — `pkg/scheduler/gpu_sharing/gpuSharing.go` line 20

1. Calls `ssn.FittingGPUs()` — finds GPUs on the node with enough free memory
2. Calls `GetNodePreferableGpuForSharing()` — selects the best GPU(s)
3. Sets `pod.GPUGroups` to the selected GPU group UUID(s)
4. Calls `allocateSharedGPUTask()` to record the allocation

---

## 4. GPU Fitting & Selection

**`FittingGPUs()`** — `pkg/scheduler/framework/session.go` line 163

Returns a list of GPU identifiers that can fit the pod:

- **Shared GPU indices** (e.g. `"0"`, `"1"`) — existing groups with enough free memory
- **`WholeGpuIndicator` (`"-2"`)** entries — one per idle/releasing whole GPU

**`GetNodePreferableGpuForSharing()`** — `pkg/scheduler/gpu_sharing/gpuSharing.go` line 38

For each fitting GPU:

- If it's a `WholeGpuIndicator` → generates a **new UUID** (first pod to share this GPU)
- Otherwise → uses the existing GPU group index (joining an existing shared group)

Returns grouped GPUs with `IsReleasing` flag.

---

## 5. Predicates Plugin — GPU Group Awareness

**`evaluateTaskOnPrePredicate()`** — `pkg/scheduler/plugins/predicates/predicates.go` line 123

Called during pre-predicate phase (before node iteration).

**`evaluateTaskOnPredicates()`** — `pkg/scheduler/plugins/predicates/predicates.go` line 173

Called per-node during predicate phase.

**`willCreateNewGpuGroup()`** — `pkg/scheduler/plugins/predicates/predicates.go` line 289

Uses `FittingGPUs()` + `GetNodePreferableGpuForSharing()` to check if this allocation will create a new GPU group (new UUID rather than joining an existing one).

**`checkMaxPodsWithGpuGroupReservation()`** — `pkg/scheduler/plugins/predicates/predicates.go` line 264

If creating a new GPU group, ensures the node has at least **2 available pod slots** (one for the pod, one for the **resource reservation pod** that holds the GPU).

---

## 6. Virtual Allocation (Statement)

**`Statement.Allocate()`** — `pkg/scheduler/framework/statement.go` line 296

Records the allocation virtually (not yet committed to K8s API):

- Sets pod's `NodeName`
- Adds task to node's task list
- Marks pod with `IsVirtualStatus = true`

---

## 7. Commit & BindRequest Creation

**`Statement.Commit()`** — `pkg/scheduler/framework/statement.go` line 535

Iterates all recorded operations and commits them.

**`Statement.commitAllocate()`** — `pkg/scheduler/framework/statement.go` line 359

For fractional GPU pods (`IsFractionAllocation()` at `pkg/scheduler/api/pod_info/pod_info.go` line 312):

- Ensures each GPU group entry exists in node's `UsedSharedGPUsMemory` map
- Calls `ssn.BindPod()`

**`BindPod()`** — `pkg/scheduler/framework/session.go` line 111

1. Calls `MutateBindRequestAnnotations()` (`pkg/scheduler/framework/session_plugins.go` line 443) — collects annotations from all registered plugin `BindRequestMutateFn`s
2. Calls `Cache.Bind()`

**`Cache.Bind()`** — `pkg/scheduler/cache/cache.go` line 267

Calls `createBindRequest()` (line 290) which creates a `BindRequest` CRD:

- `Spec.SelectedNode` = node name
- `Spec.SelectedGPUGroups` = GPU group UUIDs from `pod.GPUGroups`
- `Spec.ReceivedGPU.Portion` = fractional amount (e.g. `"0.50"`)
- `Spec.ReceivedResourceType` = `"Fraction"`

---

## 8. Binder Picks Up BindRequest

**`BindRequestReconciler.Reconcile()`** — `pkg/binder/controllers/bindrequest_controller.go` line 89

Watches for new `BindRequest` resources. On reconcile:

1. Gets the Pod and Node objects
2. Calls `binder.Bind()`
3. On error, calls `binder.Rollback()`
4. Updates BindRequest status

---

## 9. GPU Reservation & Device Assignment

**`Binder.Bind()`** — `pkg/binder/binding/binder.go` line 42

For shared GPU allocations (`IsSharedGPUAllocation()` at `pkg/scheduler/api/pod_info/pod_info.go` line 336):

1. Calls `reserveGPUs()` (line 111):
   - For each GPU group UUID in `BindRequest.Spec.SelectedGPUGroups`
   - Calls `resourceReservationService.ReserveGpuDevice()` (`pkg/binder/binding/resourcereservation/resource_reservation.go` line 211)
   - Returns actual GPU device indices (e.g. `["0"]`, `["1"]`)
2. Creates `BindingState` with `ReservedGPUIds`
3. Calls `plugins.PreBind()`
4. Binds pod to node via K8s binding API
5. Calls `plugins.PostBind()`

---

## 10. GPU Sharing Plugin — ConfigMap & Device Injection

**`GPUSharing.PreBind()`** — `pkg/binder/plugins/gpusharing/gpu_sharing.go` line 44

For shared GPU allocations:

1. Gets reserved GPU device IDs from `BindingState`
2. Converts to CDI device names if needed
3. Creates/updates a **ConfigMap** with GPU device info
4. Injects env vars into the pod's container

**`SetNvidiaVisibleDevices()`** — `pkg/binder/common/gpu_access.go` line 63

- Sets ConfigMap key `NVIDIA_VISIBLE_DEVICES` = comma-separated device indices

**`SetGPUPortion()`** — `pkg/binder/common/gpu_access.go` line 103

- Sets ConfigMap key `GPU_PORTION` = fractional value (e.g. `"0.50"`)

---

## Key Data Structures

| Struct / Field | Location | Role |
|---|---|---|
| `PodInfo.GPUGroups` | `pkg/scheduler/api/pod_info/pod_info.go` line 47 | GPU group UUIDs assigned to pod |
| `WholeGpuIndicator = "-2"` | `pkg/scheduler/api/pod_info/pod_info.go` line 47 | Sentinel for unshared whole GPU slots |
| `GpuSharingNodeInfo` | `pkg/scheduler/api/node_info/gpu_sharing_node_info.go` line 18 | Per-node GPU memory tracking by UUID |
| `UsedSharedGPUsMemory` | `pkg/scheduler/api/node_info/gpu_sharing_node_info.go` line 23 | `map[string]int64` — GPU group UUID to total allocated memory |
| `BindRequest.Spec.SelectedGPUGroups` | (CRD) | UUIDs passed from scheduler to binder |

## Key Methods

| Method | Location | Purpose |
|---|---|---|
| `IsFractionRequest()` | `pkg/scheduler/api/pod_info/pod_info.go` line 308 | True if pod requests fractional GPU |
| `IsSharedGPURequest()` | `pkg/scheduler/api/pod_info/pod_info.go` line 332 | True if fractional or memory request |
| `IsSharedGPUAllocation()` | `pkg/scheduler/api/pod_info/pod_info.go` line 336 | True if pod has GPUGroups assigned |
| `addSharedTaskResources()` | `pkg/scheduler/api/node_info/gpu_sharing_node_info.go` line 73 | Update node GPU memory when pod added |
| `removeSharedTaskResources()` | `pkg/scheduler/api/node_info/gpu_sharing_node_info.go` line 151 | Update node GPU memory when pod removed |
