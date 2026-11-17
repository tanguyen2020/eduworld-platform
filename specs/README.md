# EduWorld Specs

> Trạng thái: Draft · Phiên bản: 0.5 (modular monolith, ADR-0074) · Owner: Backend Architect · Cập nhật: 2026-11-17 (ADR-0074: spec internal là hợp đồng facade, bỏ `serviceToken`, `x-allowed-callers` dùng tên module, thêm `x-context-required`)
> Liên quan: [ADR-0081](../docs/adr/ADR-0081-user-type-student-staff-phan-biet-bang-tenant.md) · [ADR-0082](../docs/adr/ADR-0082-spec-driven-development-va-noi-dung-deferred.md) · [API_DESIGN](../docs/07-backend/API_DESIGN.md) · [REALTIME_ARCHITECTURE](../docs/06-architecture/REALTIME_ARCHITECTURE.md) · [EVENT_DESIGN](../docs/07-backend/EVENT_DESIGN.md) · ADR-0010 · [ADR-0074](../docs/adr/ADR-0074-modular-monolith-co-ranh-gioi-cuong-che.md) · ADR-0040 · ADR-0041

Đặc tả máy đọc là **nguồn sự thật cho contract** giữa Godot client, CMS/LMS web, các module của backend Go (modular monolith, ADR-0074) và hệ thống ngoài (SIS). Quy trình: sửa spec → review → sinh code → triển khai (contract-first, ADR-0010).

## Cấu trúc

