# Feature Specification: F107 — Tournament: 6 format, bracket, lịch, thử thách khối/giải trường từ template

**Feature Branch**: `107-tournament-challenges` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: module `competition` · **Mốc**: M4b · **Phụ thuộc**: F105, F106
**Input**: School Admin tạo thử thách khối (`grade_challenge`) hoặc giải khối/giải trường từ template platform đã gán cho tenant — đối tượng là một khối của trường ± danh sách học sinh cùng trường (ADR-0095); hệ thống quản lý đăng ký, xác nhận, check-in, chia bảng, sinh bracket, lên lịch, tạo trận, xác nhận kết quả, trao thưởng và lưu lịch sử bền vững. FR nguồn: FR-TRN-001…003, FR-LMS-004; US-TRN-001, US-LMS-008 (**Removed (ADR-0095)** — thay bằng US-TRN-001).

## Clarifications

### Session 2026-11-16

- Q: `POST /lms/challenges` thuộc module nào? Spec hiện ghi `x-service: learning`. → A: module competition (route ngoại lệ dưới `/lms/*`, giống `/lms/event-templates` → module game). Template, phép gán template (`infra.tenant_resource_assignments` của DB competition) và việc thực thi đều ở module competition ([SCHOOL_LMS §7](../../docs/04-admin-portal/SCHOOL_LMS.md#7-thử-thách-lớp-và-giải-trường), [COMPETITION_SYSTEM §10.2](../../docs/01-game-design/COMPETITION_SYSTEM.md#102-mô-hình)); đổi `x-service` là delta Phase 2 [Confirmed].
- Q: "6 format" gồm gì và phase? → A: Single Elimination, Double Elimination, Round Robin (thuật toán), Team (entry là đội), School, Grade Tournament (phạm vi; trước ADR-0095 là Class Tournament). PRD: single elim + round robin ở P4, còn lại P6 — US5 (P3) làm phần P6, tắt theo template tới khi platform bật.
- Q: Ai nhận gán template từ System Admin? → A: F107 làm bên nhận `assignTenantResources`/`unassignTenantResources`/`listTenantResourceAssignments` (internal, F045 khai báo) cho `tournament_template`, `challenge_template`.
- Q: Màn hình portal cho School Admin thuộc feature nào? → A: F108 (workspace C); F107 chỉ backend.

### Session 2026-11-21 (ADR-0095 — [Confirmed])

- Q: Thử thách lớp/giải lớp còn không? → A: Không. Theo [ADR-0095 mục 5 (D5)](../../docs/adr/ADR-0095-bo-khai-niem-lop-hoc-sinh-theo-khoi.md#5-game-xã-hội-chat-d2-d5-d7-confirmed--owner-2026-11-21) thay bằng **thử thách khối/giải khối của trường**: template `grade_challenge` (thay `class_challenge`/`class_tournament`) và `school_tournament`; vẫn chỉ `school_admin` (trường mình) và System Admin (tenant đích) tạo từ template platform (ADR-0072, ADR-0027) — FR-TRN-001, FR-LMS-004, US-TRN-001.
- Q: Đối tượng của thử thách khối là gì? → A: Trong **một trường** (`school_id` bắt buộc): `targets[]` gồm `{target_type: grade, grade_level}` và/hoặc `{target_type: students, student_ids ≤ 500}` (học sinh cùng trường, có thể khác khối), trừ `exclude_student_ids` (≤ 500); **exclude thắng include**; học sinh trường khác → `422 VALIDATION_FAILED` (cùng quy tắc đối tượng với giao bài theo khối, ADR-0095 mục 3; [COMPETITION_SYSTEM §10.4](../../docs/01-game-design/COMPETITION_SYSTEM.md#104-đăng-ký-và-xác-nhận-confirmed-trừ-khi-ghi-khác)). Giải khối của trường = `TournamentCreate.scope_type = school` + `grade_level`.
- Q: Seeding `separate_class`? → A: Bỏ (không còn lớp); còn `rating`, `random` (mặc định giải khối lớp 1–5), `manual`.
- Q: Thử thách theo nhóm học tập (`participation = study_group`)? → A: Bỏ — nhóm học tập LMS đã bỏ (ADR-0095 mục 1); `participation` chỉ còn `individual` [Proposed — nếu cần thử thách theo đội, mở câu hỏi riêng; giải đội dùng `participant_type = team` của tournament như cũ].
- Q: Contract cần đổi? → A: Đã áp bởi lead: `TournamentCreate.scope_type` = `school`, `organization` (bỏ `class`). Chờ áp sau WP-L1 (ops `learning.v1.yaml`): `listChallenges` query `class_id` → `grade_level`, `ChallengeTemplate.kind` → `grade_challenge`, `school_tournament`, `ChallengeCreate` thêm `school_id`, `targets[]`, `exclude_student_ids`, `participation` bỏ `study_group` — ghi ở [contracts §6](contracts/README.md#6-delta-cần-thực-hiện).
- Q: Học sinh đổi khối/chuyển trường sau khi đăng ký? → A: Entry giữ tới khi khóa bracket (như cũ). Điều kiện thuộc đối tượng được xét theo `ref.students(school_id, grade_level)` hiện hành tại lúc đăng ký/đăng ký hộ, nên học sinh mới vào khối trước khi đóng đăng ký (`student.placement_changed`) đủ điều kiện nếu không bị exclude [Proposed — theo tinh thần ADR-0095 mục 3].

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — School Admin tạo thử thách khối/giải trường từ template (Priority: P1) 🎯 MVP

`ad.truong.a` chọn template "Giải Toán khối" (`grade_challenge`) đã gán, đặt tên "Đấu trường Toán khối 3", chọn khối 3 của TH Demo A, thêm một học sinh khối 4 và loại trừ một học sinh khối 3, chọn lịch, bộ câu hỏi; luật chơi và phần thưởng lấy nguyên từ template (US-TRN-001).

**Why this priority**: Không có giải/thử thách thì không có luồng nào khác; là điểm nối LMS ↔ thi đấu.

**Independent Test**: `createChallenge` với template đã gán, `targets` khối 3 + 1 học sinh khối 4, `exclude_student_ids` 1 học sinh → `201`, `tournament_id` khác null, giải `draft` ghim version template, đối tượng = khối 3 − 1 + 1; tham số `reward` → `422`; học sinh trường khác trong `student_ids` → `422`; `qlcm.truong.a` gọi → `403`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F107-AC-01 | KHI người có `lms.challenge.write` gọi `createChallenge` với template đã publish và được gán cho tenant, module competition PHẢI tạo Challenge ghim `template_version_id`, lấy luật chơi, phần thưởng, ràng buộc từ template; template `school_tournament` và template `grade_challenge` có format giải PHẢI tạo kèm Tournament `draft` (giải khối: `scope_type = school` + `grade_level`); template `grade_challenge` dạng Timed Quiz PHẢI tạo thử thách Timed Quiz `scheduled`; template/`kind` `class_challenge`, `class_tournament` PHẢI bị từ chối `422 VALIDATION_FAILED` (ADR-0095) | FR-LMS-004 · FR-TRN-002 · [COMPETITION_SYSTEM §10.2](../../docs/01-game-design/COMPETITION_SYSTEM.md#102-mô-hình) | TC-CMS-AUTHZ-07 |
| F107-AC-02 | NẾU template chưa gán cho tenant (hoặc đã gỡ gán), THÌ HỆ THỐNG PHẢI trả `403 RESOURCE_NOT_ASSIGNED`; NẾU `params` có khóa ngoài `allowed_params` của template (ví dụ `reward`, luật điểm), THÌ PHẢI trả `422 VALIDATION_FAILED` chỉ rõ khóa | AC-LMS-005 · ADR-0027 · ADR-0029 | TC-CMS-ASG-01, TC-CMS-AUTHZ-07 |
| F107-AC-03 | HỆ THỐNG PHẢI chỉ cho `school_admin` (trường mình) và System Admin (tenant đích) tạo/sửa/hủy; `academic_manager`, `tenant_admin` PHẢI nhận `403 PERMISSION_DENIED`; `school_id` ngoài scope UserRole hoặc thuộc tenant khác PHẢI nhận `404 NOT_FOUND`; System Admin không gửi tenant đích PHẢI nhận `422 VALIDATION_FAILED` | FR-LMS-004 · ADR-0072 · ADR-0095 · [SCHOOL_LMS §9](../../docs/04-admin-portal/SCHOOL_LMS.md#9-phạm-vi-dữ-liệu-theo-vai-trò) | TC-TEN-003, TC-CMS-TEN-03 |
| F107-AC-04 | HỆ THỐNG PHẢI bắt buộc `Idempotency-Key` cho `createChallenge` (gửi lại → cùng Challenge) và ghi audit `challenge.created`, `challenge.cancelled`, `tournament.published`, `tournament.cancelled` (actor, tenant, trường, template version) | AGENTS R10 · ADR-0010 | — |
| F107-AC-05 | NẾU `name` vượt `maxLength`, `ends_at ≤ starts_at`, lịch ngoài khoảng template cho phép, `targets` rỗng, `student_ids`/`exclude_student_ids` vượt 500, `grade_level` ngoài 1–9 hoặc ngoài `grade_levels` của template, hay có học sinh không thuộc `school_id` (khác trường), THÌ HỆ THỐNG PHẢI trả `422 VALIDATION_FAILED`; đối tượng hiệu lực PHẢI = (học sinh hiệu lực của khối trong trường ∪ `student_ids`) − `exclude_student_ids` (exclude thắng) | ADR-0041 · ADR-0095 | TC-TEN-024 |
| F107-AC-06 | KHI người tổ chức gọi `listChallenges` (lọc theo `grade_level`, không còn `class_id` — ADR-0095)/`getChallenge`, HỆ THỐNG PHẢI trả thử thách/giải trong phạm vi (trạng thái, đối tượng khối ± học sinh, số tham gia, kết quả); `cancelChallenge` PHẢI hủy (soft) khi chưa chạy, hoặc chuyển `cancelled` có lý do khi đang chạy; kết quả thử thách PHẢI không xuất hiện trong báo cáo kết quả học tập | FR-LMS-004 · [SCHOOL_LMS §7, §9](../../docs/04-admin-portal/SCHOOL_LMS.md#7-thử-thách-lớp-và-giải-trường) | — |

---

### User Story 2 — Đăng ký, xác nhận, check-in (Priority: P1)

Học sinh khối 3 TH Demo A đăng ký "Đấu trường Toán khối 3" (hoặc School Admin đăng ký hộ theo khối ± danh sách học sinh), School Admin xác nhận danh sách; 15 phút trước giờ đấu học sinh check-in, ai không check-in bị rút và bracket nhận bye.

**Why this priority**: FR-TRN-002 yêu cầu đăng ký và xác nhận người tham gia trước khi chia bảng.

**Independent Test**: Giải `registration_open`: `hs.an.3a` đăng ký → `registered`; `hs.dung.7a` (trường khác) → `404`; đủ `max_entries` → `409`; quá giờ check-in không check-in → `withdrawn`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F107-AC-07 | HỆ THỐNG PHẢI chạy vòng đời giải `draft → published → registration_open → registration_closed → seeding → scheduled → in_progress → completed → rewarded → archived` (hủy được từ `draft`, `published`, `registration_open`, `scheduled`) bằng command và job theo thời điểm; chuyển trạng thái sai PHẢI trả `422 INVALID_STATE_TRANSITION` | FR-TRN-002 · [COMPETITION_SYSTEM §10.3](../../docs/01-game-design/COMPETITION_SYSTEM.md#103-vòng-đời) | — |
| F107-AC-08 | KHI học sinh gọi `registerTournamentEntry` trong `registration_open`, HỆ THỐNG PHẢI kiểm tra khối thuộc `grade_levels`, thuộc phạm vi giải (đối tượng khối ± học sinh của thử thách khối/trường/tenant), không bị hạn chế thi đấu; học sinh ngoài phạm vi PHẢI nhận `404 NOT_FOUND`; bị hạn chế PHẢI nhận `403 POLICY_DENIED`; đủ `max_entries` PHẢI nhận `409 CONFLICT`; ngoài cửa sổ PHẢI nhận `422 INVALID_STATE_TRANSITION`; gửi lại cùng `Idempotency-Key` PHẢI trả cùng entry | FR-TRN-002 · AC-TRN-006 · [COMPETITION_SYSTEM §10.4](../../docs/01-game-design/COMPETITION_SYSTEM.md#104-đăng-ký-và-xác-nhận-confirmed-trừ-khi-ghi-khác) | TC-TEN-003 |
| F107-AC-09 | KHI người tổ chức đăng ký hộ theo đối tượng của thử thách khối (khối ± `student_ids` − `exclude_student_ids`, ADR-0095) hoặc gọi `confirmTournamentEntries`, HỆ THỐNG PHẢI tạo/xác nhận entry cho mọi học sinh active đủ điều kiện, không tạo entry cho học sinh bị exclude; Ở NƠI template bật auto-confirm, HỆ THỐNG PHẢI tự xác nhận khi đóng đăng ký; `withdrawTournamentEntry` PHẢI chuyển entry `withdrawn` (soft) trước khi khóa bracket | FR-TRN-002 · [COMPETITION_SYSTEM §10.4](../../docs/01-game-design/COMPETITION_SYSTEM.md#104-đăng-ký-và-xác-nhận-confirmed-trừ-khi-ghi-khác) | — |
| F107-AC-10 | TRONG KHI cửa sổ check-in theo `tournament_timers` mở, HỆ THỐNG PHẢI nhận `checkInTournamentEntry` của chính học sinh; KHI cửa sổ đóng, entry chưa check-in PHẢI chuyển `withdrawn` và bracket PHẢI điều chỉnh bye trước khi khóa; NẾU số entry dưới tối thiểu của format, THÌ giải PHẢI chuyển `cancelled` và thông báo người tổ chức | FR-TRN-002 · [COMPETITION_SYSTEM §10.4](../../docs/01-game-design/COMPETITION_SYSTEM.md#104-đăng-ký-và-xác-nhận-confirmed-trừ-khi-ghi-khác) | — |

---

### User Story 3 — Seeding và sinh bracket (Priority: P1)

Đóng đăng ký với 5 học sinh, School Admin bấm "Tạo bracket": 8 vị trí, 3 bye cho hạt giống 1–3, hạt giống 1 và 2 ở hai nửa nhánh; khối 3 mặc định xếp ngẫu nhiên có lưu seed.

**Why this priority**: Bracket đúng là điều kiện công bằng của giải; FR-TRN-003 yêu cầu lưu bền vững.

**Independent Test**: Unit test N = 2..64 cho single elim và round robin; `generateTournamentBracket` hai lần cùng key → cùng bracket; sau khi khóa gọi lại → `409`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F107-AC-11 | Ở NƠI format là `single_elim`, HỆ THỐNG PHẢI tạo số slot là lũy thừa của 2 ≥ N, bye cho hạt giống cao, vị trí chuẩn để hạt giống 1 và 2 chỉ gặp ở chung kết (N = 5 → 8 slot, 3 bye cho hạt giống 1–3) | FR-TRN-001 · AC-TRN-001 · [COMPETITION_SYSTEM §10.5](../../docs/01-game-design/COMPETITION_SYSTEM.md#105-seeding-và-bracket) | — |
| F107-AC-12 | Ở NƠI format là `round_robin`, HỆ THỐNG PHẢI xếp lịch bằng circle method (N lẻ thêm bye), mỗi entry đấu N−1 trận không trùng cặp, xếp hạng thắng 3/hòa 1/thua 0 → hiệu số câu đúng → đối đầu → tổng thời gian trả lời đúng | FR-TRN-001 · AC-TRN-003 | — |
| F107-AC-13 | HỆ THỐNG PHẢI hỗ trợ `seeding_policy` `rating` (μ Glicko-2 mùa hiện tại, provisional xếp sau), `random` (seed lưu để audit; mặc định giải khối lớp 1–5), `manual`; `seeding_policy = separate_class` PHẢI bị từ chối `422 VALIDATION_FAILED` (bỏ theo ADR-0095) | [COMPETITION_SYSTEM §10.5](../../docs/01-game-design/COMPETITION_SYSTEM.md#105-seeding-và-bracket) | — |
| F107-AC-14 | KHI người tổ chức gọi `generateTournamentBracket`, HỆ THỐNG PHẢI sinh toàn bộ bảng/nút bracket trong một transaction PostgreSQL (không chỉ Valkey), idempotent theo `Idempotency-Key`, ghi audit; gọi lại sau khi giải `scheduled` PHẢI trả `422 INVALID_STATE_TRANSITION`; N dưới tối thiểu PHẢI trả `422 VALIDATION_FAILED` | FR-TRN-003 · ADR-0004 | — |

---

### User Story 4 — Lịch, trận, xác nhận kết quả, trao thưởng (Priority: P1)

Tới giờ, hệ thống mời hai bạn vào ready check; ai không vào sau 3 phút bị xử thua; kết quả trận tự đẩy người thắng lên vòng sau; School Admin có 10 phút xem log để điều chỉnh khi phát hiện gian lận; giải kết thúc, mọi người hoàn thành ít nhất một trận nhận thưởng tham gia.

**Why this priority**: Phần "quản lý trận, xác nhận kết quả, trao thưởng, lưu lịch sử" của FR-TRN-002.

**Independent Test**: Phát `match.completed` hai lần cho một nút → người thắng lên vòng sau một lần; `overrideBracketNode` sau khi nút kế bắt đầu → `409`; trao thưởng chạy hai lần → mỗi entry nhận một lần.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F107-AC-15 | KHI tới `scheduled_at` của nút có đủ hai entry, worker PHẢI tạo trận qua proposal trực tiếp + ready check của F104 (`tournament_node_id`), và PHẢI phát `tournament.match_scheduled` trước giờ đấu 10 phút và 1 phút | FR-TRN-002 · [COMPETITION_SYSTEM §10.6](../../docs/01-game-design/COMPETITION_SYSTEM.md#106-lịch-thi-đấu) | — |
| F107-AC-16 | NẾU một entry không vào ready check sau thời hạn no-show (`tournament_timers`), THÌ HỆ THỐNG PHẢI xử `forfeit`, đối thủ đi tiếp; NẾU cả hai không vào, THÌ single elim loại cả hai (nút sau nhận bye), round robin tính cả hai thua | AC-TRN-005 · [COMPETITION_SYSTEM §10.7](../../docs/01-game-design/COMPETITION_SYSTEM.md#107-no-show-disconnect-forfeit-kết-quả-confirmed) | — |
| F107-AC-17 | KHI nhận `match.completed` của trận giải, HỆ THỐNG PHẢI ghi `winner_entry_id` và đẩy entry sang nút kế (thắng/thua) idempotent theo `match_id`; hết cửa sổ khiếu nại (`tournament_timers`) PHẢI chuyển nút `confirmed`; chung kết xác nhận PHẢI chuyển giải `completed` và sinh kết quả thứ hạng | FR-TRN-002 · AC-TRN-004 | TC-CMP-009 |
| F107-AC-18 | KHI người tổ chức gọi `overrideBracketNode` trong cửa sổ khiếu nại với lý do bắt buộc, HỆ THỐNG PHẢI đổi người thắng, ghi audit (actor, lý do, trước/sau); NẾU nút kế đã bắt đầu hoặc thiếu lý do, THÌ PHẢI trả `422 INVALID_STATE_TRANSITION` / `422 VALIDATION_FAILED` | AC-TRN-007 · [ANTI_CHEAT §8](../../docs/10-security/ANTI_CHEAT.md#8-bảo-vệ-api-thi-đấu) | — |
| F107-AC-19 | NẾU trận giải `aborted` do sự cố server, THÌ HỆ THỐNG PHẢI lên lịch lại sau 2 phút, tối đa 2 lần, sau đó chờ người tổ chức quyết (override) | [COMPETITION_SYSTEM §10.7](../../docs/01-game-design/COMPETITION_SYSTEM.md#107-no-show-disconnect-forfeit-kết-quả-confirmed) | — |
| F107-AC-20 | KHI giải `completed`, HỆ THỐNG PHẢI ghi Competition Points `source_type = tournament`, trao thưởng theo `reward_rule_refs` của template (vô địch, á quân, top 4, tham gia ≥ 1 trận) idempotent theo `(tournament_id, entry_id, rule_id)`, phát `tournament.rewarded` cho module game cấp vật phẩm, chuyển `rewarded`; trao thưởng chạy lại PHẢI không cấp lần hai; phần thưởng PHẢI không ảnh hưởng điểm học tập | FR-TRN-002 · AC-TRN-008 · AGENTS R19 | — |
| F107-AC-21 | HỆ THỐNG PHẢI không đổi rating từ trận giải (chỉ dùng rating để seed) và PHẢI lưu bracket, kết quả, thứ hạng lâu dài trong PostgreSQL | FR-TRN-003 · [COMPETITION_SYSTEM §10.8](../../docs/01-game-design/COMPETITION_SYSTEM.md#108-trao-thưởng-và-lịch-sử) | — |

---

### User Story 5 — Format mở rộng: double elim, vòng bảng, giải đội, giải trường (Priority: P3)

Giải trường khối 6 theo đội 3 người, vòng bảng rồi loại trực tiếp; giải nhánh thắng/thua cho câu lạc bộ.

**Why this priority**: PRD xếp ngoài single elim/round robin ở P6; làm sau khi luồng P4 ổn định, bật theo template.

**Independent Test**: Unit test double elim: người thua vòng 1 nhánh thắng vào đúng vị trí nhánh thua; groups N = 12 → 3 bảng snake, top 2 vào knockout; giải đội thiếu người dưới `team_size − 1` → forfeit.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F107-AC-22 | Ở NƠI format là `double_elim`, HỆ THỐNG PHẢI đưa người thua nhánh thắng vào nhánh thua theo bảng ánh xạ chuẩn, "bracket reset" ở chung kết mặc định tắt | FR-TRN-001 · AC-TRN-002 | — |
| F107-AC-23 | Ở NƠI format là `groups_then_single_elim`, HỆ THỐNG PHẢI chia bảng 4–6 theo hạt giống kiểu snake, chạy round robin trong bảng, đưa top 2 mỗi bảng vào loại trực tiếp | FR-TRN-001 · [COMPETITION_SYSTEM §10.5](../../docs/01-game-design/COMPETITION_SYSTEM.md#105-seeding-và-bracket) | — |
| F107-AC-24 | Ở NƠI `participant_type = team`, HỆ THỐNG PHẢI nhận entry là đội (mọi thành viên đủ điều kiện, cùng trường), cho thi đấu khi còn ≥ `team_size − 1` người, dưới mức đó xử `forfeit`; giải scope trường/tenant đơn vị PHẢI chỉ nhận học sinh trong phạm vi đó | FR-TRN-001 · [COMPETITION_SYSTEM §10.4, §10.7](../../docs/01-game-design/COMPETITION_SYSTEM.md#104-đăng-ký-và-xác-nhận-confirmed-trừ-khi-ghi-khác) | — |

---

### User Story 6 — Học sinh xem giải, tham gia thử thách khối; gán template (Priority: P1)

`hs.an.3a` mở danh sách giải trong game, xem bracket dạng "con đường"; tới giờ thử thách khối "Thử thách nhân nhanh khối 3" bấm "Tham gia" để vào phòng Timed Quiz. System Admin gán template giải cho tenant `th-demo-a`.

**Why this priority**: Không xem/tham gia được thì giải không chạy; gán template là điều kiện để tenant tạo giải.

**Independent Test**: `hs.an.3a` `getTournamentBracket` → chỉ nickname; `hs.dung.7a` → 404; `joinChallenge` trong khung → room; ngoài khung → 409; identity gọi `assignTenantResources` → `tenant_resource_assignments` có dòng.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F107-AC-25 | KHI học sinh gọi `listTournaments`, `getTournament`, `getTournamentBracket`, HỆ THỐNG PHẢI chỉ trả giải trong phạm vi của học sinh (đối tượng khối ± học sinh/trường/tenant) với nickname/avatar, không tên thật; giải ngoài phạm vi hoặc tenant khác PHẢI trả `404 NOT_FOUND`; người tổ chức (portal) PHẢI thấy tên thật trong trường thuộc scope | FR-TRN-002 · AGENTS R16 · [COMPETITION_SYSTEM §10.8](../../docs/01-game-design/COMPETITION_SYSTEM.md#108-trao-thưởng-và-lịch-sử) | TC-TEN-001 |
| F107-AC-26 | HỆ THỐNG PHẢI cho route học sinh (`listTournaments`, `register…`, `checkIn…`, `listMyChallenges`, `joinChallenge`) chỉ qua `game_player` (nhân sự → `403 GAME_ACCESS_STUDENT_ONLY`) và route người tổ chức (`publish`, `cancel`, `confirm`, `generate-bracket`, `confirm`/`override` nút) chỉ qua portal với `lms.challenge.write` cho trường sở hữu; `X-Tenant-Id` khác tenant phiên PHẢI nhận `403 TENANT_MISMATCH` | AGENTS R7, R23 · ADR-0031 | TC-AUTH-014, TC-TEN-011 |
| F107-AC-27 | KHI học sinh thuộc đối tượng gọi `joinChallenge` trong khung tham gia của thử thách khối, HỆ THỐNG PHẢI đưa học sinh vào room Timed Quiz (F105) của thử thách; ngoài khung PHẢI trả `422 INVALID_STATE_TRANSITION`; không thuộc đối tượng PHẢI trả `404 NOT_FOUND`; `listMyChallenges` PHẢI trả thử thách đang/sắp diễn ra của học sinh | FR-LMS-004 · [COMPETITION_SYSTEM §3.4](../../docs/01-game-design/COMPETITION_SYSTEM.md#34-timed-quiz) | — |
| F107-AC-28 | HỆ THỐNG PHẢI không có tham số template hay `allowed_params` nào tắt chat `match`/`team` của trận giải; log trận dùng cho khiếu nại PHẢI không chứa nội dung chat | AGENTS R15 · ADR-0020 · [COMPETITION_SYSTEM §10.7](../../docs/01-game-design/COMPETITION_SYSTEM.md#107-no-show-disconnect-forfeit-kết-quả-confirmed) | — |
| F107-AC-29 | KHI module identity gọi facade `assignTenantResources`/`unassignTenantResources` cho `tournament_template`/`challenge_template` đã publish, module competition PHẢI ghi/soft-delete dòng `tenant_resource_assignments` idempotent và audit; NẾU module khác ngoài `x-allowed-callers` import facade này, THÌ `make arch-check` PHẢI fail; gỡ gán PHẢI không ảnh hưởng giải đã tạo | ADR-0029 · ADR-0074 · AGENTS R22, R29 | TC-SVC-006 |

### Edge Cases

- NẾU template có version mới publish sau khi tạo giải, THÌ giải vẫn chạy theo version đã ghim (F107-AC-01).
- NẾU học sinh đổi khối/chuyển trường sau khi đăng ký, THÌ entry giữ tới khi khóa bracket; sau đó vẫn thi đấu (đã khóa) — không tự rút.
- NẾU học sinh bị dọn (soft-delete) giữa giải, THÌ trận kế tiếp xử `forfeit`.
- NẾU hai worker cùng tạo trận cho một nút, THÌ proposal trực tiếp idempotent theo `node_id` (F107-AC-15).
- NẾU override đổi người thắng sau khi người cũ đã được đẩy sang nút kế (nút kế chưa bắt đầu), THÌ nút kế được cập nhật entry trong cùng transaction (F107-AC-18).
- NẾU khung giờ truy cập của trường (OQ-17) không bao trùm giờ đấu, THÌ `422 VALIDATION_FAILED` khi lên lịch [Confirmed].

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-TRN-001 | 6 format: single/double elim, round robin, vòng bảng, đội, giải trường/giải khối của trường (thay giải lớp — ADR-0095) | [COMPETITION_SYSTEM §10.1, §10.5](../../docs/01-game-design/COMPETITION_SYSTEM.md#101-yêu-cầu-nguồn-confirmed) |
| FR-TRN-002 | Tạo giải, đăng ký, chia bảng, bracket, lịch, xác nhận, quản lý trận, xác nhận kết quả, trao thưởng, lịch sử | [COMPETITION_SYSTEM §10.2–§10.8](../../docs/01-game-design/COMPETITION_SYSTEM.md#102-mô-hình) |
| FR-TRN-003 | Bracket bền vững PostgreSQL | [COMPETITION_SYSTEM §10.1](../../docs/01-game-design/COMPETITION_SYSTEM.md#101-yêu-cầu-nguồn-confirmed) |
| FR-LMS-004 | Chỉ `school_admin` tổ chức thử thách khối/giải trường từ template (backend; đối tượng khối ± học sinh) | [SCHOOL_LMS §7](../../docs/04-admin-portal/SCHOOL_LMS.md#7-thử-thách-lớp-và-giải-trường) |

### Key Entities

- **Challenge**: thử thách/giải do nhà trường tạo từ một version template; trường, đối tượng (khối ± danh sách học sinh, danh sách loại trừ), lịch, tham số cho phép, trạng thái.
- **Tournament**: giải đấu (format, kiểu entry, phạm vi, khối, chính sách seed, lịch, trạng thái, phiên bản luật).
- **Tournament Entry**: học sinh hoặc đội tham gia, hạt giống, trạng thái `registered → confirmed → checked_in → eliminated | champion` (hoặc `withdrawn`, `disqualified`).
- **Tournament Group / Bracket Node**: bảng đấu; nút bracket (nhánh, vòng, vị trí, hai entry, trận, người thắng, nút kế thắng/thua, trạng thái).
- **Tournament Result / Reward**: thứ hạng cuối, điểm, thưởng đã trao.
- **Template assignment**: template platform được gán cho tenant.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `tournament_timers` | Check-in, no-show, grace trận giải, cửa sổ khiếu nại |
| `disconnect_grace` | Grace trận giải (F105 áp) |
| `glicko2_params` | Seed theo rating mặc định khi chưa có rating |
| `idempotency_record_ttl` | Hạn Idempotency-Key |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Giải luôn từ template platform; tenant chỉ điền `allowed_params` | AGENTS R20 · ADR-0027 · ADR-0029 |
| Chỉ `school_admin` trong role tenant tạo thử thách | ADR-0072 · FR-LMS-004 |
| Bracket, kết quả, thưởng bền vững PostgreSQL | AGENTS R9 · ADR-0004 |
| Ghi quan trọng idempotent + audit | AGENTS R10 |
| Chỉ nickname với người chơi khác | AGENTS R16 |
| Không phần thưởng ảnh hưởng học tập, không pay-to-win | AGENTS R19 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: Bracket sinh đúng với mọi N = 2..64 (single elim, round robin), hạt giống 1 và 2 không gặp trước chung kết trong 100% trường hợp.
- **SC-002**: Xử lý lặp `match.completed` và chạy trao thưởng hai lần: 0 entry bị đẩy hai lần, 0 phần thưởng cấp trùng.
- **SC-003**: Một giải khối 32 học sinh chạy từ đăng ký tới trao thưởng không cần thao tác tay ngoài tạo giải, xác nhận danh sách và tạo bracket.
- **SC-004**: 0 trường hợp tenant tạo giải với tham số ngoài `allowed_params` hoặc template chưa gán.

## Ngoài phạm vi

- Màn hình portal tạo/quản lý thử thách (workspace C) và soạn template (workspace A) — F108.
- Trận đấu, Timed Quiz room, grace — F105; proposal trực tiếp — F104; rating, Competition Points ledger — F106.
- Cấp vật phẩm/badge/title thưởng giải — module game (tiêu thụ `tournament.rewarded`); giao diện bracket trong Godot — F109.
- Giải liên tenant do platform tổ chức (scope `platform`) — chỉ chuẩn bị mô hình (`tenant_ids`), API tạo thuộc F108 sau.
- Saga gán/gỡ gán phía module identity — F045.

## Assumptions

- [Confirmed] `/lms/challenges` và `/lms/catalog/challenge-templates` chuyển `x-service` sang competition (route ngoại lệ APISIX).
- [Confirmed] Khung tham gia thử thách khối: từ `starts_at` trừ 5 phút tới khi câu cuối mở; giá trị trong template.
- [Confirmed] Tenant Admin tenant `unit` tạo giải liên trường ([COMPETITION_SYSTEM §10.2](../../docs/01-game-design/COMPETITION_SYSTEM.md#102-mô-hình)) mâu thuẫn ADR-0072/ADMIN_PORTAL §2.3 (Tenant Admin không có `lms.challenge.write`) — spec theo ADMIN_PORTAL; ghi vào báo cáo.
- Template giải/thử thách được seed và gán cho `th-demo-a` ở môi trường local tới khi F108 xong.
