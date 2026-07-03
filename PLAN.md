# Scaffold Implementation Plan

Detected module owner: `hlfshell`. Module paths in this workspace use `github.com/hlfshell/...`.

Toolbox modules live under the top-level `toolbox/` folder and use short service names, for example `toolbox/redis` with module path `github.com/hlfshell/scaffold-toolbox/redis`.

## Project setup

- [x] Create `scaffold/` workspace
- [x] Clone `docker-harness` into `reference/docker-harness`
- [x] Copy reference implementation patterns into Scaffold core
- [x] Rename project references from docker-harness to Scaffold
- [ ] Initialize local git repositories
- [ ] Create or document remote repository setup

## Core Scaffold

- [x] Implement core `Container`
- [x] Implement lifecycle methods
- [x] Implement service interface
- [x] Implement stack abstraction
- [x] Implement cleanup ordering
- [x] Implement simple readiness waiters
- [x] Implement shared Docker network support
- [x] Implement preload/setup hook pattern
- [x] Add tests
- [x] Add README examples

## Existing harness migration

- [x] Move Postgres into standalone repo
- [x] Move MySQL into standalone repo
- [x] Move Redis into standalone repo
- [x] Move Memcached into standalone repo
- [x] Update imports to Scaffold
- [x] Add preload helpers
- [x] Add docs and examples

## New data/database harnesses

- [ ] MongoDB
- [ ] ClickHouse
- [x] Qdrant
- [ ] Weaviate
- [x] MinIO
- [ ] Trino
- [ ] Iceberg-compatible local data lake
- [ ] Optional OpenSearch
- [ ] Optional Neo4j

## Cloud harnesses

- [x] LocalStack base harness
- [ ] AWS S3 helpers
- [ ] AWS SQS helpers
- [ ] AWS SNS helpers
- [ ] AWS DynamoDB helpers
- [ ] AWS Secrets Manager helpers
- [ ] AWS EventBridge helpers
- [ ] AWS preset stack

## AI/dev harnesses

- [ ] Ollama
- [ ] LiteLLM
- [ ] Mailpit
- [ ] Toxiproxy
- [ ] OpenTelemetry Collector
- [ ] NATS
- [ ] Redpanda

## Kubernetes and Argo

- [ ] Mini Kubernetes harness using kind or k3d
- [ ] Argo CD stack
- [ ] Argo Workflows stack
- [ ] Examples for applying manifests
- [ ] Cleanup and teardown docs

## Preset stacks

- [x] RAG stack
- [ ] SaaS backend stack
- [ ] Event-driven AWS stack
- [ ] Data lake stack
- [ ] AI agent backend stack
- [ ] Kubernetes workflow stack

## Documentation

- [x] Core README
- [ ] Harness authoring guide
- [ ] Stack authoring guide
- [ ] Preload data guide
- [ ] Testing guide
- [ ] Development environment guide
- [ ] Client-side service management guide

## Review and feature pass

- [x] Read `scaffold/review.md`
- [x] Convert review findings into concrete implementation tasks
- [x] Protect reserved scaffold labels from inherited/user override
- [x] Validate stack names, nil services, and duplicate service names
- [x] Make stack `Create` non-reentrant
- [x] Cleanup all services whose `Create` was invoked when a group fails
- [x] Add nested stack network inheritance
- [x] Track shared network ownership and only remove owned networks
- [x] Use Docker API version negotiation for all Docker clients
- [x] Make same-name container cleanup label-safe
- [x] Cleanup partially created containers when `Start` fails
- [x] Let Docker assign host ports and inspect actual bindings
- [x] Default host binds to `127.0.0.1` with an option for public bind
- [x] Clone container input/output maps
- [x] Add container port helper APIs
- [x] Stop writing Docker pull output to stdout by default
- [x] Fix `WaitForLogText` EOF/timeout behavior
- [x] Add container log reader helpers
- [x] Remove unused core `Preloadable` interface
- [x] Implement high-confidence review fixes
- [x] Update tests for review fixes
- [x] Read `scaffold/features.md`
- [x] Convert requested features into concrete implementation tasks
- [x] Add `EnvProvider` and `Stack.Env()`
- [x] Add `Stack.WriteEnvFile(path)`
- [x] Add `EndpointProvider`, `Stack.Endpoint(name)`, and endpoint aggregation
- [x] Add `Stack.Summary()`
- [x] Remove `Doctor()` from the framework
- [x] Add `Run(stack, fn)` helper
- [x] Add `RunContext(ctx, stack, fn)` helper
- [x] Add `testutil.RequireDocker(t)`
- [x] Add context-aware lifecycle API (`CreateContext`, `CleanupContext`) without breaking `Service`
- [x] Add context-aware container lifecycle methods and waiters
- [x] Add context lifecycle support to implemented toolbox services
- [x] Add generated name prefix support where safe
- [x] Add generated name prefix support to implemented toolbox services
- [x] Surface service groups in summaries
- [x] Implement high-confidence feature work
- [x] Update docs for changed behavior
- [x] Run formatting
- [x] Run workspace tests
- [x] Record any deferred or blocked items

Deferred from this pass:

- [ ] Stack-level preload sequencing was not implemented because service-level preload is already explicit and the review suggested removing unused preload interfaces.
- [ ] `DeleteImage` remains available but should be reconsidered before a stable API.
- [ ] Stub toolbox modules remain intentionally stubbed until their service designs are ready.
