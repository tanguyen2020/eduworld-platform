# Feature Specification: F134 — Công cụ vận hành bổ sung: lệnh `reconcile-<module>`, loại tenant QA khỏi analytics, trang tải bộ cài cho trường

**Feature Branch**: `134-ops-reconcile-qa-installer` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: `src/backend/pkg/` + mọi module có DB (lệnh đối soát trong binary `worker`) · module `identity` (cờ QA) · module `game` (analytics, API bộ cài) · module `learning` (tổng hợp platform) · `src/cms/frontend/apps/portal` (trang tải bộ cài, cờ QA) · CI `deploy/installer` · **Mốc**: M6 · **Phụ thuộc**: F080, F131, F133 (dùng F010 `updateTenant`, F043/F044 tổng hợp học tập platform)
**Input**: Ba công cụ vận hành còn thiếu trước phát hành: (1) lệnh `reconcile-<module>` (lệnh con `worker reconcile --module=<module>`, ADR-0074) đối soát event giữa các module sau khi khôi phục riêng một database module ([BACKUP_RECOVERY §3.5, §8](../../docs/11-operations/BACKUP_RECOVERY.md), TC-SVC-014); (2) đánh dấu tenant QA (dùng cho synthetic monitoring của F131 trên production) và loại dữ liệu của nó khỏi Game Analytics và tổng hợp học tập platform ([TEST_STRATEGY §4](../../docs/12-project/TEST_STRATEGY.md)); (3) trang portal cho School Admin tải gói cài trường Android (F133; không còn Windows — ADR-0101) theo khối của trường ([DEPLOYMENT §9](../../docs/11-operations/DEPLOYMENT.md)). FR nguồn: FR-OPS-002, NFR-DEPL-002, FR-ANL-009, FR-ANL-012, FR-OPS-003, NFR-PERF-019, FR-GAME-009.

## Clarifications

### Session 2026-11-16

- Q: `reconcile-<module>` đọc dữ liệu module khác thế nào khi không được đọc DB chéo (R13)? → A: Mỗi module có DB hiện thực hai operation facade (spec internal) chỉ trả **ID event + metadata** từ bảng hạ tầng `infra.outbox_events`, `infra.processed_events`, `infra.dead_letters` của database mình; lệnh chạy trong binary `worker` (`worker reconcile --module=<module>`, job Swarm một lần) với code của module được khôi phục, gọi facade của các module khác trong tiến trình qua `src/backend/pkg/modcall` (ADR-0074, R29) — không HTTP, không token.
- Q: Thế nào là "lệch không giải thích được" ([BACKUP_RECOVERY §8](../../docs/11-operations/BACKUP_RECOVERY.md))? → A: [Confirmed] Lệch có event time ≤ điểm khôi phục T, hoặc event nguồn đã publish quá thời gian chờ mà bên nhận chưa xử lý và không nằm trong DLQ. Lệch có event time > T (bên nhận đã xử lý event mà nguồn mất sau khôi phục) là **giải thích được** và được liệt kê để bù trừ thủ công.
- Q: Lệnh có tự bù trừ (ví dụ thu hồi XP)? → A: Không. Chỉ đọc và lập danh sách; bù trừ bằng API nghiệp vụ có audit (vd. `createEconomyAdjustment`), không xóa ([BACKUP_RECOVERY §3.5](../../docs/11-operations/BACKUP_RECOVERY.md)).
- Q: Đánh dấu QA ở cấp tenant hay trường/tài khoản? → A: Cấp tenant (CODE_PLAN: "loại tenant QA"); tenant QA là tenant riêng chỉ có tài khoản test (F131). Chỉ System Admin đặt cờ; tenant `common` không được đánh dấu.
- Q: Link tải bộ cài có cần ký/hạn? → A: Không. Bộ cài không chứa dữ liệu tenant hay secret (F133-AC-17), phát qua CDN công khai; API chỉ trả danh sách đã kiểm chữ ký chỉ mục, lọc theo khối của trường.

