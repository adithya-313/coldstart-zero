# ColdStart-Zero v2

## System Overview & Problem Statement

Large Language Model inference serving suffers from a fundamental cold-start problem:
loading multi-gigabyte weight files from remote artifact stores (S3, GCS, ECR) on
every pod creation adds **10–15 minutes** of startup latency and saturates network
links with **10GB+ of egress traffic per replica**.

ColdStart-Zero v2 decouples **state from application logic**:

- **State** (model weights) is cached on the node via `hostPath` volumes and survives
  pod restarts through SHA256-verified idempotent provisioning.
- **Application logic** (the inference micro-API) ships as a **sub-15MB distroless
  Go binary** that boots in microseconds and occupies negligible disk.
- **Warm starts** (cache hit) complete in **~3.3 seconds** — the entire
  init-container verification + API boot cycle — reducing cold-start overhead
  by over 99.9%.

---

## Architecture & Tradeoffs

### 1. Node-Level Caching (`hostPath` over Network-Attached Storage)

| Approach | Latency | IOPS | Statefulness |
|---|---|---|---|
| S3/GCS download per pod | 10–15 min | N/A | Stateless |
| PVC (network FS) | 100–500 ms | Shared-bottleneck | Stateful (slow) |
| **hostPath (local SSD)** | **< 1 ms** | **Node-isolated** | **Stateful (fast)** |

**Tradeoff:** `hostPath` ties data to a specific node, but this is acceptable
because the weights are read-only after provisioning and the init-container
idempotency guarantees cache integrity via SHA-256.

### 2. Zero-Trust Security (`tmpfs` Secrets over Environment Variables)

Environment variables leak credentials into:
- `docker inspect` output
- Process crash dumps (`/proc/*/environ`)
- CI/CD log artifacts

ColdStart-Zero v2 uses **Docker Secrets** mounted as `tmpfs` files inside the
provisioning container. The secret file lives **only in RAM** and dies when
the container stops. The application never sees the raw secret — only a masked
log line (`hf_mock_token_***`).

**Validation:** `docker inspect <container> | grep hf_mock_token` yields zero
results (confirmed).

### 3. Distroless Micro-API (Go `net/http` over Python/FastAPI)

| Metric | Python/FastAPI (typical) | **Go/scratch (this project)** |
|---|---|---|
| Image size | ~120–300 MB | **10.9 MB** |
| Boot time | ~1–5 s | **4 µs** |
| p99 latency @ 50 concurrency | ~50–200 ms | **7.65 ms** |
| Startup order | Requires init-system | **Static binary, no dependencies** |

**Tradeoff:** Go lacks Python's rich ML ecosystem. This is acceptable because
the micro-API is purely an inference gateway — model execution happens on GPU
nodes via a separate sidecar (see Scaling Path).

---

## Empirical Telemetry

All measurements taken on Docker Desktop (Windows) against `coldstart-api:v1`.

| Metric | Value | Conditions |
|---|---|---|
| **Internal Go Boot Time** | **4 µs** | Time from `main()` to `ListenAndServe` |
| **API p99 Latency** | **7.65 ms** | 50 concurrent VUs, 7252 requests over 20 s |
| **API p95 Latency** | **5.72 ms** | Same load profile |
| **Total Image Size** | **10.9 MB** | `scratch`-based distroless build |
| **Infrastructure Warm Boot** | **~3.3 s** | `docker-compose up` → HTTP 200 (cache hit) |
| **Avg Request Rate** | **361 req/s** | Sustained under 50 concurrent users |
| **Failure Rate** | **0%** | Zero dropped requests |

---

## Directory Structure

```
coldstart-zero/
├── .github/workflows/ci.yml   # CI pipeline: vet → build → size gate → k8s-lint
├── k8s/
│   ├── 01-secret.yaml          # Opaque secret (cloud_token)
│   ├── 02-deployment.yaml      # Deployment: initContainer + distroless API
│   └── 03-service.yaml         # ClusterIP:80 → 8080
├── secrets/                    # (gitignored) mock cloud_token.txt
├── node_cache/                 # (gitignored) cached weights + SHA256
├── docs/
│   └── benchmark-report.json   # Hard telemetry from empirical validation
├── main.go                     # Go micro-API: /health, /predict, /metrics
├── Dockerfile                  # Multi-stage: golang:1.21-alpine → scratch
├── docker-compose.yml          # init-provisioner + api orchestration
├── provision.sh                # Idempotent SHA256-verified cache script
├── loadtest.js                 # k6 load test (50 VU, 20 s)
├── .yamllint                   # Relaxed lint rules for k8s manifests
└── .gitignore
```

---

## Scaling Path (Production Evolution)

Before deploying a real 15 GB model (e.g., Llama 3 70B), the following changes
are required:

### Hardware: GPU Nodes

Transition from CPU-only Docker Desktop to GPU-accelerated nodes
(e.g., **AWS g4dn.xlarge** with NVIDIA T4). The Go micro-API remains on CPU
for request routing; inference runs on GPU via a sidecar.

### Orchestration: DaemonSet over InitContainer

The current architecture places one init-container per Pod. When 50 replicas
restart simultaneously, each runs its own SHA256 verification — this is the
**Thundering Herd** problem.

**Fix:** Move the weight-provisioning logic to a **K8s DaemonSet** that runs
once per node and writes weights to the shared `hostPath`. Pods then mount
the pre-warmed cache and start instantly. This also reduces network egress to
**one download per node** instead of one per replica.

### Compilation: CGO for GPU Binding

Recompile the Go binary with `CGO_ENABLED=1` and link against:
- **CUDA** (NVIDIA GPU acceleration)
- **AVX2** (CPU-side vectorization for token generation)
- **go-llama.cpp** (LLM inference engine)

This increases the binary size (~40 MB with CUDA stubs) but remains well
under the 100 MB mark. The `scratch` base image must be replaced with a
`gcr.io/distroless/cc-debian12` base that includes `libc`, `libcuda`, and
`libstdc++`.

### Production Checklist

- [ ] Replace mock token with Vault/KMS-backed secrets
- [ ] Add horizontal pod autoscaler (HPA) based on request latency
- [ ] Implement graceful shutdown with draining in `main.go`
- [ ] Set resource requests/limits on the Deployment
- [ ] Add Prometheus ServiceMonitor for automatic scraping
- [ ] Enable structured JSON logging (`log/slog`) for log aggregators
