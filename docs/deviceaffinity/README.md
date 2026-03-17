# GPU Device Affinity

## Goal

Allow users to define GPU device affinity between their pods.

## Motivation

Certain workloads benefit from the ability to share a specific GPU across their Pods.

This raises the need for an API that will allow Pods to define which other Pods they would like or would not like to share GPU devices with.

## Proposed API

Users will have the option to set a label on a Pod that defines a logical identifier for it that the scheduler uses for grouping Pods on a device.

Along with that label, users can define additional labels which represent affinity rules between Pods that want or don't want to share the same GPU device.

If a Pod rejects all gpu-groups due to its affinity rules, a new gpu-group should be allocated for this Pod

## Example Usage

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: pod-1
  annotations:
    # Required annotation to trigger the gpu-group flow
    gpu-fraction: "0.5"
    # When a pod needs multiple GPUs, this annotation should be specified
    gpu-fraction-num-devices: "2"
  labels:
    # Defines which sharing identifier devices allocated to this Pod should recieve
    kai.scheduler/gpu-sharing-identifier: "A"

    # Defines identifiers gpu-groups must have in order to be allocated to this Pod
    kai.scheduler/gpu-sharing-group-required-affinity: "B,C"
    # Defines identifiers gpu-groups are preferred to have in order to be allocated to this Pod
    kai.scheduler/gpu-sharing-group-preferred-affinity: "D,E"
 
    # Defines identifiers gpu-groups shouldn't have in order to be allocated to this Pod
    kai.scheduler/gpu-sharing-group-required-anti-affinity: "A"
    # Defines identifiers gpu-groups are preferred to not have in order to be allocated to this Pod
    kai.scheduler/gpu-sharing-group-preferred-anti-affinity: "A"

    # Specifies whether a free GPU is allowed to be allocated for this Pod if no affinity constraints are valid
    kai.scheduler/gpu-sharing-group-allow-free-gpu-allocation: "true"
spec:
  ...
---
apiVersion: v1
kind: Pod
metadata:
  name: pod-2
  annotations:
    # Required annotation to trigger the gpu-group flow
    gpu-fraction: "0.5"
    # When a pod needs multiple GPUs, this annotation should be specified
    gpu-fraction-num-devices: "2"
  labels:
    # Defines which sharing identifier devices allocated to this Pod should recieve
    kai.scheduler/gpu-sharing-identifier: "A"

    # Defines identifiers gpu-groups must have in order to be allocated to this Pod
    kai.scheduler/gpu-sharing-group-required-affinity: "B,C"
    # Defines identifiers gpu-groups are preferred to have in order to be allocated to this Pod
    kai.scheduler/gpu-sharing-group-preferred-affinity: "D,E"
 
    # Defines identifiers gpu-groups shouldn't have in order to be allocated to this Pod
    kai.scheduler/gpu-sharing-group-required-anti-affinity: "A"
    # Defines identifiers gpu-groups are preferred to not have in order to be allocated to this Pod
    kai.scheduler/gpu-sharing-group-preferred-anti-affinity: "A"

    # Specifies whether a free GPU is allowed to be allocated for this Pod if no affinity constraints are valid
    kai.scheduler/gpu-sharing-group-allow-free-gpu-allocation: "true"
spec:
  ...
---
apiVersion: v1
kind: Pod
metadata:
  name: pod-3
  annotations:
    # Required annotation to trigger the gpu-group flow
    gpu-fraction: "0.5"
    # When a pod needs multiple GPUs, this annotation should be specified
    gpu-fraction-num-devices: "2"
  labels:
    # Defines which sharing identifier devices allocated to this Pod should recieve
    kai.scheduler/gpu-sharing-identifier: "B"

    # Defines identifiers gpu-groups must have in order to be allocated to this Pod
    kai.scheduler/gpu-sharing-group-required-affinity: "A,C"
    # Defines identifiers gpu-groups are preferred to have in order to be allocated to this Pod
    kai.scheduler/gpu-sharing-group-preferred-affinity: "D,E"
 
    # Defines identifiers gpu-groups shouldn't have in order to be allocated to this Pod
    kai.scheduler/gpu-sharing-group-required-anti-affinity: "B"
    # Defines identifiers gpu-groups are preferred to not have in order to be allocated to this Pod
    kai.scheduler/gpu-sharing-group-preferred-anti-affinity: "B"

    # Specifies whether a free GPU is allowed to be allocated for this Pod if no affinity constraints are valid
    kai.scheduler/gpu-sharing-group-allow-free-gpu-allocation: "true"