### Session 2026-11-24 (ADR-0101 — bỏ bản Windows)

- Q: Owner bỏ hẳn bản Windows — "bộ cài trường" trên trang còn gì? → A: [Confirmed] Chỉ gói APK Android kèm gói vùng theo khối (MDM/USB) — [ADR-0101](../../docs/adr/ADR-0101-bo-windows-khoi-nen-tang-client-release-1-0.md) mục 7; `platform` của chỉ mục/response chỉ còn `android`; hướng dẫn `device.cfg` chỉ áp cho thiết bị Android. PC phòng máy chơi bản Web, không cần cài.
- Q: Trang nên thêm link tới bản Web cho PC phòng máy, hay bỏ trang khi chỉ còn APK? → A: [TBD] Câu hỏi mở cho owner. Tạm giữ trang chỉ với gói APK Android; không thêm link Web cho tới khi owner chốt (không mở rộng phạm vi, R34).

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Đối soát liên module sau khôi phục một database (Priority: P1) 🎯 MVP

Sau khi khôi phục `eduworld_learning` về T (RB-01), người trực chạy `worker reconcile --module=learning` (`reconcile-learning`): báo cáo liệt kê event `activity.graded` mà module game đã cộng XP nhưng module learning không còn (sau T) và xác nhận 0 lệch không giải thích được.

**Why this priority**: Bước bắt buộc của RB-01 và diễn tập hằng quý; thiếu lệnh thì không đo được dữ liệu lệch giữa các database (ADR-0037).

**Independent Test**: Trên môi trường test: ghi attempt → khôi phục learning về T → chạy lệnh → báo cáo `lost_after_restore` đúng số event sau T, `unexplained = 0`, exit code 0.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F134-AC-01 | KHI chạy `reconcile-<module>` với `--restore-point` T và cửa sổ `[--from, --to]`, công cụ PHẢI đọc `infra.outbox_events` của chính module trong cửa sổ, gọi facade `listProcessedEventIds` của mọi module có consumer của nó và phân loại mỗi lệch là `lost_after_restore`, `pending_publish`, `pending_delivery`, `dead_lettered` hoặc `unexplained` theo quy tắc ở Clarifications | FR-OPS-002 · NFR-DEPL-002 · [BACKUP_RECOVERY §3.5](../../docs/11-operations/BACKUP_RECOVERY.md) | TC-SVC-014 |
| F134-AC-02 | KHI đối soát, công cụ PHẢI kiểm cả chiều ngược: gọi facade `listOutboxEventIds` của các module nguồn mà module này tiêu thụ và so với `infra.processed_events` của chính nó, để phát hiện event chưa được xử lý lại sau khi reset consumer | [BACKUP_RECOVERY §3.5](../../docs/11-operations/BACKUP_RECOVERY.md) | TC-SVC-014 |
| F134-AC-03 | HỆ THỐNG PHẢI xuất báo cáo JSON theo `reconcile-report.schema.json` (chỉ `event_id`, `event_type`, `consumer`, `tenant_id`, thời điểm, phân loại — không payload, không PII) và bảng tóm tắt; exit code PHẢI là `0` khi `unexplained = 0`, `2` khi > 0, `3` khi có module không đọc được (database module lỗi, báo cáo `incomplete`), `1` khi tham số sai | [BACKUP_RECOVERY §8](../../docs/11-operations/BACKUP_RECOVERY.md) · AGENTS R14 | TC-SVC-014 |
| F134-AC-04 | NẾU cửa sổ dài hơn `outbox_retention`, `--from` ≥ `--to`, hoặc T nằm ngoài cửa sổ, THÌ công cụ PHẢI dừng với exit code `1` và thông báo giới hạn, không gọi module nào | [CONFIG_DEFAULTS](../../docs/00-overview/CONFIG_DEFAULTS.md) · [DATABASE_DESIGN §3.9](../../docs/07-backend/DATABASE_DESIGN.md) | — |
| F134-AC-05 | `listOutboxEventIds` và `listProcessedEventIds` PHẢI chỉ là operation facade (không route HTTP, không qua APISIX); NẾU module không có trong `x-allowed-callers` import facade đó, THÌ `make arch-check` PHẢI fail; facade PHẢI kiểm tham số: `source_module` là enum, `limit` có trần, cursor có `maxLength` (vi phạm → lỗi `VALIDATION_FAILED`) | AGENTS R7, R28, R29 · ADR-0074 | TC-SVC-005, TC-SVC-006 |
| F134-AC-06 | Công cụ PHẢI chỉ đọc: không ghi bảng nghiệp vụ, không xóa, không bù trừ tự động; lần chạy PHẢI ghi một audit `ops.reconcile.completed` (module, cửa sổ, số lệch theo loại, `run_id`) | AGENTS R10, R21 · FR-OPS-002 | — |

