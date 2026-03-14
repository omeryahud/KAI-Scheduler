# Deterministic Scheduling on Shared GPUs

## Motivation

Allowing workloads of different kinds to share a group of GPUs (driver-managed time-sharing or Run:AI Swap) in a deterministic way.

## Proposal

### Scheduling API

#### New `GPUGroup` CRD is introduced:

A `GPUGroup` is a new resource that users can define and reference in their Pods. It gives them the ability to request scheduling of multiple Pods to share the same GPU devices (1 or more whole GPU devices).

```yaml
apiVersion: kai.scheduler/v1alpha1
kind: GPUGroup
metadata:
  name: gpu-group-1
  # The namespace of the GPUGroup, and the namespace that consuming Pods must reside in
  namespace: swap
  labels:
    # Required. Specifies from which Queue's quota the GPUs within this group will be allocated
    kai.scheduler/queue: swap
spec:
  # Required. Immutable. Number of GPUs to allocate for this GPUGroup
  gpuCount: 2
  # Optional. Mutable, can only be increased. If not specified (nil), unlimited. Specifies the maximum number of Pods that can be attached to this GPUGroup
  maxAttachedPods: 3
status:
  # Kubernetes conditions to communicate state
  conditions: 
    ...
  # Specifies what's the Phase of this GPUGroup:
  #   Accepted  - No physical GPUs were allocated yet (no requesting Pods)
  #   Allocated - Physical GPUs allocated on a node
  #   Failed    - Previously allocated node became unavailable; nodeName is cleared and GPUGroup awaits re-allocation
  phase: Accepted | Allocated | Failed
  # Node name that GPUs for this group were allocated from
  nodeName: node-1
  # Specifies GPU UUIDs of the allocated GPUs within nodeName
  gpusUUIDs:
  - uuid-123412341234
  - uuid-qwerqwerqwer
  # Specifies names of Pods that have this GPUGroup's GPUs attached
  attachedPodsNames:
  - consumer-1
  - consumer-2
  # Specifies unique member IDs specified by Pods that have the GPUs of this GPUGroup attached. An incoming Pod cannot have the GPUGroups' GPUs attached to it, if it specifies a unique-member-id that was already specified by a proior Pod
  uniqueMemberIDs: 
  - unique-member-id-1
  - unique-member-id-2
```

Once a `GPUGroup` is created, Pods can reference it and request to be scheduled on the same Node, and have all GPUs allocated to the `GPUGroup` attached:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: consumer-1
  # Pods can only reference GPUGroups allocated within their namespace
  namespace: swap
  labels:
    # References the GPUGroup the Pod requests to be attached to it
    kai.scheduler/gpu-group: gpu-group-1
    # Optional. If specified, guarentees that only a single pod with this unique-member-id will have the GPUGroup's GPUs attacehd to it.
    kai.scheduler/gpu-group-unique-member-id: consumer-1
    # Pods can only reference GPUGroups allocated within their queue
    kai.scheduler/queue: swap
  annotations:
    # Optional. If not specified, GPUs will be attached to the first container in the pod. Specifies which containers within this Pod should have GPUs from the GPUGroup attached
    kai.scheduler/gpu-group-attached-container-names: primary,secondary
spec:
  schedulerName: kai-scheduler
  ...
---
apiVersion: v1
kind: Pod
metadata:
  name: consumer-2
  # Pods can only reference GPUGroups allocated within their namespace
  namespace: swap
  labels:
    # References the GPUGroup the Pod requests to be attached to it
    kai.scheduler/gpu-group: gpu-group-1
    # Optional. If specified, guarentees that only a single pod with this unique-member-id will have the GPUGroup's GPUs attacehd to it.
    kai.scheduler/gpu-group-unique-member-id: consumer-2
    # Pods can only reference GPUGroups allocated within their queue
    kai.scheduler/queue: swap
  annotations:
    # Optional. If not specified, GPUs will be attached to the first container in the pod. Specifies which containers within this Pod should have GPUs from the GPUGroup attached
    kai.scheduler/gpu-group-attached-container-names: primary,secondary
spec:
  schedulerName: kai-scheduler
  ...
