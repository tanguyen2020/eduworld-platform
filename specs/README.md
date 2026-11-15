# EduWorld Specs

> Trạng thái: Draft · Phiên bản: 0.3 (microservices) · Owner: Backend Architect · Cập nhật: 2026-11-15
> Liên quan: [API_DESIGN](../docs/07-backend/API_DESIGN.md) · [REALTIME_ARCHITECTURE](../docs/06-architecture/REALTIME_ARCHITECTURE.md) · [EVENT_DESIGN](../docs/07-backend/EVENT_DESIGN.md) · ADR-0010 · ADR-0036 · ADR-0040 · ADR-0041

Đặc tả máy đọc là **nguồn sự thật cho contract** giữa Godot client, CMS/LMS web, các microservice Go và hệ thống ngoài (SIS). Quy trình: sửa spec → review → sinh code → triển khai (contract-first, ADR-0010).

## Cấu trúc

| Đường dẫn | Chuẩn | Nội dung | Nguồn sự thật? | Sinh code / dùng ở đâu |
|---|---|---|---|---|
| `openapi/eduworld-api.v1.yaml` | OpenAPI 3.1 | **Spec public gộp** `/api/v1/*` (312 path, 364 operation, 344 schema, 151 `ProblemCode`; header `X-Tenant-Id` bắt buộc ở 249 operation, tùy chọn ở 62 — ADR-0065). Mọi operation có `x-service` (service sở hữu, ADR-0036); `/game-admin/{config_type}` có thêm `x-service-routing`. `/sis/*`, `/admin/*` gắn `x-internal: true` | **Có** — sửa tay ở đây | Web: `openapi-typescript` · Godot: API layer viết tay theo spec, kiểm tra bằng contract test |
| `openapi/services/<service>.v1.yaml` (`identity`, `learning`, `game`, `competition`, `chat`) | OpenAPI 3.1 | Spec public của từng service, **sinh tự động** từ spec gộp bởi `scripts/openapi/split-public.mjs` (chỉ giữ component được tham chiếu) | Không — **không sửa tay** | Go: `oapi-codegen` strict server cho từng service |
| `openapi/services/<service>.internal.v1.yaml` | OpenAPI 3.1 | **Internal API** `/internal/v1/*` giữa service (ADR-0040): identity (`principals:resolve`, tenants, memberships), learning (attempts, question-plans) và interface platform ops | **Có** — sửa tay (trừ khối platform ops) | Go: server stub bên nhận + **typed client** `oapi-codegen` bên gọi (tương đương Refit) |
| `openapi/services/common/internal-common.v1.yaml` | Thư viện YAML (không phải tài liệu độc lập) | Interface **platform ops** chuẩn (tenant clone steps, trash, students cleanup/restore, users merge-into/anonymize) + component dùng chung (Problem, header `traceparent`/`X-Tenant-Id`/`X-On-Behalf-Of`/`Idempotency-Key`, security `serviceToken`) | **Có** | Khối platform ops được chép inline vào từng `*.internal.v1.yaml` bởi `scripts/openapi/sync-platform-ops.mjs` |
| `sis/eduworld-sis-contract.v1.yaml` | OpenAPI 3.1 (có `webhooks`) | **EduWorld SIS Contract v1**: API + DTO do EduWorld sở hữu mà **SIS của trường triển khai** (pull), cùng payload webhook `ChangeBatch` SIS đẩy tới EduWorld (ADR-0021). Không thuộc `/api/v1` | Có | Go: `oapi-codegen` client trong `identity-service` · Mock server + conformance test kit giao cho trường |
| `asyncapi/eduworld-realtime.v1.yaml` | AsyncAPI 3.0 | Giao thức WebSocket `/realtime/v1` qua `realtime-gateway` (108 message, 21 operation; gồm boss/challenge/progression của game-service; message type `<domain>.<entity>.<action>`, bảng domain → service) | Có | Go message types · GDScript message constants |
| `events/` | JSON Schema 2020-12 | Envelope domain event (outbox → NATS JetStream, ADR-0042) + danh mục stream/subject ([events/README](events/README.md)) | Có | Contract test producer/consumer |
| `schemas/*.schema.json` | JSON Schema 2020-12 | Nội dung data-driven và cấu hình (bảng dưới) | Có | Validate khi CMS lưu/publish và khi import |
| `examples/*.json` | — | Ví dụ hợp lệ theo schema, dùng làm test fixture và seed | — | CI validate |
| `reference-data/*.json` | JSON Schema `schemas/reference/*` | Danh mục platform (ADR-0060): quốc gia, 34 tỉnh/thành Việt Nam đang hiệu lực + 63 đơn vị cũ (inactive) + bảng kế thừa cũ → mới. `official_code` = null, chờ đối chiếu văn bản chính thức trước khi seed production | Seed `eduworld_identity` | `node scripts/specs/validate-reference-data.mjs` (CI) |
| `.spectral.yaml`, `spectral-functions/` | Spectral ruleset | `spectral:oas` + rule `string-must-have-maxlength` (ADR-0041) + `operation-must-declare-service` (spec public) | — | CI lint OpenAPI |
| `postman/` *(kế hoạch Phase 1)* | Postman Collection | Collection API test sinh từ OpenAPI + environment `local`/`dev`/`staging` (không chứa secret; token lấy qua biến môi trường CI) | — | Chạy bằng Postman CLI trong CI ([TEST_STRATEGY](../docs/12-project/TEST_STRATEGY.md)) |

