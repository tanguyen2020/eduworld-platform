# Implementation Plan: F116 — Pipeline kiểm tra tin nhắn, danh mục từ nhạy cảm, công tắc kiểm duyệt

**Branch**: `116-chat-moderation-pipeline` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Thêm module `moderation` (phần pipeline + danh mục + công tắc) vào module chat. `ContentValidator` (port của F115) được hiện thực bởi `pipeline.Validator` chạy in-process: `TechnicalGate` (độ dài, rate, sanction — luôn chạy) → `SwitchResolver` → `Normalizer` (NFC, lowercase, `vi_fold`, teencode, leetspeak, gộp lặp, bỏ chen giữa, homoglyph) → `FormatStep` → `SensitiveTermStep` (Aho–Corasick trên mọi góc nhìn + RE2 timeout) → `PIIStep` → `SpamStep` (Valkey cửa sổ trượt, auto-mute) → `BullyingStep` (rule) → `ContextRules` → `Decision`. Danh mục biên dịch theo `(tenant_id, catalog_version_platform, catalog_version_tenant)`, cache `ew:chat:sensitive:compiled:*`, hoán đổi `atomic.Pointer` khi nhận `ew:chat:sensitive:version`. Command quản trị danh mục/công tắc (write context, authz tường minh đầu handler), endpoint `/admin/chat/*` gateway nội bộ. Job `moderation-switch-expirer`. Quick message: query `listQuickMessages` + kiểm `kind = quick`.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 |
| Primary Dependencies | ent, Atlas, oapi-codegen, `golang.org/x/text/unicode/norm`, Aho–Corasick (`github.com/cloudflare/ahocorasick` hoặc tự viết trong `internal/domain/moderation/ac`), RE2 (`regexp` chuẩn Go), `valkey-go`, `src/backend/pkg/entx` (`UnitOfWork`), `src/backend/pkg/outbox`, `src/backend/pkg/idempotency`, `src/backend/pkg/authz` |
| Storage | PostgreSQL `eduworld_chat` (`chat.sensitive_term_categories`, `chat.sensitive_terms`, `chat.sensitive_allowlist`, `chat.sensitive_catalog_versions`, `chat.tenant_category_settings`, `chat.moderation_switches`, `chat.moderation_switch_changes`, `chat.moderation_decisions` P tháng, `chat.sanctions`, `chat.quick_messages`); Valkey (`ew:chat:sensitive:*`, `ew:chat:switches:*`, `ew:rl:chat:*`, cửa sổ spam) |
| Testing | go test (unit bảng normalize/engine), benchmark CI (`go test -bench`), testcontainers (PG, Valkey, NATS); bộ hồi quy placeholder trong CI |
| Target Platform | Linux container: module `chat` nạp trong `core-api` + profile `worker-chat` (job expirer, phát pub/sub version) |
| Project Type | backend module |
| Performance Goals | `moderation_pipeline_latency` (NFR-SAFE-002); reload ≤ 60 s |
| Constraints | AGENTS R14, R15, R20, R21, R26, R27, R28; ADR-0020, ADR-0030, ADR-0056, ADR-0072 |
| Scale/Scope | ≤ 200 pattern/tenant [Proposed]; danh mục platform + bổ sung mỗi tenant |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Operation danh mục/công tắc có sẵn; delta (`category_code` trong Problem, `params` quick, event payload) ở Phase 2 |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Chỉ module chat; engine thuần ở domain; không gọi module khác trên đường gửi |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Bảng H/T có RLS; term tenant chỉ hiệu lực tenant đó; xóa mềm; version append-only |
| IV Server-authoritative | ✅ | Quyết định ở server; ghi quản trị có `Idempotency-Key`/`If-Match` + audit |
| V Xác thực, phân quyền | ✅ | `chat.sensitive_terms.read/write`, `chat.policy.toggle`, `chat.tenant_terms.write`, `chat.tenant_category.toggle` platform-only (catalog `src/backend/pkg/authz` kiểm lúc khởi động) |
| VI An toàn trẻ em | ✅ | R15 đầy đủ: không hardcode từ, không lộ từ khớp, không cờ tắt trong code/env, không điều kiện khối |
| VII Một nguồn sự thật, cấu hình | ✅ | Tham số theo CONFIG_DEFAULTS; ngưỡng spam đề xuất đăng ký |
| VIII UI tách logic, token | ✅ | Không có UI (Portal ở F122) |
| IX Test có bằng chứng | ✅ | Test mọi F116-AC-*, benchmark CI, test hồi quy placeholder |

## Project Structure

### Documentation (feature này)

```text
specs/116-chat-moderation-pipeline/
├── spec.md · plan.md · research.md · data-model.md · quickstart.md · tasks.md
├── contracts/README.md
└── checklists/requirements.md
```

### Source Code (repository)