| Đường dẫn | Chuẩn | Nội dung | Nguồn sự thật? | Sinh code / dùng ở đâu |
|---|---|---|---|---|
| `openapi/eduworld-api.v1.yaml` | OpenAPI 3.1 | **Spec public gộp** `/api/v1/*` (372 path, 441 operation, 419 schema, 164 `ProblemCode`; header `X-Tenant-Id` bắt buộc ở 278 operation, tùy chọn ở 93, không gửi ở 61 — ADR-0065). Mọi operation có `x-service` (module sở hữu, ADR-0074); `/game-admin/{config_type}` có thêm `x-service-routing`. Operation Admin Portal gọi có `x-portal-action` (khóa action trong `portal/menu-catalog.yaml`, dạng `{param, map}` cho operation định tuyến theo tham số) hoặc `x-portal-shell: true` (ADR-0090). `/sis/*`, `/admin/*` gắn `x-internal: true`. Enum học tập (`QuestionType`, `ActivityType`, `FeedbackTiming`, `ResultRelease`, `AttemptMode`, `AttemptStatus`, `LearningContentType`) khai báo inline trong `components.schemas`, giá trị y hệt `schemas/common/enums.schema.json`; `QuestionResponse` = `schemas/question.schema.json#/$defs/response` | **Có** — sửa tay ở đây | Web: `openapi-typescript` · Godot: API layer viết tay theo spec, kiểm tra bằng contract test |
| `openapi/modules/<module>.v1.yaml` (`identity`, `learning`, `game`, `competition`, `chat`) | OpenAPI 3.1 | Spec public của từng module, **sinh tự động** từ spec gộp bởi `scripts/openapi/split-public.mjs` (chỉ giữ component được tham chiếu) | Không — **không sửa tay** | Go: `oapi-codegen` strict server cho handler public của từng module (`modules/<module>/internal/presentation/http/gen`), cùng chạy trong binary `core-api` |
| `openapi/modules/<module>.internal.v1.yaml` | OpenAPI 3.1 | **Hợp đồng facade giữa module** (ADR-0074, ADR-0040): identity (`resolvePrincipal`, `getTenant`, `checkMemberships`), learning (attempts, question-plans) và interface platform ops. Gọi **trong tiến trình**: path `/internal/v1/...` chỉ là định danh operation (dùng lại khi tách module thành service), không có HTTP nội bộ, `servers` hay security scheme. Mỗi operation có `x-allowed-callers` (tên module) và `x-context-required` khi cần `tenant_scope`/`idempotency_key` | **Có** — sửa tay (trừ khối platform ops) | Go: `make gen` sinh kiểu DTO + interface strict-server vào `modules/<module>/api/`; module đích hiện thực, module gọi dùng qua adapter + `pkg/modcall`; `tools/allowedcallers` sinh luật depguard từ `x-allowed-callers` |
| `openapi/modules/common/internal-common.v1.yaml` | Thư viện YAML (không phải tài liệu độc lập) | Interface **platform ops** chuẩn (tenant clone steps, trash, students cleanup/restore, users merge-into/anonymize) + component dùng chung (Problem, tham số path/phân trang, response lỗi). Không có tham số header ngữ cảnh: trace, `TenantScope`, Principal gốc, `Idempotency-Key` đi qua `context.Context` (`pkg/modcall`) | **Có** | Khối platform ops được chép inline vào từng `*.internal.v1.yaml` bởi `scripts/openapi/sync-platform-ops.mjs` |
| `sis/eduworld-sis-contract.v1.yaml` | OpenAPI 3.1 (có `webhooks`) | **EduWorld SIS Contract v1**: API + DTO do EduWorld sở hữu mà **SIS của trường triển khai** (pull), cùng payload webhook `ChangeBatch` SIS đẩy tới EduWorld (ADR-0021). Version 1.2.0: chỉ trường, năm học, lớp, học sinh, enrollment — không có nhân sự/phân công giảng dạy (ADR-0072). Không thuộc `/api/v1` | Có | Go: `oapi-codegen` client trong module `identity` · Mock server + conformance test kit giao cho trường |
| `asyncapi/eduworld-realtime.v1.yaml` | AsyncAPI 3.0 | Giao thức WebSocket `/realtime/v1` qua `realtime` (115 message, 22 operation; `AnswerResponse` oneOf theo loại câu như `QuestionResponse`; gồm boss/challenge/progression của module game; message type `<domain>.<entity>.<action>`, bảng domain → module) | Có | Go message types · GDScript message constants |
| `events/` | JSON Schema 2020-12 | Envelope domain event (outbox → NATS JetStream, ADR-0042), payload schema theo event (`payloads/identity/*`, `payloads/learning/*`) + danh mục stream/subject ([events/README](events/README.md)) | Có | Contract test producer/consumer |
| `portal/menu-catalog.yaml` | YAML theo `schemas/portal-menu-catalog.schema.json` | **Danh mục workspace → menu → action của Admin Portal** (ADR-0090): khóa, `path`, `label_key`, `required_permissions` (any-of), `operation_ids`, `can_disable` (5 workspace, 78 menu, 221 action; 10 menu và 30 action không tắt được) | Có | Sinh `menuRegistry`/`workspaceRegistry` của `web/apps/portal` (`make portal-catalog`), bảng đọc-chỉ `platform.portal_menu_catalog` (ghi bởi `cmd/migrate`), bảng route → action của middleware `featuregate` (`make gen`); kiểm tra hai chiều với `x-portal-action` bằng `scripts/specs/check-portal-catalog.mjs` |
| `schemas/*.schema.json` | JSON Schema 2020-12 | Nội dung data-driven và cấu hình (bảng dưới) | Có | Validate khi CMS lưu/publish và khi import |
| `examples/*.json` | — | Ví dụ hợp lệ theo schema, dùng làm test fixture và seed | — | CI validate |
| `reference-data/*.json` | JSON Schema `schemas/reference/*` | Danh mục platform (ADR-0060): quốc gia, 34 tỉnh/thành Việt Nam đang hiệu lực + 63 đơn vị cũ (inactive) + bảng kế thừa cũ → mới. `official_code` = null, chờ đối chiếu văn bản chính thức trước khi seed production | Seed `eduworld_identity` | `node scripts/specs/validate-reference-data.mjs` (CI) |
| `.spectral.yaml`, `spectral-functions/` | Spectral ruleset | `spectral:oas` + rule `string-must-have-maxlength` (ADR-0041) + `operation-must-declare-service` (spec public) + `internal-operation-must-declare-callers`, `internal-context-required-values`, `internal-no-transport-security` (spec internal, ADR-0074) | — | CI lint OpenAPI |
| `_deferred/` | — | Schema/example **không thuộc version này** (ADR-0082). Không sinh code, không lint, không triển khai; script trong `scripts/specs/`, `scripts/openapi/` không quét | Không | — ([_deferred/README](_deferred/README.md)) |
| `postman/` *(kế hoạch Phase 1)* | Postman Collection | Collection API test sinh từ OpenAPI + environment `local`/`dev`/`staging` (không chứa secret; token lấy qua biến môi trường CI) | — | Chạy bằng Postman CLI trong CI ([TEST_STRATEGY](../docs/12-project/TEST_STRATEGY.md)) |

