# GPU Device Affinity API Proposal

## Context

Pods requesting GPU fractions need a way to deterministically co-schedule on the same physical GPU devices, or avoid sharing GPUs with specific pods. The API uses **label selectors** (like Kubernetes pod affinity) so it works naturally with Deployments where all replicas share the same pod template.

## Annotations

### `kai.scheduler/gpu-device-affinity` (soft)

Prefer GPUs that already have pods matching the given selector.

```
kai.scheduler/gpu-device-affinity: "<selector>"
```

- **Soft constraint**: prefers matching GPUs, falls back to a free GPU if no match.
- Scoring: +1.0 for each matching rule that has a matching pod on the GPU.

### `kai.scheduler/gpu-device-anti-affinity` (hard)

Must not use GPUs that have pods matching the given selector.

```
kai.scheduler/gpu-device-anti-affinity: "<selector>"
```

- **Hard constraint**: GPU is excluded if any pod on it matches. Pod stays pending if no GPU satisfies the constraint.

## Label Selector Syntax

```
key=value    match pods WITH label key=value
!key         match pods WITHOUT label key
```

**Separators:**
- `,` = AND within a single rule (all conditions must match the same pod)
- `;` = OR between independent rules (GPU excluded/scored if any rule matches any pod)

**Examples:**
```
"app=worker"             → pods labeled app=worker
"!role"                  → pods that lack the 'role' label
"app=worker; !role"      → pods matching app=worker OR pods lacking 'role'
"app=server,env=prod"    → pods with BOTH app=server AND env=prod
```

## Full Example: Two Deployments Pairing on GPUs

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: inference-server
spec:
  replicas: 3
  selector:
    matchLabels:
      app: inference-server
  template:
    metadata:
      labels:
        app: inference-server
        role: gpu-pair
      annotations:
        gpu-fraction: "0.5"
        gpu-fraction-num-devices: "2"
        # Hard: don't share GPUs with other inference-server replicas or standalone pods
        kai.scheduler/gpu-device-anti-affinity: "app=inference-server; !role"
        # Soft: prefer GPUs that already have a preprocessor pod
        kai.scheduler/gpu-device-affinity: "app=preprocessor"
    spec:
      schedulerName: kai-scheduler
      containers:
        - name: server
          resources:
            limits:
              nvidia.com/gpu: 1
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: preprocessor
spec:
  replicas: 3
  selector:
    matchLabels:
      app: preprocessor
  template:
    metadata:
      labels:
        app: preprocessor
        role: gpu-pair
      annotations:
        gpu-fraction: "0.5"
        gpu-fraction-num-devices: "2"
        # Hard: don't share GPUs with other preprocessor replicas or standalone pods
        kai.scheduler/gpu-device-anti-affinity: "app=preprocessor; !role"
        # Soft: prefer GPUs that already have an inference-server pod
        kai.scheduler/gpu-device-affinity: "app=inference-server"
    spec:
      schedulerName: kai-scheduler
      containers:
        - name: preprocess
          resources:
            limits:
              nvidia.com/gpu: 1
```

**Result** (on a node with 6+ GPUs):
- 3 pairs formed, each pair sharing 2 GPU devices
- No two inference-server replicas share a GPU
- No two preprocessor replicas share a GPU
- Standalone pods (without `role` label) cannot land on these GPUs

**How it works:**
- `!role` in anti-affinity keeps unaware pods off these GPUs (explicit isolation via label)
- `app=inference-server` in anti-affinity prevents server replicas from stacking
- `app=preprocessor` in affinity steers server pods toward GPUs with preprocessors (soft)

## Scheduling Behavior

For each candidate GPU on a node:

1. **Collect pods** already allocated on that GPU
2. **Anti-affinity — forward check (filter)**: Parse the scheduling pod's anti-affinity selector. For each rule (`;`-separated), check if any existing pod on the GPU matches all conditions (`,`-separated). If any rule matches → **exclude GPU**
3. **Anti-affinity — reverse check (filter)**: For each existing pod on the GPU that has an anti-affinity annotation, check if the scheduling pod matches its rules. If any rule matches → **exclude GPU**
4. **Affinity (score)**: Parse the scheduling pod's affinity selector. For each rule, if any existing pod on the GPU matches → **score += 1.0**

Anti-affinity is **bi-directional**: both the scheduling pod's rules and existing pods' rules are enforced. This means a pod on GPU-0 with `anti-affinity: "!role"` will block any new pod lacking the `role` label from that GPU — even if the new pod has no annotations itself.

## Semantics Summary

| Scenario | Behavior |
|---|---|
| Pod has `gpu-device-affinity: "app=X"` | Prefers GPUs with `app=X` pods (soft, +1.0 score) |
| Pod has `gpu-device-anti-affinity: "app=X"` | Excluded from GPUs with `app=X` pods (hard, forward check) |
| Pod has `anti-affinity: "!role"` | Excluded from GPUs with pods lacking `role` label |
| Pod has neither annotation | Can use any GPU, unless an existing pod's anti-affinity blocks it (reverse check) |
| Existing pod has `anti-affinity: "!role"`, new pod lacks `role` | GPU excluded — reverse check fires |
| All rules in both directions fail to match | GPU is eligible |
| Any single rule in either direction matches | GPU is excluded |