```text
src/backend/modules/chat/
├── ent/schema/        # sensitivetermcategory.go, sensitiveterm.go, sensitiveallowlist.go, sensitivecatalogversion.go,
│                      # tenantcategorysetting.go, moderationswitch.go, moderationswitchchange.go, moderationdecision.go, sanction.go, quickmessage.go
├── internal/domain/moderation/
│   ├── normalize/     # normalizer.go (views), vi_fold.go, leet.go, homoglyph.go
│   ├── catalog/       # compiled_catalog.go, ac_matcher.go, re2_matcher.go, variants.go
│   ├── pii/ spam/ bullying/ context/
│   ├── decision.go    # Decision, Action, severity→action, strictness bump
│   └── switches.go    # EffectiveSwitches (AND platform ∧ tenant)
├── internal/application/pipeline/validator.go        # hiện thực port ContentValidator (F115)
├── internal/application/command/                     # catalog_*.go, publish_catalog.go, tenant_terms_*.go, toggle_tenant_category.go, set_switches.go, expire_switches.go, apply_system_mute.go
├── internal/application/query/                       # list_categories.go, list_terms.go, test_terms.go, list_versions.go, list_switches.go, list_quick_messages.go
├── internal/infrastructure/catalogstore/             # compiled cache Valkey, version pub/sub subscriber, periodic reconcile
├── internal/infrastructure/regression/               # chạy bộ hồi quy (đọc object storage)
├── internal/presentation/http/admin/                 # sensitive_*.go, moderation_switches.go, tenant_terms.go, tenant_categories.go
├── internal/presentation/http/public/quick_messages.go
├── internal/presentation/job/moderation_switch_expirer.go
└── tests/{unit,integration,bench,static}/
```

**Structure Decision**: Engine và normalize là domain thuần (không I/O) để benchmark và test bảng; tải/biên dịch danh mục ở infrastructure.

## Phase 0: Research

→ [research.md](research.md): R1 biên dịch và hoán đổi danh mục, R2 decision log ghi ở đâu, R3 lưu tin `held`.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `Normalizer`, `CompiledCatalog.Match(views) []Hit`, `PIIDetector`, `SpamRules`, `BullyingRules`, `ContextRules`, `Decide(hits, strictness)`, `EffectiveSwitches` | Không tham số khối/lớp |
| application | `pipeline.Validator` (port F115); Command: `CreateSensitiveCategory`, `UpdateSensitiveCategory`, `DeleteSensitiveCategory`, `CreateSensitiveTerm`, `UpdateSensitiveTerm`, `DeleteSensitiveTerm`, `CreateAllowlistEntry`, `DeleteAllowlistEntry`, `PublishSensitiveCatalog`, `CreateTenantSensitiveTerm`, `UpdateTenantSensitiveTerm`, `DeleteTenantSensitiveTerm`, `PublishTenantSensitiveCatalog`, `ToggleTenantSensitiveCategory`, `SetModerationSwitches`, `ExpireModerationSwitches`, `ApplySystemMute`; Query: `ListSensitiveCategories`, `ListSensitiveTerms`, `TestSensitiveTerms`, `ListSensitiveAllowlist`, `ListSensitiveCatalogVersions`, `ListModerationSwitches`, `ListModerationSwitchHistory`, `ListTenantSensitiveTerms`, `ListTenantSensitiveCategories`, `ListQuickMessages` | Permission theo [contracts §1](contracts/README.md) |
| infrastructure | ent repo; Valkey compiled cache + pub/sub `ew:chat:sensitive:version`, `ew:chat:switches:changed`; spam window; regression runner | |
| presentation | 20 handler `/admin/chat/*`, `/tenant/chat/categories`, `listQuickMessages`; job `moderation-switch-expirer` (worker) | |

### Tích hợp và phụ thuộc

- F115: port `ContentValidator`, `SendMessage`. F121 tiêu thụ `chat.message_held`, đọc `chat.sanctions`, `chat.moderation_decisions`.
- Event phát: `sensitive_term.created/updated/deleted`, `sensitive_catalog.published`, `chat_policy.moderation_switch_changed`, `chat_policy.tenant_category_toggled`, `chat.message_held`, `sanction.applied`, `audit.recorded`.

### Bảo mật, tenant, idempotency, audit

- Endpoint `/admin/chat/*`: gateway nội bộ, middleware `portal_staff` + MFA (`acr`) + permission; ghi của System Admin luôn có tenant đích khi `scope = tenant` (R20). Đọc `tenant_ids` dùng `CrossTenantScope` + audit.
- `Idempotency-Key` cho create/publish/toggle/set; `If-Match` cho update.
- Audit: mọi thay đổi danh mục (kèm `catalog_version`, không nội dung term), mọi bật/tắt công tắc, `testSensitiveTerms` (xem nội dung danh mục), áp sanction hệ thống.

### Quan sát

- Metric: `eduworld_chat_filter_latency_seconds`, `eduworld_chat_sensitive_matches_total{category,action}`, `eduworld_chat_unchecked_messages_total{tenant}`, `eduworld_chat_policy_disabled{tenant,step}`, `eduworld_chat_sensitive_catalog_version{scope}`, `eduworld_chat_re2_timeouts_total`. Alert `ChatModerationDisabled`.

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
