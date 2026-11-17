# Contracts: F003 — Platform kit dữ liệu: entx + UnitOfWork, CQRS read/write, idempotency, outbox, modcall, testkit

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| — (component) | Parameter `IdempotencyKey`, `IfMatch`; header `Idempotent-Replayed` | `specs/openapi/eduworld-api.v1.yaml` | — | `IdempotencyKey`, `IfMatch` có sẵn; header response `Idempotent-Replayed` Cần thêm (§6) |
| — (component) | `ProblemCode` `IDEMPOTENCY_KEY_REQUIRED`, `IDEMPOTENCY_KEY_REUSED`, `IDEMPOTENCY_IN_PROGRESS`, `VERSION_MISMATCH`, `CONFLICT`, `VALIDATION_FAILED`, `UNAVAILABLE` | `specs/openapi/eduworld-api.v1.yaml` | — | Có sẵn |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| `resolvePrincipal` | mọi module domain → identity (adapter `PrincipalResolver` ở composition root, qua `pkg/modcall`) | `specs/openapi/modules/identity.internal.v1.yaml` | `learning`, `game`, `competition`, `chat` | Có sẵn (delta `issuer`/`subject`, `tenant_id` nullable: nội dung F012 §6, áp sớm ở F002 T005) |
| — (extension) | `x-context-required: [tenant_scope, idempotency_key]` (ngữ cảnh `modcall` kiểm), `x-allowed-callers`; schema `Problem` dùng chung. Không còn header ngữ cảnh (`X-Tenant-Id`, `X-On-Behalf-Of`, `traceparent`) trong spec internal — ngữ cảnh đi qua `context` (ADR-0074) | `specs/openapi/modules/*.internal.v1.yaml`, `common/internal-common.v1.yaml` | — | Có sẵn |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| — | Không dùng | — |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| (envelope mọi event) | Writer/Consumer validate | `specs/events/envelope.schema.json` | Có sẵn |
| `audit.recorded` | Phát: mọi module (`outbox.AuditRecorder`) và `realtime` (F090) · Tiêu thụ: module identity (F015) | `specs/events/payloads/common/audit.recorded.v1.schema.json` — owner F003 duy nhất (F012, F013, F015, F090 tham chiếu) | Cần thêm |
| `user_role.changed` | Tiêu thụ: `natsx.PrincipalInvalidator` | `specs/events/payloads/identity/user_role.changed.v1.schema.json` | Có sẵn |
| `role.updated` | Tiêu thụ: `natsx.PrincipalInvalidator` | `specs/events/payloads/identity/role.updated.v1.schema.json` | Có sẵn |
| `tenant_membership.changed` | Tiêu thụ: `natsx.PrincipalInvalidator` (owner F010) | `specs/events/payloads/identity/tenant_membership.changed.v1.schema.json` | Cần thêm |
| `user.access_revoked` | Tiêu thụ: `natsx.PrincipalInvalidator` (owner F011) | `specs/events/payloads/identity/user.access_revoked.v1.schema.json` | Cần thêm |
| `enrollment.changed` | Tiêu thụ: `natsx.PrincipalInvalidator` (owner F010) | `specs/events/payloads/identity/enrollment.changed.v1.schema.json` | Cần thêm |
| `tenant.updated` | Tiêu thụ: `natsx.PrincipalInvalidator` (owner F010) | `specs/events/payloads/identity/tenant.updated.v1.schema.json` | Cần thêm |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| — | Không dùng | — |

## 6. Delta cần thực hiện

- Thêm `specs/events/payloads/common/audit.recorded.v1.schema.json`: `actor_id` (uuid, nullable), `actor_type` (enum `user|service|system`), `action` (maxLength 64), `target_type` (64), `target_id` (128), `result` (enum `success|denied|error`), `reason` (1000, nullable), `metadata` (object, `maxProperties` 50, giá trị string maxLength 255) — khớp DATABASE_DESIGN §3.11; cập nhật bảng trong `specs/events/README.md`. Task T004.
- Thêm payload tối thiểu (`user_id` và/hoặc `tenant_id`, `version`) cho `tenant_membership.changed`, `user.access_revoked`, `enrollment.changed`, `tenant.updated` theo EVENT_DESIGN §4.1; feature phát (F010, F011, F012, F016) bổ sung trường nghiệp vụ. Task T005.
- Spec gộp: khai báo header response `Idempotent-Replayed` (`components.headers.IdempotentReplayed`, `type: string, enum: ["true"]`) và gắn vào response 2xx của operation có `IdempotencyKey`. Task T006.