```

##### Semantics

- Pods that reference a `GPUGroup` will leverage the gpu-sharing mechanism used for fractional GPUs (`gpu-sharing` ConfigMap, env vars, etc)
- If Pods reference a non-existent `GPUGroup`, the scheduler should mark the Pods as `Unschedulable`, and attempt scheduling once a matching `GPUGroup` is created
- If Pods reference an existing `GPUGroup`:
  - If all other scheduling constraints of the Pod are met:
    - If a `gpu-reservation` Pod for this `GPUGroup` does not exist, the scheduler should pick a Node for the incoming Pod, create a `gpu-reservation` Pod, and leave the incoming Pod Pending until the `gpu-reservation` Pod becomes Ready
    - If a `gpu-reservation` Pod already exists and is Ready:
      - The scheduler should attempt scheduling of the incoming Pod to the same Node as the `gpu-reservation` Pod, and the binder should inject the `GPUGroup`'s GPUs' UUIDs to the incoming Pod's `gpu-sharing` ConfigMap
- If no Pods reference a `GPUGroup`, its `gpu-reservation` Pod should be deleted
- The `gpu-reservation` Pod's owner references should be updated to include all Pods that have the GPUGroup's GPUs attached to them

##### Quota Accounting

- The `gpu-reservation` Pod's GPU resources are accounted against the Queue referenced by the GPUGroup's `kai.scheduler/queue` label

##### GPU Reservation Pod Failure

- If the `gpu-reservation` Pod reserving a GPUGroup's GPUs becomes unhealthy, the GPUGroup transitions to `Failed` phase.
- The GPUGroup remains in `Failed` phase until its `gpu-reservation` Pod becomes healthy again

##### Consumer Pod Scheduling Constraints

- The first consumer Pod's scheduling constraints (nodeSelector, affinity, tolerations) determine which Node the GPUGroup's `gpu-reservation` Pod is placed on
- Subsequent consumer Pods are constrained to the same Node. If a consumer Pod has scheduling constraints incompatible with the GPUGroup's Node (e.g., a nodeSelector that doesn't match), it will be marked as `Unschedulable`
- Users should ensure all Pods referencing the same GPUGroup have compatible scheduling constraints

##### Validation

- An admission webhook validates that:
  - Pods reference a `GPUGroup` within their own namespace and queue
  - `spec.gpuCount` is immutable after creation

#### New `GPUGroupTemplate` CRD is introduced:

A `GPUGroupTemplate` is a resource that allows Pods to either dynamically request the creation of `GPUGroup`s, or request the GPUs of an existing `GPUGroup` to be attached to them.

Pods are usually created by a higher level controller that manages multiple replicas of the same Pod template. The use of a `GPUGroup` will result in replicas of the same Pod sharing the same GPUs. Although this is a valid use case, it is not generally how they are used.

This API aims to allow seemless scaling of Pods that require `GPUGroup`s.

```yaml
apiVersion: kai.scheduler/v1alpha1
# Defines a group of GPU devices that multiple Pods can request to be attached to 
kind: GPUGroupTemplate
metadata:
  name: gpu-group-template-1
  # The namespace of the GPUGroupTemplate, and the namespace that consuming Pods must reside in
  namespace: swap
spec:
  # Specifies metadata and spec of GPUGroups created from this template
  template:
    ...
status:
  # Kubernetes conditions to communicate state
  conditions: 
    ...
  # Specifies all GPUGroups created from this template
  templatedGPUGroupsNames:
  - gpu-group-template-1-abcd
  - gpu-group-template-1-efgh
```

The below manifest requires that Pods managed by the same Deployment will not have GPUs of the same `GPUGroup` attached to them:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: distinct-gpu-groups-consumers
  namespace: swap
spec:
  replicas: 2
  selector:
    matchLabels:
      app: distinct-consumer
  template:
    metadata:
      labels:
        app: distinct-consumer
        kai.scheduler/gpu-group-template: gpu-group-template-1
        kai.scheduler/gpu-group-unique-member-id: distinct-consumer
        kai.scheduler/queue: swap
    spec:
      schedulerName: kai-scheduler
      ...
```

However, the below manifest will allow Pods managed by the same Deployment to share the GPUs of the same `GPUGroup`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: same-gpu-groups-consumers
  namespace: swap
spec:
  replicas: 2
  selector:
    matchLabels:
      app: same-consumer
  template:
    metadata:
      labels:
        app: same-consumer
        kai.scheduler/gpu-group-template: gpu-group-template-1
        kai.scheduler/queue: swap
    spec:
      schedulerName: kai-scheduler
      ...