spec:
  ...
---
apiVersion: v1
kind: Pod
metadata:
  name: pod-4
  annotations:
    # Required annotation to trigger the gpu-group flow
    gpu-fraction: "0.5"
    # When a pod needs multiple GPUs, this annotation should be specified
    gpu-fraction-num-devices: "2"
  labels:
    # Defines which sharing identifier devices allocated to this Pod should recieve
    kai.scheduler/gpu-sharing-identifier: "B"

    # Defines identifiers gpu-groups must have in order to be allocated to this Pod
    kai.scheduler/gpu-sharing-group-required-affinity: "A,C"
    # Defines identifiers gpu-groups are preferred to have in order to be allocated to this Pod
    kai.scheduler/gpu-sharing-group-preferred-affinity: "D,E"
 
    # Defines identifiers gpu-groups shouldn't have in order to be allocated to this Pod
    kai.scheduler/gpu-sharing-group-required-anti-affinity: "B"
    # Defines identifiers gpu-groups are preferred to not have in order to be allocated to this Pod
    kai.scheduler/gpu-sharing-group-preferred-anti-affinity: "B"

    # Specifies whether a free GPU is allowed to be allocated for this Pod if no affinity constraints are valid
    kai.scheduler/gpu-sharing-group-allow-free-gpu-allocation: "true"
spec:
  ...
```

The scheduling result of the above should be the following: 

- Pods `pod-1` and either `pod-3` or `pod-4` should have the same GPU device attached to them
- Pods `pod-2` and either `pod-3` or `pod-4` (whichever was not grouped with `pod-1` shuold have the same GPU device attached to them

## Implementation Details

GPU device affinity is implemented as a scheduler plugin (`gpudeviceaffinity`) that registers a `GpuOrderFn`. The scheduler calls this function for each candidate GPU when selecting GPUs for a fractional-GPU pod. The function returns a score to influence GPU selection and an error to filter out a GPU entirely.

### Core Model

- Each pod declares a `gpu-sharing-identifier` label. When the pod is allocated to a GPU group, that GPU group receives the identifier.
- A GPU group's set of identifiers is the union of all `gpu-sharing-identifier` values from pods currently allocated on it.
- Affinity rules on a scheduling pod specify which identifiers a GPU group must/should (or must not/should not) have.

### Constraint Types


| Label                                       | Type | Behavior                                                                                            |
| ------------------------------------------- | ---- | --------------------------------------------------------------------------------------------------- |
| `gpu-sharing-group-required-affinity`       | Hard | GPU group **must** have all listed identifiers. GPU is filtered out (error) if any are missing.     |
| `gpu-sharing-group-required-anti-affinity`  | Hard | GPU group **must not** have any listed identifiers. GPU is filtered out (error) if any are present. |
| `gpu-sharing-group-preferred-affinity`      | Soft | Score **+1.0** per matched identifier found on the GPU group.                                       |
| `gpu-sharing-group-preferred-anti-affinity` | Soft | Score **-1.0** per matched identifier found on the GPU group.                                       |


Multiple identifiers are specified as comma-separated values (e.g. `"B,C"`).

### Free GPU Allocation

The `gpu-sharing-group-allow-free-gpu-allocation` label controls whether a pod can be placed on a free (empty) GPU when no existing GPU group satisfies its constraints:

- `"true"` (default when absent): The pod is allowed to fall back to a free GPU with a neutral score of 0.
- `"false"`: The pod is blocked from using a free GPU — it must land on an existing GPU group that satisfies its affinity rules or remain unscheduled on the node.

### Multi-Device Pods

For pods requesting multiple GPU devices (`gpu-fraction-num-devices > 1`), the scheduler selects GPUs one at a time via `GetNodePreferableGpuForSharing`. The `GpuOrderFn` is called independently for each GPU group selection, so affinity scoring applies per-GPU naturally without special handling.

### Scoring Priority

Affinity scores (+1.0 / -1.0 per match) dominate over packing/spreading scores (0–1 range), ensuring affinity rules take priority over GPU packing or spreading strategies.