# Feature Specification: F108 — Cấu hình thi đấu `/game-admin/*` và UI workspace A

**Feature Branch**: `108-competition-config-portal` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: module `competition` + `src/cms/frontend/apps/portal` · **Mốc**: M4b · **Phụ thuộc**: F079, F107
**Input**: Competition Admin (platform) soạn, duyệt, publish, rollback cấu hình thi đấu có phiên bản (matchmaking, ranking rules, mùa giải, template giải/thử thách, định nghĩa bảng xếp hạng) qua `/game-admin/*` và màn "Thi đấu" của workspace A; School Admin tạo và điều hành thử thách lớp/giải trường từ template trong workspace C. FR nguồn: FR-CMS-005, FR-LMS-004 (màn workspace C).

## Clarifications

### Session 2026-11-16

- Q: Màn workspace C "thử thách lớp/giải trường" thuộc feature nào? → A: F108 (bổ sung theo điều phối; F055 ghi "chưa có feature"). Backend thuộc F107.
- Q: Tạo bản nháp mới từ version đã publish bằng API nào? → A: `createGameConfig_competition` với `config_id` + `based_on_version_id` (delta `GameConfigInput`); `updateGameConfigDraft_competition` trên version `published` trả `409 CONTENT_NOT_EDITABLE`.
- Q: Permission `competition.config.reward.write` dùng ở đâu? → A: Thay bởi Session 2026-11-17 (`competition-rewards`).
- Q: Gán template cho tenant làm ở đâu? → A: Workspace D (F057) qua saga F045; bên nhận ở F107. F108 chỉ hiển thị trạng thái gán (chỉ đọc).

### Session 2026-11-17

- Q: Cấu hình phần thưởng thi đấu sau đối soát (RB-415)? → A: Thêm loại thứ 7 `config_type = competition-rewards` (kebab, [API_DESIGN §9.13](../../docs/07-backend/API_DESIGN.md)) với quyền `competition.config.reward.write`; phần thưởng chỉ cosmetic/badge/title, tham chiếu reward rule của module game. Delta contract ở contracts §6.

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Cấu hình thi đấu phiên bản hóa qua `/game-admin/*` (Priority: P1) 🎯 MVP

Competition Admin tạo bản nháp "Matchmaking v3" nới cửa sổ rating, gửi duyệt; người có quyền publish đặt `effective_from` 00:00 thứ Hai; một tuần sau rollback về v2 kèm lý do.

**Why this priority**: F104–F107 đang chạy trên seed; FR-CMS-005 yêu cầu cấu hình chỉ platform, có version.