### JSON Schema

| Schema | Nội dung | Tài liệu nguồn | Ví dụ |
|---|---|---|---|
| `question.schema.json` | Question version — 11 dạng, tách `render_payload` / `answer_key` / `scoring_rule` | [QUESTION_BANK](../docs/02-learning/QUESTION_BANK.md) | (ví dụ inline trong tài liệu) |
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
| `activities/activity-base.schema.json` | Trường chung của Learning Activity (ADR-0048), `mathDisplay` (ADR-0055) | [ACTIVITY_ENGINE](../docs/02-learning/ACTIVITY_ENGINE.md) | — |
| `activities/{video-lesson,audio-lesson,speaking,written-response,simulation,project}.schema.json` | Cấu hình riêng từng `activity_type` | [VIDEO_LEARNING](../docs/02-learning/VIDEO_LEARNING.md), [AUDIO_LANGUAGE_LEARNING](../docs/02-learning/AUDIO_LANGUAGE_LEARNING.md), [WRITTEN_RESPONSE_RUBRIC](../docs/02-learning/WRITTEN_RESPONSE_RUBRIC.md), [SIMULATION_ENGINE](../docs/02-learning/SIMULATION_ENGINE.md) | ví dụ video lớp 4, lời giải toán lớp 7 |
| `activities/rubric.schema.json` | Rubric (tiêu chí, mức, điểm) | [WRITTEN_RESPONSE_RUBRIC](../docs/02-learning/WRITTEN_RESPONSE_RUBRIC.md) | rubric văn lớp 7 |
| `learning-event.schema.json` | Learning event envelope + payload theo `event_type` (`producer` = `<service>-service/<module>`) | [EVENT_DESIGN](../docs/07-backend/EVENT_DESIGN.md) | (ví dụ inline trong schema) |
| `../events/envelope.schema.json` | Envelope domain event | [EVENT_DESIGN §2](../docs/07-backend/EVENT_DESIGN.md#2-envelope-chuẩn) | — |

## Quy tắc

1. Không breaking change trong `v1`. Breaking change phải có version mới song song (ADR-0010). CI chạy `oasdiff breaking` cho spec gộp và từng spec internal.
2. Schema nội dung có trường `schema_version`. Backend hỗ trợ đọc mọi version còn được tham chiếu bởi content đã publish.
3. Render payload **không bao giờ** chứa `answer_key` hoặc `scoring_rule` (ADR-0016) — kể cả internal API (`question-plans` chỉ trả render payload).
4. Mọi example trong `examples/` phải validate được bằng schema tương ứng (CI).
5. JSON field `snake_case`, ID là UUIDv7 (string), thời gian ISO-8601 UTC.
6. Mã lỗi (`ProblemCode` trong OpenAPI) và catalog trong [API_DESIGN §4.1](../docs/07-backend/API_DESIGN.md#41-catalog-mã-lỗi) phải sửa trong cùng merge request.
7. **ADR-0041:** mọi `type: string` có `maxLength` (trừ `enum`, `const`, `format` uuid/date/date-time/time/duration/binary/byte). Bảng độ dài chuẩn trong ADR-0041 và [API_DESIGN §13](../docs/07-backend/API_DESIGN.md#13-giới-hạn-độ-dài-chuỗi-và-validation-adr-0041).
8. **ADR-0036:** mọi operation trong spec gộp có `x-service`; sau khi sửa spec gộp phải chạy `split-public.mjs` và commit file sinh ra. Mọi operation internal có `x-allowed-callers`.
9. JSON Schema phải compile với **Ajv full strict** (`strict: true`) cùng `allowMatchingProperties: true` (cho phép khai báo `properties` trùng `patternProperties`, dùng ở `i18nText`). Hiện **mọi** schema đã đạt.

## Kiểm tra

Đã chạy ngày 2026-11-15:

| Kiểm tra | Công cụ | Kết quả |
|---|---|---|
| OpenAPI lint — 12 file (gộp, 5 public theo service, 5 internal, SIS contract) | `@redocly/cli@1.25.0 lint` (recommended) | 12/12 hợp lệ, 0 error, 0 warning |
| OpenAPI ruleset | `@stoplight/spectral-cli@6.11.1 lint -r specs/.spectral.yaml` (12 file) | 0 error, 0 warning |
| AsyncAPI | `@asyncapi/parser@3.3.0` | Hợp lệ, 0 error, 0 warning (1 info) |
| JSON Schema + examples | `scripts/specs/validate-schemas.mjs` (`ajv@8.17.1` draft 2020-12, `strict: true`, `allowMatchingProperties`, `ajv-formats`) | 33/33 schema compile (gồm `activities/*`, `reference/*`, `events/envelope`, `events/payloads/identity/*`), 14/14 example hợp lệ |
| maxLength | `scripts/specs/check-maxlength.mjs` | 50 file, 2.463 schema string, 0 vi phạm |
| Catalog mã lỗi | `scripts/specs/check-problem-codes.mjs` (API_DESIGN §4.1 ↔ `ProblemCode` ↔ AsyncAPI `ErrorCode`) | 151 = 151 mã, `ErrorCode` 38 (7 chỉ-WS), 0 lỗi |
| Reference data | `scripts/specs/validate-reference-data.mjs` | 4 quốc gia, 97 tỉnh/thành (34 VN active), 63 kế thừa, 0 lỗi |
| Đồng bộ file sinh | `split-public.mjs --check`, `sync-platform-ops.mjs --check` | Không lệch |

Lệnh trong CI (tooling pin phiên bản trong `scripts/package.json`; công cụ CLI pin trong job CI):

```bash
npm ci --prefix scripts
node scripts/openapi/split-public.mjs --check
node scripts/openapi/sync-platform-ops.mjs --check
node scripts/specs/check-maxlength.mjs
node scripts/specs/check-problem-codes.mjs
node scripts/specs/validate-reference-data.mjs
npx @redocly/cli@1.25.0 lint specs/openapi/eduworld-api.v1.yaml specs/openapi/services/*.v1.yaml specs/sis/eduworld-sis-contract.v1.yaml
npx @stoplight/spectral-cli@6.11.1 lint -r specs/.spectral.yaml specs/openapi/eduworld-api.v1.yaml "specs/openapi/services/*.v1.yaml" specs/sis/eduworld-sis-contract.v1.yaml
npx @asyncapi/cli validate specs/asyncapi/eduworld-realtime.v1.yaml
node scripts/specs/validate-schemas.mjs
npx oasdiff breaking <base>/specs/openapi/eduworld-api.v1.yaml specs/openapi/eduworld-api.v1.yaml
```

File thư viện `openapi/services/common/internal-common.v1.yaml` không lint độc lập; nó được kiểm tra gián tiếp qua các file `*.internal.v1.yaml` tham chiếu nó.