### JSON Schema

| Schema | Nội dung | Tài liệu nguồn | Ví dụ |
|---|---|---|---|
| `common/enums.schema.json` | Enum chuẩn dùng chung: `QuestionType`, `ActivityType`, `FeedbackTiming`, `ResultRelease`, `AttemptMode`, `AttemptStatus`, `LearningContentType` (schema khác `$ref`; OpenAPI/AsyncAPI khai báo inline, `scripts/specs/check-enums.mjs` kiểm tra đồng bộ) | [ACTIVITY_ENGINE](../docs/02-learning/ACTIVITY_ENGINE.md), [CONTENT_MODEL](../docs/02-learning/CONTENT_MODEL.md) | — |
| `question.schema.json` | Question version — 11 dạng, tách `render_payload` / `answer_key` / `scoring_rule`; `options[].feedback` (phía đáp án), `hints[]`, `min_response_ms`, `worked_example_ref`; `$defs.response` = câu trả lời theo từng loại câu | [QUESTION_BANK](../docs/02-learning/QUESTION_BANK.md) | (ví dụ inline trong tài liệu) |
| `question-set.schema.json` | Question set tĩnh hoặc selection rules | [QUESTION_BANK](../docs/02-learning/QUESTION_BANK.md) | — |
| `assessment-blueprint.schema.json` | Ma trận đề | [EXAM_ENGINE](../docs/02-learning/EXAM_ENGINE.md) | — |
| `quest.schema.json` | Quest version **v2** (12 loại quest, steps gồm discover/challenge/boss, repeatability, failure conditions, rewards) | [QUEST_SYSTEM](../docs/01-game-design/QUEST_SYSTEM.md) | `quest.math-g3-multiplication.json` |
| `minigame-config.schema.json` | Cấu hình mini-game (không chứa câu hỏi riêng) | [MINIGAME_ENGINE](../docs/08-game-client/MINIGAME_ENGINE.md) | `minigame.math-g3-matching.json` |
| `npc-dialogue.schema.json` | Đồ thị hội thoại NPC | [QUEST_SYSTEM](../docs/01-game-design/QUEST_SYSTEM.md) | — |
| `reward-rule.schema.json` | Reward rule + cap + diminishing returns | [GAME_DESIGN](../docs/01-game-design/GAME_DESIGN.md), [ANTI_CHEAT](../docs/10-security/ANTI_CHEAT.md) | — |
| `progression-curve.schema.json` | Đường cong XP, danh hiệu, phần thưởng level | [PROGRESSION_SYSTEM](../docs/01-game-design/PROGRESSION_SYSTEM.md) | `progression-curve.default.json` |
| `challenge.schema.json` | Challenge PvE (11 loại, 5 mức độ khó) | [CHALLENGE_BOSS_SYSTEM](../docs/01-game-design/CHALLENGE_BOSS_SYSTEM.md) | `challenge.math-g3-time-attack.json` |
| `boss.schema.json` | Boss nhiều phase, đóng góp nhóm | [CHALLENGE_BOSS_SYSTEM](../docs/01-game-design/CHALLENGE_BOSS_SYSTEM.md) | `boss.math-fraction-dragon.json` |
| `achievement.schema.json` | Thành tựu: trigger, tiến độ, tier, ẩn | [INVENTORY_ECONOMY](../docs/01-game-design/INVENTORY_ECONOMY.md) | `achievement.explorer-math-zone.json` |
| `shop-item.schema.json` | Mặt hàng Shop (cosmetic, giá Xu) | [INVENTORY_ECONOMY](../docs/01-game-design/INVENTORY_ECONOMY.md) | `shop-item.explorer-hat.json` |
| `chat-policy.schema.json` | Chat policy theo cấp platform/tenant/school/class (ADR-0020), v5: `moderation_switches` + `sensitive_term_policy` (ADR-0030), ràng buộc always-on gameplay (ADR-0020 §6), `features` + sticker/gif (ADR-0034) | [CHAT_SYSTEM](../docs/03-social-multiplayer/CHAT_SYSTEM.md), [MODERATION](../docs/03-social-multiplayer/MODERATION.md) | `chat-policy.platform-default.json` |
| `sensitive-term-catalog.schema.json` | Định dạng export/import danh mục từ nhạy cảm (category, term, allowlist) theo platform/tenant (ADR-0030). Không commit danh mục thật | [MODERATION §11](../docs/03-social-multiplayer/MODERATION.md) | `sensitive-term-catalog.placeholder.json` (placeholder) |
| `activities/activity-base.schema.json` | Trường chung của Learning Activity (ADR-0048), `mathDisplay` (ADR-0055); `activity_type` chỉ gồm loại trong version này, chiến lược chấm chỉ tự động | [ACTIVITY_ENGINE](../docs/02-learning/ACTIVITY_ENGINE.md) | — |
| `activities/{quiz,interactive-exercise,minigame,video-lesson,audio-lesson,simulation}.schema.json` | Cấu hình riêng từng `activity_type` trong Release | [ACTIVITY_ENGINE](../docs/02-learning/ACTIVITY_ENGINE.md), [MINIGAME_ENGINE](../docs/08-game-client/MINIGAME_ENGINE.md), [VIDEO_LEARNING](../docs/02-learning/VIDEO_LEARNING.md), [AUDIO_LANGUAGE_LEARNING](../docs/02-learning/AUDIO_LANGUAGE_LEARNING.md), [SIMULATION_ENGINE](../docs/02-learning/SIMULATION_ENGINE.md) | ví dụ video lớp 4 |
| `learning-event.schema.json` | Learning event envelope + payload theo `event_type` (`producer` = `<module>/<thành phần>`, ADR-0074); `actor.type` = student/system/staff, `context.mode` = AttemptMode; `question.answered` có `hints_used`, `response_ms`, `rapid_guess`; `activity.submitted/graded`, `exam.graded`, `attempt.re_evaluated`, `learning_objective.mastered` | [EVENT_DESIGN](../docs/07-backend/EVENT_DESIGN.md) | (ví dụ inline trong schema) |
| `portal-menu-catalog.schema.json` | Cấu trúc danh mục menu/action Admin Portal (khóa, `label_key`, permission, `operation_ids`, `can_disable`) — ADR-0090 | [ADMIN_PORTAL §2.7](../docs/04-admin-portal/ADMIN_PORTAL.md) | `portal/menu-catalog.yaml` |
| `../events/envelope.schema.json` | Envelope domain event | [EVENT_DESIGN §2](../docs/07-backend/EVENT_DESIGN.md#2-envelope-chuẩn) | — |

## Quy tắc

1. Không breaking change trong `v1`. Breaking change phải có version mới song song (ADR-0010). CI chạy `oasdiff breaking` cho spec gộp và từng spec internal.
2. Schema nội dung có trường `schema_version`. Backend hỗ trợ đọc mọi version còn được tham chiếu bởi content đã publish.
3. Render payload **không bao giờ** chứa `answer_key`, `scoring_rule`, `accepted_answers`, `correct_option_id(s)`, `explanation`, `feedback`/`option_feedback`, `hints`, `misconception_tag` (ADR-0016, R6) — kể cả internal API (`question-plans` chỉ trả render payload); OpenAPI chặn bằng `not: anyOf: required`.
4. Mọi example trong `examples/` phải validate được bằng schema tương ứng (CI).
5. JSON field `snake_case`, ID là UUIDv7 (string), thời gian ISO-8601 UTC.
6. Mã lỗi (`ProblemCode` trong OpenAPI) và catalog trong [API_DESIGN §4.1](../docs/07-backend/API_DESIGN.md#41-catalog-mã-lỗi) phải sửa trong cùng merge request.
7. **ADR-0041:** mọi `type: string` có `maxLength` (trừ `enum`, `const`, `format` uuid/date/date-time/time/duration/binary/byte). Bảng độ dài chuẩn trong ADR-0041 và [API_DESIGN §13](../docs/07-backend/API_DESIGN.md#13-giới-hạn-độ-dài-chuỗi-và-validation-adr-0041).
8. **ADR-0074:** mọi operation trong spec gộp có `x-service` (tên module); sau khi sửa spec gộp phải chạy `split-public.mjs` và commit file sinh ra. Mọi operation internal có `x-allowed-callers` là danh sách tên module (`identity`, `learning`, `game`, `competition`, `chat`; không dùng tên client `svc-*`), khai báo `x-context-required` khi bắt buộc `tenant_scope`/`idempotency_key`, không có `servers`/`security`; sau khi sửa phải chạy `make gen` và `make arch-check`.
9. JSON Schema phải compile với **Ajv full strict** (`strict: true`) cùng `allowMatchingProperties: true` (cho phép khai báo `properties` trùng `patternProperties`, dùng ở `i18nText`). Hiện **mọi** schema đã đạt.

## Kiểm tra

Đã chạy ngày 2026-11-15:

| Kiểm tra | Công cụ | Kết quả |
|---|---|---|
| OpenAPI lint — 12 file (gộp, 5 public theo module, 5 internal, SIS contract) | `@redocly/cli@1.25.0 lint` (recommended) | Chưa chạy lại sau đợt ADR-0072 (CLI không cài cục bộ); đã kiểm bằng js-yaml: parse OK, mọi `$ref` nội bộ resolve, 0 operationId trùng, path param khai báo đủ, 0 component thừa |
| OpenAPI ruleset | `@stoplight/spectral-cli@6.11.1 lint -r specs/.spectral.yaml` (12 file) | 2026-11-17, 5 file internal: rule ADR-0074 đạt; còn 1 lỗi `array-items` có sẵn ở `learning.internal.v1.yaml` (`QuestionResponse.oneOf[7].pairs`, tuple lồng). Spec public chưa chạy lại; `x-service` có ở mọi operation (`split-public.mjs` kiểm tra) |
| AsyncAPI | `@asyncapi/parser@3.3.0` | Chưa chạy lại; YAML parse + `$ref` resolve OK |
| JSON Schema + examples | `scripts/specs/validate-schemas.mjs` (`ajv@8.17.1` draft 2020-12, `strict: true`, `allowMatchingProperties`, `ajv-formats`) | 37/37 schema compile (gồm `common/*`, `activities/*`, `reference/*`, `events/envelope`, `events/payloads/{identity,learning}/*`), 12/12 example hợp lệ; không quét `_deferred/` |
| maxLength | `scripts/specs/check-maxlength.mjs` | 54 file, 2.673 schema string, 0 vi phạm (bỏ qua `examples/`, `_deferred/`, `features/`) |
| Danh mục menu/action portal | `scripts/specs/check-portal-catalog.mjs` (schema; khóa duy nhất, cha–con, danh sách không tắt được của ADR-0090 mục 2; permission có trong ADMIN_PORTAL §2.2; hai chiều `operation_ids` ↔ `x-portal-action`; mọi route portal có `x-portal-action` hoặc `x-portal-shell`) | 5 workspace, 78 menu, 221 action, 258 operation có `x-portal-action`, 13 `x-portal-shell`, 0 lỗi (2026-11-17) |
| Catalog mã lỗi | `scripts/specs/check-problem-codes.mjs` (API_DESIGN §4.1 ↔ `ProblemCode` ↔ AsyncAPI `ErrorCode`) | 153 = 153 mã, `ErrorCode` 38 (7 chỉ-WS), 0 lỗi (2026-11-16, thêm `REFRESH_TOKEN_*` — ADR-0086) |
| Đồng bộ enum học tập | `scripts/specs/check-enums.mjs` (`schemas/common/enums.schema.json` ↔ OpenAPI gộp + internal ↔ AsyncAPI; quét enum mọi spec không chứa giá trị deferred) | 7 enum nguồn, 13 khai báo đã so sánh, 51 file không chứa giá trị deferred (`written_response`, `speaking`, `project`, `rubric`…, ADR-0082), 0 lỗi |
| Reference data | `scripts/specs/validate-reference-data.mjs` | 4 quốc gia, 97 tỉnh/thành (34 VN active), 63 kế thừa, 0 lỗi |
| Event và bảng giữa feature | `scripts/specs/check-cross-features.mjs` (`--verbose` in cảnh báo, `--list` in bảng module:event → feature sở hữu). Đọc catalog [EVENT_DESIGN §4](../docs/07-backend/EVENT_DESIGN.md#4-catalog-domain-events) (+ tên ở §3.2), mục "Event / outbox" của `NNN-*/data-model.md`, mục "Event (NATS)" của `NNN-*/contracts/README.md`, `events/payloads/**`. **Lỗi**: event có 0 hoặc > 1 feature sở hữu theo module phát; feature sở hữu khác module producer; event dùng trong feature mà không có trong catalog; event tiêu thụ/phát lại không có owner; "owner F<NNN>" khai sai; file/đường dẫn schema mang tên không phải event của catalog hoặc sai thư mục module; feature `Done` thiếu file schema của mình; bảng ghi "Mới" ở data-model của hơn một feature. **Cảnh báo**: owner chưa tạo file schema (feature chưa Done) | 2026-11-17: 93 feature, 197 event, 209 cặp module:event có owner, 217 dòng tiêu thụ, 89 bảng "Mới", 0 lỗi, 197 cảnh báo (schema payload chưa tạo) |
| Đồng bộ file sinh | `split-public.mjs --check`, `sync-platform-ops.mjs --check` | Không lệch |

Lệnh trong CI (tooling pin phiên bản trong `scripts/package.json`; công cụ CLI pin trong job CI):

```bash
npm ci --prefix scripts
node scripts/openapi/split-public.mjs --check
node scripts/openapi/sync-platform-ops.mjs --check
node scripts/specs/check-maxlength.mjs
node scripts/specs/check-problem-codes.mjs
node scripts/specs/check-portal-catalog.mjs
node scripts/specs/check-enums.mjs
node scripts/specs/validate-reference-data.mjs
node scripts/specs/check-cross-features.mjs
npx @redocly/cli@1.25.0 lint specs/openapi/eduworld-api.v1.yaml specs/openapi/modules/*.v1.yaml specs/sis/eduworld-sis-contract.v1.yaml
npx @stoplight/spectral-cli@6.11.1 lint -r specs/.spectral.yaml specs/openapi/eduworld-api.v1.yaml "specs/openapi/modules/*.v1.yaml" specs/sis/eduworld-sis-contract.v1.yaml
npx @asyncapi/cli validate specs/asyncapi/eduworld-realtime.v1.yaml
node scripts/specs/validate-schemas.mjs
npx oasdiff breaking <base>/specs/openapi/eduworld-api.v1.yaml specs/openapi/eduworld-api.v1.yaml
```

File thư viện `openapi/modules/common/internal-common.v1.yaml` không lint độc lập; nó được kiểm tra gián tiếp qua các file `*.internal.v1.yaml` tham chiếu nó.
