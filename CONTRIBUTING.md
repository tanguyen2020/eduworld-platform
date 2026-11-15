# Contributing to EduWorld

Hướng dẫn đóng góp cho mọi thành viên (người và AI coding agent). Đọc trước: [README](README.md), [AGENTS](AGENTS.md), [docs/README](docs/README.md), [README](docs/adr/README.md).

## 1. Quy trình làm một thay đổi

Theo Master Prompt §23 Step 6:

1. Đọc requirement (FR/US ID) và tài liệu liên quan.
2. Thiết kế giải pháp; nếu thay đổi kiến trúc → viết ADR mới **trước**.
3. Xác định service sở hữu ([AGENTS §6](AGENTS.md#6-domain--tài-liệu--service)). Thay đổi chạm nhiều service: tách MR theo service, thứ tự service nhận (endpoint internal/event mới, tương thích ngược) → service gọi.
4. Cập nhật **specs trước code**: `specs/openapi/services/<service>.v1.yaml` (public), `<service>.internal.v1.yaml` (internal), `specs/asyncapi`, `specs/schemas`, `specs/events` → lint pass (gồm rule `maxLength`) → `make gen SVC=<service>`.
5. Implement backend theo tầng (ent schema → Atlas migration → domain → application command/query → infrastructure → presentation) → Godot client → CMS/LMS (nếu liên quan).
6. Viết test (unit theo tầng, integration, contract test với service liên quan, API) và chạy.
7. Cập nhật tài liệu và `CHANGELOG.md`.
8. Mở Merge Request với checklist §5.

## 2. Branching

- Nhánh chính: `main` (luôn deploy được lên dev). Được bảo vệ, chỉ merge qua MR.
- Nhánh làm việc (ngắn hạn, ≤ 3 ngày nếu có thể):
  - `feat/<domain>-<mô-tả-ngắn>` — vd. `feat/party-invitation`
  - `fix/<domain>-<mô-tả>`
  - `docs/<chủ-đề>`
  - `chore/<mô-tả>`, `refactor/<domain>-<mô-tả>`
  - `adr/<NNNN>-<chủ-đề>`
- Release **theo service**: tag `<service>/vMAJOR.MINOR.PATCH` từ `main` (ví dụ `chat-service/v1.2.0`); web và Godot có tag riêng (`web/v…`, `godot/v…`). Hotfix: `hotfix/<service>-<mô-tả>` từ tag, merge lại `main`.
- Rebase lên `main` trước khi merge; squash commit nếu lịch sử nhánh lộn xộn.

## 3. Commit convention

[Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <mô tả ngắn, mệnh lệnh, ≤ 72 ký tự>

<thân: vì sao thay đổi, ảnh hưởng>

Refs: FR-PTY-003, ADR-0011
```

- `type`: `feat`, `fix`, `docs`, `test`, `refactor`, `perf`, `chore`, `ci`, `build`, `security`.
- `scope`: tên service (`identity`, `learning`, `game`, `competition`, `chat`, `realtime`), `pkg`, hoặc mã domain trong [docs/README §3.3](docs/README.md#34-mã-định-danh) viết thường (`party`, `qb`...), hoặc `godot`, `web`, `specs`, `deploy`.
- Thay đổi breaking: thêm `!` (`feat(api)!: ...`) và mục `BREAKING CHANGE:`.
- Không đưa token, mật khẩu, dữ liệu học sinh vào commit message.

## 4. Code style

### 4.1. Go
- `gofmt`/`goimports`; `golangci-lint` cấu hình trong repo phải pass (gồm `govet`, `staticcheck`, `errcheck`, `gosec`, `depguard`).
- **Cấu trúc service (ADR-0036, ADR-0038):** `services/<service>/{cmd/{api,worker}, internal/{domain, application/{command,query}, infrastructure, presentation/{http,consumer,job}}, ent/schema, migrations, api}`. Phụ thuộc chỉ hướng vào trong; không import code service khác; chỉ dùng chung `pkg/`. Linter `go-arch-lint` kiểm tra.
- **CQRS:** mỗi use case là một command hoặc query + handler đăng ký vào mediator `pkg/mediator`; command qua aggregate + outbox trong một transaction; query không ghi.
- `context.Context` là tham số đầu tiên của mọi hàm I/O; không lưu context trong struct.
- Lỗi: wrap bằng `fmt.Errorf("...: %w", err)`; lỗi domain là kiểu/sentinel có `code` ổn định để ánh xạ Problem Details.
- Log bằng `log/slog`, có `trace_id`; không log PII/token ([MONITORING §8](docs/11-operations/MONITORING.md#8-logging)).
- Truy cập DB bằng **ent** (ADR-0039): schema dùng mixin chung (`id`, `tenant_id`, audit, `deleted_at`, `version`); mọi field string có `MaxLen(n)` (ADR-0041); global filter tenant + soft-delete luôn bật (bỏ chỉ qua `pkg/tenancy.Bypass` có audit). SQL tường minh cho Query side đặt trong `infrastructure`, có tham số và điều kiện tenant + soft-delete. Migration chỉ sinh bằng `make migrate-diff` (Atlas), review SQL trong MR.
- Gọi service khác **chỉ** qua typed client sinh từ spec internal bằng `make gen`, bọc bởi `pkg/svcclient` (timeout, retry idempotent, circuit breaker, token `svc-*`) (ADR-0040).
- Không `panic` trong luồng request; recover middleware chỉ là lưới an toàn.
- Test: table-driven, tên `Test<Unit>_<Scenario>`; integration test dùng build tag `integration`.

### 4.2. GDScript (Godot 4.x)
- Static typing bắt buộc (ADR-0014); bật warning `untyped_declaration` = error.
- `snake_case` cho biến/hàm/file, `PascalCase` cho `class_name`/node, hằng `UPPER_SNAKE_CASE`.
- Business logic không nằm trong scene script nếu tách được — đặt vào class `RefCounted` trong `scripts/` để test bằng GdUnit4.
- Giao tiếp giữa scene bằng signal/event bus; không `get_node("../../..")` xuyên scene.
- Không hardcode nội dung học tập, URL môi trường, client secret.
- Lint bằng `gdlint`/`gdformat` (gdtoolkit).
- UI dùng Theme sinh từ `design/game-tokens/`; không override màu/font/cỡ chữ trong scene (lint `tools/godot/lint_theme_overrides.gd`). Không sửa tay `eduworld_theme.tres`, `ui_tokens.gd`, `resources/art/generated/`.

### 4.3. TypeScript (web)
- `strict: true`; ESLint + Prettier; không `any` trừ khi có comment lý do.
- API client sinh từ OpenAPI — không viết tay type trùng lặp.
- Ẩn/hiện UI theo quyền chỉ là trải nghiệm; quyền thật do backend quyết định.
- Giao diện theo Admin/Ops UI (ADR-0047): dùng component Ant Design / Ant Design Pro và `web/packages/ops-ui`; tùy biến chỉ qua theme token; không màu/font hardcode (stylelint + ESLint chặn); component mới có story trong Storybook.

### 4.3b. Thay đổi design token (`design/`)
1. Sửa JSON token trong `design/game-tokens/`, `design/admin-tokens/` hoặc `design/shared-tokens/` (không sửa file sinh ra).
2. Chạy `make design-tokens`; commit cả nguồn và file sinh trong cùng MR.
3. Đính kèm ảnh trước/sau (Storybook cho CMS, `UiGallery.tscn` cho game) và báo cáo tương phản.
4. Reviewer: Game Director cho game-tokens/art3d, UX Lead cho admin-tokens, cả hai cho shared-tokens.
5. Đổi tên/xóa token: đánh dấu `deprecated` ít nhất một phiên bản, cập nhật mọi nơi dùng; ghi `design/CHANGELOG.md`.

### 4.4. Specs & tài liệu
- OpenAPI 3.1 / AsyncAPI 3.0; field `snake_case`; mọi endpoint có `security`, response lỗi Problem Details, ví dụ; **mọi `type: string` có `maxLength`** (trừ uuid/date/date-time/enum/pattern giới hạn).
- **Spec internal** (`specs/openapi/services/<service>.internal.v1.yaml`): path `/internal/v1/...`, mỗi operation có `x-allowed-callers` (danh sách `svc-*`). Đổi spec internal = đổi contract: chạy `make specs-breaking` và `make test-contract`; breaking change phải mở `/internal/v2` song song đến khi mọi bên gọi chuyển xong.
- Event mới: thêm schema vào `specs/events/` với `schema_version`; consumer đọc được N và N-1.
- Tài liệu theo quy ước [docs/README §3](docs/README.md#3-quy-ước-tài-liệu): header, nhãn [Confirmed]/[Proposed]/[TBD], mã ID.

### 4.5. Quy trình tài liệu

- **Một nguồn sở hữu:** mỗi quy tắc/tham số/sơ đồ chỉ viết ở một tài liệu ([docs/README §3.3](docs/README.md)); nơi khác link. Tham số mặc định đăng ký ở [CONFIG_DEFAULTS](docs/00-overview/CONFIG_DEFAULTS.md); đổi giá trị = sửa tài liệu sở hữu + CONFIG_DEFAULTS trong cùng MR.
- **Tài liệu mới:** đặt đúng thư mục theo [bản đồ tài liệu](docs/README.md) (game design ở `01-game-design/`, học tập ở `02-learning/`…), thêm vào docs/README và mục "Liên quan" của tài liệu gần nhất.
- **Di chuyển/đổi tên:** `node scripts/docs/move-docs.mjs <map.json>` (map `{ "docs/cũ.md": "docs/mới.md" }`, tự sửa mọi link). Không đổi tên bằng tay.
- **Gộp tài liệu:** đưa nội dung vào file đích, thay file nguồn bằng đúng một dòng `<!-- MERGED-INTO: docs/<đích>.md -->`, chạy `node scripts/docs/redirect-merged.mjs` (xóa file nguồn, chuyển link và anchor còn tồn tại).
- **Kiểm tra:** `make docs-check` (= `node scripts/docs/check-mermaid.mjs` + `node scripts/docs/check-links.mjs`, gồm anchor, + `node scripts/docs/build-rtm.mjs --check`) trước khi mở MR; CI chặn merge nếu lỗi.
- **Ma trận truy vết:** thêm/sửa FR, AC, US, TC hoặc operation OpenAPI → chạy `node scripts/docs/build-rtm.mjs` để sinh lại [TRACEABILITY_MATRIX](docs/12-project/TRACEABILITY_MATRIX.md) (không sửa tay). Ghi mã FR/AC trong bảng TC để có liên kết tường minh.

## 5. Merge Request checklist

```markdown
## Mô tả
- Liên kết: FR/US/ADR/OQ:
- Thay đổi chính:

## Checklist
- [ ] Specs (public/internal/event) cập nhật trước code, lint pass (Spectral + `maxLength`, AsyncAPI, ajv); không breaking change ngoài ý muốn (oasdiff); `make gen-check` sạch
- [ ] Đúng tầng Clean Architecture, đúng CQRS (command/query tách), không import chéo service, không truy cập DB service khác
- [ ] Field string mới có giới hạn độ dài ở ent schema, migration (`varchar(n)`) và spec
- [ ] Gọi service khác qua typed client + resilience; contract test với service liên quan pass
- [ ] Test mới/cập nhật: unit / integration / API (Postman CLI) — kết quả đính kèm
- [ ] AuthN + AuthZ + tenant scope có test từ chối người ngoài phạm vi
- [ ] Thao tác quan trọng có Idempotency-Key và test nộp trùng
- [ ] Không tin điểm/kết quả/reward từ client
- [ ] Migration Atlas theo expand → contract, `atlas migrate lint` pass, có `lock_timeout`, không xóa migration cũ
- [ ] Log/metric/trace bổ sung; không log PII/token
- [ ] Tài liệu + CHANGELOG cập nhật; ADR mới nếu đổi kiến trúc
- [ ] Không có secret (gitleaks pass)
- [ ] Ảnh/video demo cho thay đổi UI (Godot/web)
```

Số approval: 1 cho thay đổi thường; **2** cho `identity-service`, `pkg/` (ảnh hưởng mọi service), `pkg/authz`, chat moderation, `migrations/`, spec internal, `deploy/`, ADR.

## 6. Review checklist bảo mật & tenant (bắt buộc với reviewer)

| # | Câu hỏi | Ghi chú |
|---|---|---|
| 1 | Endpoint/message mới có yêu cầu xác thực? Có trong route APISIX đúng nhóm? | ADR-0009 |
| 2 | Use case kiểm tra quyền theo **phạm vi** (school/class/subject), không chỉ role? | ADR-0005 |
| 3 | Mọi truy vấn tenant-scoped có `tenant_id` từ `TenantScope` của Principal (global filter ent còn bật), không từ input client? Endpoint internal có kiểm tra `X-Tenant-Id` khớp tài nguyên? | ADR-0005, ADR-0039 |
| 4 | Trả 404 thay vì 403 khi tài nguyên thuộc tenant khác (không lộ tồn tại)? | |
| 5 | Có khả năng IDOR qua ID trong path/body/WS message? | |
| 6 | Dữ liệu trả ra có field nhạy cảm (tên thật, ngày sinh, mã HS, answer key) cho người không đủ quyền? | DOMAIN_MODEL §2, ADR-0016 |
| 7 | Kết quả/điểm/XP/rating được tính phía server? Có chống replay/duplicate? | ADR-0016, ANTI_CHEAT |
| 8 | Input được validate (kích thước, kiểu, enum); WS message có giới hạn kích thước và rate? | |
| 9 | Có hành động cần audit log (quản trị, moderation, truy cập dữ liệu học sinh, thay đổi quyền)? | |
| 10 | Tính năng social có tôn trọng ChatPolicy, block, pipeline kiểm duyệt? Có thể bị dùng để liên lạc ngoài kiểm soát? | ADR-0020, ADR-0030, CHILD_SAFETY |
| 13 | Endpoint internal mới có `x-allowed-callers` tối thiểu, xác thực token `svc-*`, không nhận token người dùng? | ADR-0040 |
| 14 | Event/projection mới chỉ chứa dữ liệu cần thiết (data minimization) và consumer idempotent? | ADR-0037, ADR-0042 |
| 11 | Có secret/credential/URL nội bộ trong code, config, log, test fixture? | |
| 12 | Dữ liệu mới thu thập có cần thiết (data minimization) và có retention? | DATA_PRIVACY |

## 7. ADR process

1. Tạo nhánh `adr/<NNNN>-<chủ-đề>`; tạo file `docs/adr/ADR-NNNN-<slug>.md` từ [TEMPLATE](docs/adr/TEMPLATE.md) với trạng thái `Proposed`, thêm dòng vào bảng chỉ mục và mục "Quyết định hiện hành theo chủ đề" của [docs/adr/README.md](docs/adr/README.md).
2. Review bởi Solution Architect + owner domain bị ảnh hưởng (≥ 2 approval).
3. Khi chấp nhận: đổi trạng thái `Accepted (YYYY-MM-DD)`. ADR Accepted không sửa nội dung quyết định — muốn đổi thì viết ADR mới và đánh dấu ADR cũ `Superseded by ADR-xxxx` (khối cảnh báo ở đầu + dòng trong "Lịch sử thay đổi").
4. Cập nhật tài liệu liên quan và CHANGELOG trong cùng MR hoặc MR liền sau.

## 8. Những điều tuyệt đối không làm

- Commit secret, credential, dữ liệu học sinh thật, file `.env`.
- Hardcode nội dung học tập trong scene Godot hoặc code.
- Bỏ qua authorization/tenant isolation "tạm thời".
- Lưu dữ liệu quan trọng chỉ trong Valkey.
- Tuyên bố production-ready với mock implementation hoặc UI giả lập.
- Xóa migration đã chạy trên staging/production.
- Đọc/ghi database của service khác, hoặc gọi service khác bằng HTTP viết tay/token người dùng.
- Thêm cột/field chuỗi không giới hạn độ dài.