---

### User Story 2 — Loại tenant QA khỏi analytics (Priority: P2)

`sa.admin` đánh dấu tenant QA chứa `hs.qa.probe`; probe synthetic (F131) đăng nhập mỗi 5 phút nhưng DAU, retention, tổng hợp học tập platform không đổi; tenant được gắn nhãn "QA" trong dropdown tenant.

**Why this priority**: Không chặn phát hành nhưng làm sai chỉ số Game Analytics/Live Ops ngay khi probe chạy trên production.

**Independent Test**: Đánh dấu `pgd-demo` là QA (local) → chạy golden dataset có event của `pgd-demo` → rollup và `getGameMetrics` không tính tenant đó; bỏ cờ → event mới được tính.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F134-AC-07 | KHI người có `platform.tenant.manage` gọi `updateTenant` với `is_qa` (`If-Match`, `Idempotency-Key`), module identity PHẢI lưu cờ, ghi audit `tenant.qa_flag_changed` (trước/sau) và phát `tenant.updated` có `is_qa`; NẾU tenant là `common`, THÌ PHẢI trả `422 VALIDATION_FAILED`; role tenant PHẢI không có route/`403` | AGENTS R10, R20 · [TEST_STRATEGY §4](../../docs/12-project/TEST_STRATEGY.md) | TC-CMS-WS-02 |
| F134-AC-08 | KHI analytics consumer hoặc `postTelemetry` nhận sự kiện của tenant có `is_qa = true` (theo projection `ref.tenants`), module game PHẢI không ghi `analytics.events_raw`/`player_days` và PHẢI tăng `eduworld_analytics_events_dropped_total{reason="qa_tenant"}` | FR-ANL-009 · [GAME_ANALYTICS §5](../../docs/11-operations/GAME_ANALYTICS.md) | TC-ANL-001 |
| F134-AC-09 | HỆ THỐNG PHẢI loại tenant QA khỏi rollup, `getGameMetrics`, `getGameRetention`, `getGameFunnel`, `getGameContentStats`, `getGamePerformance`, `createGameAnalyticsExport` và `getPlatformLearningSummary`/`createPlatformReport`, kể cả dữ liệu ghi trước khi đánh dấu và khi `tenant_ids` chỉ định tenant QA (trả rỗng, kèm `excluded_tenant_ids`) | FR-ANL-009 · FR-ANL-012 · [DASHBOARD_DATA §2](../../docs/07-backend/DASHBOARD_DATA.md) | TC-ANL-001 |
| F134-AC-10 | KHI Tenant Admin của tenant QA gọi `getTenantGameSummary`, module game PHẢI trả `200` với số liệu rỗng; NẾU tenant khác gọi, THÌ PHẢI giữ nguyên quy tắc cô lập (`403`/`404`) | FR-ANL-009 · NFR-SEC-006 | TC-ANL-004 |
| F134-AC-11 | Portal PHẢI hiển thị công tắc "Tenant QA" trong form tenant (workspace D, chỉ System Admin) có hộp xác nhận nêu hệ quả, và nhãn "QA" cạnh tên tenant trong `TenantFilter`; không tính toán gì ở client | FR-ANL-009 · AGENTS R1, R32 · [ADMIN_PORTAL §3.14](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-CMS-WS-01 |

---

### User Story 3 — Trang tải bộ cài cho trường (Priority: P2)

`ad.truong.a` mở C → "Tải bộ cài": thấy gói APK Android bản production mới nhất cho khối 1–5 của trường, kích thước, SHA-256 để kiểm, nút tải và hướng dẫn cấu hình `device.cfg` cho tablet Android (máy dùng chung, cache LAN). Không có bộ cài Windows (ADR-0101).

**Why this priority**: [DEPLOYMENT §9](../../docs/11-operations/DEPLOYMENT.md) "tải từ portal (School Admin)"; NFR-PERF-019 phòng máy; F133 để trang này ngoài phạm vi.

**Independent Test**: CI phát chỉ mục bộ cài có chữ ký → `listSchoolClientInstallers` cho TH Demo A trả bản theo khối của trường; chỉ mục sai chữ ký → `503`; `ad.truong.b` hỏi trường A → `404`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F134-AC-12 | KHI phát hành bộ cài trường lên kênh `production`, CI PHẢI sinh chỉ mục `school-kit/<channel>.json` + chữ ký RSA-SHA256 theo `school-kit-index.schema.json` (`kit_id`, `platform` android — ADR-0101 bỏ windows, `client_version`, `grade_bands[]`, `url`, `sha256`, `size_bytes`, `min_os`, `released_at`) và chỉ trỏ tới bộ cài đã ký | NFR-PERF-019 · [DEPLOYMENT §9](../../docs/11-operations/DEPLOYMENT.md) | — |
| F134-AC-13 | KHI người có `school.installer.read` gọi `listSchoolClientInstallers` (`school_id` trong scope), module game PHẢI tải chỉ mục `production`, kiểm chữ ký, lọc bộ cài có `grade_bands` giao với khối của trường (từ lớp đang hoạt động; trường chưa có lớp → mọi bộ cài) và trả bản mới nhất mỗi nền tảng × nhóm khối | NFR-PERF-019 · FR-GAME-009 · [PERFORMANCE_BUDGET §10.1](../../docs/08-game-client/PERFORMANCE_BUDGET.md) | — |
| F134-AC-14 | NẾU chỉ mục không tải được hoặc sai chữ ký/schema, THÌ module game PHẢI trả `503 UNAVAILABLE`, ghi cảnh báo và PHẢI KHÔNG trả bất kỳ link nào chưa kiểm | NFR-SEC-005 · F133-AC-05 | — |
| F134-AC-15 | NẾU School Admin hỏi trường khác (cùng hoặc khác tenant), THÌ module game PHẢI trả `404`; NẾU người gọi thiếu `school.installer.read` (vd. Academic Manager) hoặc là học sinh, THÌ PHẢI trả `403 PERMISSION_DENIED` | AGENTS R7, R8 · NFR-SEC-006 | TC-TEN-003 |
| F134-AC-16 | Response PHẢI chỉ chứa dữ liệu công khai của bộ cài (không dữ liệu tenant, secret, đường dẫn nội bộ); mọi chuỗi có `maxLength` (`url` ≤ 2048, `sha256` pattern hex 64) | AGENTS R28, R31 · F133-AC-17 | TC-TEN-024 |
| F134-AC-17 | Portal PHẢI hiển thị trang "Tải bộ cài" ở workspace C (menu chỉ khi có `school.installer.read`): bảng nền tảng, phiên bản, nhóm khối, kích thước, SHA-256 (nút sao chép), nút tải, hướng dẫn `device.cfg` cho thiết bị Android (`shared_device`, `lan_cache_url`) từ file dịch; lỗi `503` hiển thị thông điệp theo `code`; style từ token | NFR-PERF-019 · AGENTS R1, R32 · [ADMIN_PORTAL §10](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-CMS-WS-01 |
| F134-AC-18 | KHI menu/action của feature đang tắt cho phạm vi phiên (`disabled_menus`/`disabled_actions` của `getMyPermissions`) — khóa: `action.platform_admin.tenants.view`, `action.live_ops.game_analytics.view`, `action.live_ops.game_analytics.export`, `action.school_lms.reports.view`, `action.platform_admin.reports.view`, `action.platform_admin.reports.export` và `menu.school_lms.installers` (`action.school_lms.installers.view`), Portal PHẢI ẩn menu và ẩn/khóa nút tương ứng; NẾU API trả `403 FEATURE_DISABLED`, Portal PHẢI hiện thông báo tính năng tạm tắt và tải lại quyền | FR-CMS-018 · ADR-0090 | TC-CMS-MENU-02 |

### Edge Cases

- NẾU module bên nhận đang lag (consumer chưa bắt kịp), THÌ event nguồn trong `--settle` (mặc định đề xuất 10 phút) gần nhất được xếp `pending_delivery`, không `unexplained` (F134-AC-01).
- NẾU `processed_events` của bên nhận đã quá retention, THÌ cửa sổ bị từ chối (F134-AC-04).
- NẾU System Admin bỏ cờ QA, THÌ dữ liệu raw giữ trước khi đánh dấu lại xuất hiện trong dashboard; dữ liệu trong thời gian đánh dấu không có (không backfill) — ghi rõ trong hộp xác nhận (F134-AC-11).
- NẾU tenant QA cũng là đích của probe synthetic, THÌ học sinh QA vẫn có thể bị job dọn học sinh (F016) — loại trừ bằng cấu hình tenant ở F131, không thuộc F134.
- NẾU chỉ mục có hai bản cùng `platform` × `grade_bands`, THÌ trả bản `released_at` mới nhất (F134-AC-13).
- NẾU người dùng Tenant Admin của tenant `unit` không chọn trường, THÌ Portal yêu cầu chọn trường trước khi gọi API.

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-OPS-002 | Công cụ đối soát thuộc bộ script backup/khôi phục | [BACKUP_RECOVERY §3.5, §9](../../docs/11-operations/BACKUP_RECOVERY.md) |
| NFR-DEPL-002 | Đối soát dữ liệu lệch giữa database riêng của từng module | [BACKUP_RECOVERY §3.5](../../docs/11-operations/BACKUP_RECOVERY.md) |
| FR-ANL-009 | Game Analytics/báo cáo tenant không tính tenant QA | [GAME_ANALYTICS §5, §7](../../docs/11-operations/GAME_ANALYTICS.md) |
| FR-ANL-012 | Tổng hợp học tập platform không tính tenant QA | [DASHBOARD_DATA §2](../../docs/07-backend/DASHBOARD_DATA.md) |
| FR-OPS-003 | Synthetic monitoring (F131) không làm sai chỉ số sản phẩm | [TEST_STRATEGY §4](../../docs/12-project/TEST_STRATEGY.md) |
| NFR-PERF-019 | Phân phối bộ cài trường kèm gói nội dung theo khối | [PERFORMANCE_BUDGET §10.1](../../docs/08-game-client/PERFORMANCE_BUDGET.md) · [DEPLOYMENT §9](../../docs/11-operations/DEPLOYMENT.md) |
| FR-GAME-009 | Trường lấy đúng gói APK cho tablet Android của trường (PC phòng máy chơi bản Web — ADR-0101) | [PERFORMANCE_BUDGET](../../docs/08-game-client/PERFORMANCE_BUDGET.md) |
| FR-CMS-018 | Menu/action của feature theo danh mục `specs/portal/menu-catalog.yaml`, ẩn khi tắt, xử lý `403 FEATURE_DISABLED` | [ADMIN_PORTAL §2.7](../../docs/04-admin-portal/ADMIN_PORTAL.md) |

### Key Entities

- **ReconcileRun**: một lần đối soát (module, điểm khôi phục, cửa sổ, kết quả theo loại lệch, trạng thái hoàn tất) — chỉ là báo cáo, không lưu DB.
- **Mismatch**: một event lệch giữa nguồn và bên nhận (event, consumer, phân loại).
- **Tenant (cờ QA)**: thuộc tính của tenant cho biết dữ liệu không tính vào analytics.
- **SchoolKitIndex**: chỉ mục bộ cài trường có chữ ký (bộ cài theo nền tảng × nhóm khối).

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `outbox_retention` | Trần cửa sổ đối soát |
| `event_stream_retention` | Chọn cách bù (reset consumer hay relay outbox) ghi trong báo cáo |
| `module_call_deadline` | Deadline lời gọi facade đối soát qua `src/backend/pkg/modcall` (không retry/circuit breaker, ADR-0074) |
| `analytics_raw_retention_days` | Dữ liệu raw còn để hiện lại khi bỏ cờ QA |
| `idempotency_record_ttl` | `updateTenant` |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Không đọc DB module khác; chỉ facade trong tiến trình | AGENTS R13, R29 · ADR-0037, ADR-0074 |
| Operation facade đối soát chỉ trả ID, không payload/PII | AGENTS R14 |
| Cờ QA chỉ System Admin; dashboard đọc rollup | AGENTS R20 · ADR-0062 |
| Không câu hỏi đề, media private trên CDN | AGENTS R31 · ADR-0044 |
| UI không logic, style theo token | AGENTS R1, R32 · ADR-0047 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: Diễn tập khôi phục một database: lệnh hoàn tất ≤ 10 phút cho 30 ngày event của một module và báo 0 lệch không giải thích được (TC-SVC-014).
- **SC-002**: Khi probe synthetic chạy 24 giờ trên tenant QA, chênh lệch DAU platform do tenant QA = 0.
- **SC-003**: School Admin tải được bộ cài đúng khối trong ≤ 3 thao tác từ trang chủ workspace C; 100% link trả về đã qua kiểm chữ ký.

## Ngoài phạm vi

- Reset consumer, relay outbox, `projection rebuild`, `saga resume`, `retention-reapply`: F130/F003 và runbook; F134 chỉ đối soát.
- Bù trừ dữ liệu lệch: thao tác thủ công qua API nghiệp vụ (vd. `createEconomyAdjustment`, F075).
- Probe synthetic, seed tenant QA, loại trừ khỏi job dọn học sinh: F131. Build/ký bộ cài, `device.cfg`: F133.
- Loại tenant QA khỏi dashboard nhà trường của chính tenant đó (chỉ có dữ liệu QA): không cần.
- Kênh MDM, Microsoft Store: ngoài Release 1.0 (F133 Clarifications). Bộ cài Windows: không làm (ADR-0101).

## Assumptions

- [Confirmed] Quy tắc phân loại lệch như Clarifications; `--settle` mặc định 10 phút, cần đăng ký CONFIG_DEFAULTS là `reconcile_settle_minutes` (R33).
- [Confirmed] Permission mới `school.installer.read` (tenant được phép; gán `tenant_admin`, `school_admin`; System Admin theo tenant đích) — cập nhật ADMIN_PORTAL §2.2.
- [Confirmed] Cột `platform.tenants.is_qa` và `ref.tenants.is_qa` (game, learning); `updateTenant` do F010 thêm vào contract.
- Ánh xạ khối → `grade_band` theo [GLOSSARY](../../docs/00-overview/GLOSSARY.md) (`g1_3`, `g4_5`, `g6_7`, `g8_9`).
- Chỉ mục bộ cài đặt cạnh manifest phát hành của F133 trên CDN `play.<domain>/releases/`; base URL và khóa công khai qua env (R3).