**Independent Test**: Tạo draft `ranking-rules`, sửa với `If-Match`, submit → approve → publish (`effective_from` tương lai) → `competition_config.published`; PATCH version đã publish → `409`; token School Admin gọi trực tiếp backend → `403 PERMISSION_DENIED`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F108-AC-01 | module competition PHẢI cung cấp `listGameConfigs_competition`, `createGameConfig_competition`, `getGameConfigVersion_competition`, `updateGameConfigDraft_competition`, `transitionGameConfigVersion_competition`, `listGameConfigVersions_competition`, `diffGameConfigVersions_competition`, `publishGameConfigVersion_competition`, `rollbackGameConfig_competition`, `unpublishGameConfig_competition`, `listGameConfigReleases_competition` cho 7 loại `matchmaking-configs`, `ranking-rules`, `seasons`, `tournament-templates`, `challenge-templates`, `leaderboard-definitions`, `competition-rewards` | FR-CMS-005 · [API_DESIGN §9.13](../../docs/07-backend/API_DESIGN.md) | TC-CMS-001 |
| F108-AC-02 | HỆ THỐNG PHẢI yêu cầu permission theo loại: `competition.config.matchmaking.write`, `competition.config.ranking.write`, `competition.config.season.write`, `competition.config.tournament_template.write` (template giải và thử thách), `competition.config.leaderboard.write`, `competition.config.reward.write` (`competition-rewards`); duyệt cần `game.content.review`, publish/rollback cần `game.content.publish`; thiếu quyền PHẢI nhận `403 PERMISSION_DENIED` và audit `authz.denied` | FR-CMS-005 · [ADMIN_PORTAL §2.2](../../docs/04-admin-portal/ADMIN_PORTAL.md#22-permission-catalog) | TC-CMS-AUTHZ-02 |
| F108-AC-03 | HỆ THỐNG PHẢI chỉ phục vụ `/game-admin/*` qua gateway nội bộ, với `azp = eduworld-portal`, MFA (`acr`) và role platform; gọi qua gateway public PHẢI nhận 404 tại APISIX; token thiếu MFA PHẢI nhận `403 MFA_REQUIRED`; role tenant không bao giờ có `competition.config.*` | AGENTS R20 · ADR-0027 · FR-CMS-005 | TC-CMS-AUTHZ-01, TC-CMS-WS-02, TC-CMS-WS-03 |
| F108-AC-04 | KHI tạo/sửa bản nháp hoặc publish, HỆ THỐNG PHẢI validate `payload` theo JSON Schema của loại và invariant nghiệp vụ: ràng buộc khối không nới được (lớp 1–5 cùng khối, không gộp `g1_3` với `g8_9`), không bật bonus tốc độ cho khối dưới `speed_bonus_min_grade`, ngưỡng tier tăng dần, bảng scope `class`/`school`/`grade` không dùng `xp`, template chỉ có `allowed_params` trong danh sách cho phép và phần thưởng chỉ cosmetic/badge/title; vi phạm PHẢI nhận `422 VALIDATION_FAILED` chỉ rõ trường | FR-CMS-005 · AC-LDB-007 · AGENTS R19 · [COMPETITION_SYSTEM §13](../../docs/01-game-design/COMPETITION_SYSTEM.md#13-cấu-hình-platform-only-adr-0027) | TC-TEN-024 |
| F108-AC-05 | HỆ THỐNG PHẢI giữ version đã publish bất biến (sửa → `409 CONTENT_NOT_EDITABLE`; bản nháp mới tạo từ `based_on_version_id`); publish PHẢI đặt `effective_from` ≥ hiện tại (không hồi tố trận đã kết thúc), cập nhật con trỏ, ghi `content_releases` và phát `competition_config.published`; rollback/unpublish PHẢI bắt buộc lý do, chỉ đổi con trỏ tới version đã từng publish (khác → `422 ROLLBACK_TARGET_INVALID`) | AGENTS R11, R31 · ADR-0043 | TC-CMS-002, TC-CMS-003 |
| F108-AC-06 | NẾU `If-Match` thiếu khi sửa/chuyển trạng thái, THÌ HỆ THỐNG PHẢI trả `428 PRECONDITION_REQUIRED`; NẾU lệch version, THÌ PHẢI trả `412` với code `CONTENT_VERSION_CONFLICT` | [API_DESIGN §9.13](../../docs/07-backend/API_DESIGN.md) | — |
| F108-AC-07 | HỆ THỐNG PHẢI bắt buộc `Idempotency-Key` cho transition, publish, rollback, unpublish và ghi audit mọi thao tác ghi (actor, `config_type`, `config_id`, version, `effective_from`, lý do) | AGENTS R10 · ADR-0010 | TC-CMS-004 |
| F108-AC-08 | KHI nhận `competition_config.published`, worker PHẢI áp cấu hình vào runtime đúng `effective_from`: mùa (`competition.seasons`), định nghĩa bảng (`leaderboard_definitions`), cấu hình matchmaking/ranking mới cho trận tạo sau thời điểm đó; trận/giải đã bắt đầu PHẢI giữ version đã ghim | FR-CMS-005 · ADR-0043 | — |

---

### User Story 2 — Màn "Thi đấu" trong Game Operations Console (workspace A) (Priority: P1)

Competition Admin mở workspace A → "Thi đấu", thấy 5 nhóm (Matchmaking, Ranking, Mùa giải, Template, Bảng xếp hạng); mỗi nhóm có danh sách version, editor, so sánh, lịch sử, rollback.

**Why this priority**: Không có UI thì platform phải sửa seed bằng tay.

**Independent Test**: Playwright: Competition Admin (MFA) tạo bản nháp ranking-rules, nhập ngưỡng tier giảm dần → hiển thị lỗi trường từ server; publish → bảng trạng thái cập nhật; School Admin mở `/game-ops/competition` → trang 403, không tải chunk `game-ops`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F108-AC-09 | Portal PHẢI hiển thị mục "Thi đấu" trong chunk lazy `game-ops` chỉ khi `GET /me/permissions` có `competition.config.*`; người dùng tenant mở URL trực tiếp PHẢI thấy trang 403 của portal và không tải chunk `game-ops` (không phải lớp bảo mật, backend vẫn chặn) | FR-CMS-005 · ADR-0027 · [ADMIN_PORTAL §1.2](../../docs/04-admin-portal/ADMIN_PORTAL.md#12-admin-portal-5-workspace-adr-0027-mục-7-master-prompt-tái-cấu-trúc-16) | TC-CMS-WS-01, TC-CMS-AUTHZ-05 |
| F108-AC-10 | Portal PHẢI có cho mỗi loại cấu hình: danh sách (ProTable: mã, tiêu đề, trạng thái, version publish, `effective_from`, người sửa), editor `ContentEditorShell` với form sinh từ JSON Schema, so sánh version, lịch sử release, hành động gửi duyệt/duyệt/từ chối/publish (chọn `effective_from`)/rollback (lý do bắt buộc), và trạng thái gán tenant (chỉ đọc) cho template | [ADMIN_PORTAL §3.4, §10.1](../../docs/04-admin-portal/ADMIN_PORTAL.md#34-competition-management-84--workspace-a-platform-only-adr-0027) | TC-CMS-001 |
| F108-AC-11 | Portal PHẢI hiển thị lỗi theo `code` của Problem Details (thông điệp tiếng Việt, `trace_id`), đánh dấu trường theo `VALIDATION_FAILED`, hiện "Cần kết nối mạng quản trị (VPN)" khi không tới gateway nội bộ, và PHẢI không tự áp quy tắc nghiệp vụ ngoài kiểm tra hình dạng schema | AGENTS R1 · [ADMIN_PORTAL §1.2](../../docs/04-admin-portal/ADMIN_PORTAL.md#12-admin-portal-5-workspace-adr-0027-mục-7-master-prompt-tái-cấu-trúc-16) | — |
| F108-AC-12 | Portal PHẢI chỉ dùng component `ops-ui`/Ant Design Pro và token admin (không hardcode màu/cỡ chữ), chuỗi qua i18n vi/en, đạt kiểm tra axe; editor chỉ hỗ trợ desktop, điện thoại chỉ xem | AGENTS R32 · ADR-0047 | TC-UI-003, TC-UI-006 |

---

### User Story 3 — School Admin tổ chức thử thách lớp/giải trường (workspace C) (Priority: P1)

`ad.truong.a` vào workspace C → "Giao bài & lịch" → "Thử thách lớp / giải trường", chọn template đã gán, điền tên/lớp/lịch/bộ câu hỏi, theo dõi đăng ký, xác nhận danh sách, tạo bracket, xem bracket với tên thật, điều chỉnh kết quả khi cần (US-TRN-001).

**Why this priority**: FR-LMS-004 phần P4; backend F107 cần màn hình để nhà trường dùng.

**Independent Test**: Playwright `ad.truong.a`: wizard chỉ hiện trường `allowed_params`; tạo thành công → chi tiết; `qlcm.truong.a` không thấy menu, mở URL → 403.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F108-AC-13 | Portal PHẢI hiển thị mục "Thử thách lớp / giải trường" trong workspace C chỉ khi người dùng có `lms.challenge.write`; `academic_manager`/`tenant_admin` PHẢI không thấy menu và mở URL trực tiếp PHẢI thấy trang 403 | FR-LMS-004 · ADR-0072 · [ADMIN_PORTAL §10](../../docs/04-admin-portal/ADMIN_PORTAL.md#10-màn-hình-chính-sitemap-theo-workspace) | TC-CMS-AUTHZ-06 |
| F108-AC-14 | KHI School Admin mở wizard tạo, Portal PHẢI liệt kê template từ `listChallengeTemplates` (chỉ template đã gán), dựng form chỉ gồm `allowed_params` của template, chọn đối tượng chỉ trong trường mình, gửi `createChallenge` với một `Idempotency-Key` cho mỗi lần bấm; lỗi `403 RESOURCE_NOT_ASSIGNED` và `422 VALIDATION_FAILED` PHẢI hiển thị đúng trường | FR-LMS-004 · AC-LMS-005 · [SCHOOL_LMS §7](../../docs/04-admin-portal/SCHOOL_LMS.md#7-thử-thách-lớp-và-giải-trường) | TC-CMS-AUTHZ-07 |
| F108-AC-15 | Portal PHẢI có danh sách và chi tiết thử thách/giải (trạng thái, đăng ký, check-in, bracket với tên thật học sinh trường mình, kết quả) và các thao tác publish, đăng ký hộ lớp, xác nhận danh sách, tạo bracket, xác nhận/điều chỉnh nút (lý do bắt buộc, xác nhận 2 bước), hủy — mỗi thao tác gọi đúng operationId của F107 | FR-LMS-004 · FR-TRN-002 | — |
| F108-AC-16 | Ở NƠI người dùng là System Admin trong workspace C, Portal PHẢI buộc chọn một tenant đích trước khi tạo/thao tác và gửi `X-Tenant-Id` tenant đó; không chọn PHẢI khóa nút tạo | AGENTS R20 · ADR-0027 | TC-CMS-TEN-03 |
| F108-AC-17 | KHI menu/action của feature (`menu.game_ops.competition`, `menu.school_lms.challenges` và action con) đang tắt cho phạm vi phiên (`disabled_menus`/`disabled_actions` của `getMyPermissions`), Portal PHẢI ẩn menu và ẩn/khóa nút tương ứng (`<ActionGate actionKey>`); NẾU API trả `403 FEATURE_DISABLED`, Portal PHẢI hiện thông báo tính năng tạm tắt và tải lại quyền | FR-CMS-018 · ADR-0090 | TC-CMS-MENU-02 |

### Edge Cases

- NẾU hai người sửa cùng bản nháp, THÌ người lưu sau nhận `412 CONTENT_VERSION_CONFLICT` và portal đề nghị tải lại/so sánh (F108-AC-06).
- NẾU publish ranking-rules đổi ngưỡng tier giữa mùa, THÌ tier chỉ tính lại khi rating thay đổi sau `effective_from`; không thông báo hạ tier hàng loạt.
- NẾU unpublish template đang được giải sử dụng, THÌ giải đã tạo vẫn chạy theo version ghim; tenant không tạo mới được (F107-AC-02).
- NẾU publish mùa chồng thời gian với mùa đang active, THÌ `422 VALIDATION_FAILED`.
- NẾU phiên portal của School Admin đổi tenant trên topbar khi wizard đang mở, THÌ wizard reset và gọi lại API với tenant mới.

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-CMS-005 | Competition Management platform-only: matchmaking config, ranking rules, tournament/challenge template, season, leaderboard definition; tenant chỉ tạo từ template được gán | [ADMIN_PORTAL §3.4](../../docs/04-admin-portal/ADMIN_PORTAL.md#34-competition-management-84--workspace-a-platform-only-adr-0027), [COMPETITION_SYSTEM §13](../../docs/01-game-design/COMPETITION_SYSTEM.md#13-cấu-hình-platform-only-adr-0027) |
| FR-LMS-004 | Màn workspace C cho `school_admin` tổ chức thử thách lớp/giải trường | [SCHOOL_LMS §7](../../docs/04-admin-portal/SCHOOL_LMS.md#7-thử-thách-lớp-và-giải-trường) |
| FR-CMS-018 | Menu/action của feature theo danh mục `specs/portal/menu-catalog.yaml`, ẩn khi tắt, xử lý `403 FEATURE_DISABLED` | [ADMIN_PORTAL §2.7](../../docs/04-admin-portal/ADMIN_PORTAL.md) |

### Key Entities

- **Game Config / Config Version**: một cấu hình thi đấu (loại, mã) và các version (nháp, chờ duyệt, đã publish, lưu trữ) với payload, `effective_from`, tác giả, người duyệt.
- **Content Release**: lịch sử con trỏ publish/rollback/unpublish kèm lý do.
- **Challenge / Tournament** (F107): đối tượng School Admin thao tác trong workspace C.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `mm_rating_window`, `mm_ready_check`, `mm_bot_after`, `mm_ticket_ttl` / `mm_tickets_per_user`, `grade_window_g6_9` | Giá trị khởi tạo form matchmaking |
| `glicko2_params`, `rating_provisional`, `tiers`, `season_length`, `season_soft_reset`, `competition_points_daily_cap`, `effort_points` | Giá trị khởi tạo ranking rules / mùa |
| `battle_1v1_format`, `team_battle_format`, `speed_bonus_min_grade`, `tournament_timers`, `leaderboard_visibility` | Giá trị khởi tạo luật mode, template, bảng; `speed_bonus_min_grade` là ràng buộc không hạ |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| `/game-admin/*` chỉ gateway nội bộ + MFA + role platform; chặn ở backend | AGENTS R7, R20 · ADR-0027 |
| Version publish bất biến; rollback đổi con trỏ | AGENTS R11, R31 · ADR-0043 |
| UI không business logic; token admin | AGENTS R1, R32 · ADR-0047 |
| Phần thưởng không pay-to-win | AGENTS R19 |
| Chuỗi có `maxLength`/`MaxLen` | AGENTS R28 · ADR-0041 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: Competition Admin publish một thay đổi cửa sổ matchmaking và nó áp dụng cho trận mới đúng `effective_from` (sai lệch ≤ 1 phút), không cần deploy.
- **SC-002**: 0 role tenant gọi được bất kỳ `/game-admin/*` nào (kiểm tra tự động mọi route × mọi role tenant).
- **SC-003**: School Admin tạo một giải lớp từ template trong ≤ 3 phút, không cần nhập luật chơi hay phần thưởng.
- **SC-004**: Rollback cấu hình về version trước trong ≤ 2 thao tác, lịch sử release ghi đủ người và lý do.

## Ngoài phạm vi

- Thực thi matchmaking, trận, rating, bảng, giải — F104–F107.
- Gán template cho tenant (workspace D) — F057/F045; khung workspace A, `ContentEditorShell` — F079/F053/F050.
- Giám sát hàng chờ ghép trận trong Live Ops (workspace E) — F080.
- Bật/tắt bảng xếp hạng lớp/trường trong cài đặt trường — F052.
- Giải liên tenant do platform tổ chức (tạo giải platform) — chưa có contract.

## Assumptions

- [Proposed] Demo local dùng `sa.admin` (System Admin kiêm `competition.config.*`, MFA tắt ở local); đề xuất thêm tài khoản `competition_admin` vào LOCAL_DEVELOPMENT.
- [Proposed] Route portal: `/game-ops/competition/{matchmaking,ranking,seasons,templates,leaderboards}` và `/school-lms/challenges` (tên route workspace theo F051).
- [Proposed] Mục "Thử thách lớp / giải trường" nằm trong nhóm "Giao bài & lịch" của workspace C ([ADMIN_PORTAL §10](../../docs/04-admin-portal/ADMIN_PORTAL.md#10-màn-hình-chính-sitemap-theo-workspace)).