```

##### Semantics

- A Pod references a `GPUGroupTemplate`:
  - If no `GPUGroup` was created from this template yet, create one
  - For each `GPUGroup` that was created from this template, and until scheduling succeeded:
    - Attempt scheduling and attaching of the `GPUGroup`'s GPUs to the Pod
  - If all attempts failed due to `GPUGroup` scheduling constraints (`GPUGroup` reached `maxAttachedPods` or already has an identical `unique-member-id` attached), create a new `GPUGroup` from the `GPUGroupTemplate` and attempt scheduling to the new `GPUGroup`
- When the scheduler resolves a `GPUGroupTemplate` to a specific `GPUGroup`, it sets the `kai.scheduler/gpu-group` label on the Pod before creating a BindRequest. On binding failure, the scheduler may override this label to retry with a different `GPUGroup` from the same template. This label override is only permitted for Pods that also carry the `kai.scheduler/gpu-group-template` label

#### Notes

- `gpuGroup.spec.maxAttachedPods` is a `*int32` pointer: nil means unlimited, a non-nil value specifies the cap

---

# Swap Aware Scheduling Using `GPUGroupTemplates`

## Motivation

Allow users to deploy multiple workloads on shared GPU resources in a deterministic way by leveraging [Run:AI Swap](https://run-ai-docs.nvidia.com/self-hosted/platform-management/runai-scheduler/resource-optimization/memory-swap).

## Proposal

### API

#### New `SwapGroup` CRD is introduced:

A `SwapGroup` is a new resource that users can define and reference in their Pods. It gives them the ability to dynamically enable and configure the Run:AI Swap feature on their Nodes.

```yaml
apiVersion: runai.scheduler/v1alpha1
# Manages groups of GPUs that are shared utilizing Run:AI's Swap feature
kind: SwapGroup
metadata:
  name: swap-group-1
  # The namespace in which the underlying GPUGroupTemplate and GPUGroups are created
  namespace: swap
spec:
  config:
    # Bidirectional swap: parallel swap-in/swap-out (~80% faster)
    biDirectional:
      enabled: true
      # Streaming block size in MiB (max memory migrated per transaction)
      # Default: 2GiB
      blockSizeMB: 2048
      # Ratio between migrate-out and migrate-in chunk sizes
      # Migrate-out is larger to balance slower migrate-in (needs GPU allocation)
      # Default: 10%
      blockSizeRatioPct: 10
    # Resource limits for swap operations
    limits:
      # CPU RAM pool size per node for storing swapped GPU data
      # Shared across all GPU devices on the node
      cpuRam: "107374182400"  # 100GB
      # System-reserved GPU RAM for unmigratable memory (binaries, GPU context)
      # Recommendation: 1GB * expected workloads per GPU
      # Default: 2GiB (2147483648 bytes)
      reservedGpuRam: "2147483648"
      # RAM in MB reserved for unaccounted device memory
      # Required for mapped mode, defaults to 150MiB
      deviceReservedMB: 150
    # Swap mode: "mapped" (UVA-based) or "managed" (UVM-based)
    # mapped: better for newer GPUs (H100, B100), uses memory-mapped access
    # managed: default UVM mode
    mode: mapped
    
    #################################### <unknown>
    # Capability-level configuration (low-level, derived from features above)
    # Normally auto-populated by FillFeatureFlags(), shown here for completeness
    capabilities:
      swap:
        enabled: true
        ramLimit: "107374182400"
        reservedGpuRam: "2147483648"
        mode: "mapped"
        deviceReservedMB: 150
        biDirEnabled: true
        biDirBlockSizeMB: 2048
        biDirBlockSizeRatioPct: 10
      # These are implicitly enabled when swap is on:
      compute:
        enabled: true
        schedulerType: "snapshot"  # swap requires snapshot scheduler
      allocator:
        enabled: true
      memory:
        enabled: true
    #################################### </unknown>
  # Specifies metadata and spec of the GPUGroupTemplate
  gpuGroupTemplate:
    metadata:
      name: swap-group-1-gpu-group-template-1
    spec:
      # Specifies metadata and spec of the GPUGroups that will be created from this GPUGroupTemplate
      template:
        metadata:
          labels:
            # Required. Specifies from which Queue's quota the GPUs of GPUGroups created by this templates will be allocated
            runai.scheduler/queue: swap
        spec:
          # Required. Immutable. Number of GPUs to allocate for this GPUGroup attached to this GPUGroup
          gpuCount: 2
          # Optional. Mutable, can only be increased. If not specified (nil), unlimited. Specifies the maximum number of Pods that can be attached to this GPUGroup
          maxAttachedPods: 3
status:
  # Kubernetes conditions to communicate state
  conditions:
    ...
  # Status of the managed GPUGroupTemplate
  gpuGroupTemplateStatus:
    ...
```

This new API leverages the `GPUGroupTemplate` API in order to reserve and share GPU devices.
In addition to that, it will be responsible for reserving the CPU  memory requested by the user, and managing the GPU memory of the shared GPUs.

##### Semantics

- When a `SwapGroup` is created, a matching `GPUGroupTemplate` should be created automatically
- Pods reference the created `GPUGroupTemplate`
- Once a `GPUGroup` is created and set up, the scheduler should also, in addition to the required logic for the `GPUGroup`, create a `swap-reservation` Pod on the target Node

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: swap-group-member
  namespace: swap
  labels:
    runai.scheduler/swap-group: swap-group-1
    runai.scheduler/sawp-group-unique-member-id: swap-group-member-1
    kai.scheduler/queue: swap
spec:
  schedulerName: kai-scheduler
  ...
```

