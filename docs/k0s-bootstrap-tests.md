# K0s Cluster Bootstrap with Flux - Test Implementation

## Task Summary

**Task ID**: forge-0bd
**Title**: Test: K0s Cluster Bootstrap with Flux
**Description**: Epic for K0s cluster deployment via layer API

## Implementation

Implemented comprehensive e2e test suite for validating K0s cluster deployment via the Layer API in the `infra-blocks` repository.

### Files Created/Modified

**In `/Users/ztaylor/repos/workspaces/infra-blocks/e2e/`:**

1. `k0s_bootstrap_test.go` - Main test file with Ginkgo BDD specs covering:
   - Layer API prerequisites and connectivity
   - VPC layer prerequisite creation
   - K0s cluster full bootstrap flow
   - Flux GitOps integration validation
   - Component versions (k0s, Cilium, Karpenter)
   - IRSA (IAM Roles for Service Accounts) configuration
   - Dex OIDC authentication setup
   - Health check verification (7 checks)
   - Layer lifecycle (creation, deletion, reconciliation)
   - Error scenarios and input validation

2. `client/layer_client.go` - Layer API HTTP client with:
   - Layer and LayerInstance types
   - Block, Operation, Health types
   - CRUD operations for layers and instances
   - Health check and reconciliation triggers

3. `blocks_suite_test.go` - Updated to initialize `layerClient`

4. `README.md` - Updated with K0s test documentation

## Test Categories

| Category | Tests |
|----------|-------|
| Layer API Prerequisites | API connectivity, list available layers |
| VPC Layer | VPC creation (prerequisite for K0s) |
| K0s Cluster Layer | Full bootstrap, input validation, error handling |
| Flux GitOps | Flux configuration, GitOps operation |
| Component Versions | k0s, Cilium, Karpenter versions |
| IRSA | IAM Roles for Service Accounts setup |
| Dex OIDC | Authentication configuration |
| Health Checks | All 7 health checks pass |
| Lifecycle | Deletion cleanup, reconciliation |

## Running Tests

```bash
cd /Users/ztaylor/repos/workspaces/infra-blocks/e2e

# Run all K0s bootstrap tests
GOWORK=off go test -v -run "K0s" -timeout 60m

# Run quick validation tests (skip long-running)
GOWORK=off go test -v -run "K0s" -short

# Full integration test (creates real AWS resources)
LAYER_API_URL=https://k0s-api.stigen.ai \
GOWORK=off go test -v -run "K0s.*Full" -timeout 90m
```

## Success Criteria

The full K0s bootstrap test verifies:

- [x] Layer instance reaches `active` state
- [x] All blocks in `active` state (VPC, IAM, EC2, S3, etc.)
- [x] All health checks pass (7/7)
- [x] Operations complete: `wait-k0s-bootstrap`, `install-cilium`, `flux-bootstrap`
- [x] Outputs available: `kubeconfig`, `cluster_endpoint`
- [x] Health status is `healthy`

## Architecture

The Layer API (`https://k0s-api.stigen.ai`) orchestrates:

```
Layer API → layer-applicator → blocks-service → work-queue → dataplane → AWS
                                                                     ↓
                                                            EC2 Instance
                                                            ├── k0s v1.31.3
                                                            ├── Dex OIDC
                                                            ├── Cilium CNI
                                                            ├── Karpenter
                                                            └── Flux GitOps
```

## Related Files

- Layer definition: `shapefile/layers/k0s-cluster/layer.cue`
- VPC layer: `shapefile/layers/vpc/layer.cue`
- Bootstrap plan: `docs/plans/01-k0s-bootstrap-with-flux.md`
- GitOps repo: `/Users/ztaylor/repos/workspaces/stigen-flux`
