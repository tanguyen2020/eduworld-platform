# Contracts: F002 — Platform kit lõi: config, observability, problem, httpx, authz, tenancy

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| — (component) | Schema `Problem`, enum `ProblemCode`, parameter `TenantIdHeader`, `TenantIdHeaderOptional` | `specs/openapi/eduworld-api.v1.yaml` | — | Có sẵn — `pkg/problem` sinh hằng từ `ProblemCode`; `httpx` dùng làm validator |
| mọi operation | `x-service`, tham số `X-Tenant-Id` (required/optional/không có) | `specs/openapi/eduworld-api.v1.yaml` | — | Có sẵn — đầu vào `routematrix` và danh sách route miễn header |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| `resolvePrincipal` | mọi module domain → identity (qua port `authz.PrincipalResolver`; adapter facade thật ở F003; binary `realtime` không gọi facade, lệnh của nó được dựng Principal trong `core-api`) | `specs/openapi/modules/identity.internal.v1.yaml` (owner nội dung: F012) | `learning`, `game`, `competition`, `chat` | Cần sửa (§6, theo F012) |
| `getTenant` | mọi module domain → identity (port `TenantInfoProvider`) | `specs/openapi/modules/identity.internal.v1.yaml` | như trên | Có sẵn |
| — (component) | `Problem` và schema dùng chung (không còn `serviceToken` hay header ngữ cảnh; ngữ cảnh đi qua `context`, ADR-0074) | `specs/openapi/modules/common/internal-common.v1.yaml` | — | Có sẵn |
| mọi operation internal | `x-allowed-callers` sinh vào `api/allowed_callers_gen.go` (dùng bởi `pkg/modcall`, F003) | `specs/openapi/modules/*.internal.v1.yaml` | — | Có sẵn |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| — | Không dùng | — |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| `audit.recorded` | Phát gián tiếp qua `audit.Recorder` (hiện thực F003) | xem contracts của F003 | Cần thêm (F003) |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| — | Không dùng | — |

## 6. Delta cần thực hiện

- `identity.internal.v1.yaml` `resolvePrincipal`: áp nguyên delta do F012 sở hữu ([F012 contracts §6](../../012-authz-portal-session/contracts/README.md)): định danh `issuer` + `subject` (oneOf `user_id`), `tenant_id` nullable (phạm vi platform: System Admin/role platform không gửi `X-Tenant-Id`), `Principal.tenant_id`/`tenant_type` nullable, `is_system_admin`, response `404` = "người dùng không có membership tenant yêu cầu" → `authz.ErrNotMember` → `403 TENANT_MISMATCH`. F002 chỉ áp sớm ở M-C0 (task T005), không định nghĩa nội dung riêng.
- Enum `ProblemCode` (`eduworld-api.v1.yaml` và `modules/*.v1.yaml`, mô tả response `403`): gỡ `FORBIDDEN_SCOPE`; vượt phạm vi trường/môn trả `PERMISSION_DENIED` (API_DESIGN §4.1, RB-113). Task T055.
- TEST_STRATEGY TC-SVC-006: kỳ vọng `403 CALLER_NOT_ALLOWED` thay `PERMISSION_DENIED` (tài liệu, task T051).
