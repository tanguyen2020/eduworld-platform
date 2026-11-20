# Feature Specification: F064 — Exam Zone: thi có giờ theo server, tự nộp, resume, thông báo bài mới

**Feature Branch**: `064-exam-zone` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: `src/game/` · **Mốc**: M-C3 · **Phụ thuộc**: F042, F063
**Input**: Học sinh làm đề kiểm tra/thi thử có giờ theo đồng hồ server, đánh dấu câu, nộp hoặc được tự nộp khi hết giờ, làm tiếp sau rớt mạng, xem kết quả khi được công bố; nhận thông báo trong game khi có bài/đề mới được giao. FR nguồn: FR-EXM-003, FR-EXM-005, FR-NTF-001.

## Clarifications

### Session 2026-11-16

- Q: Câu trả lời khi mất mạng lưu ở đâu? EXAM_ENGINE §6 cho phép "lưu đệm local mã hóa nhẹ", GODOT_ARCHITECTURE §8.2 và GAME_NETWORKING §2.4 cấm lưu câu hỏi đề/câu trả lời xuống đĩa. → A: Theo tài liệu sở hữu client (GODOT_ARCHITECTURE): chỉ hàng đợi RAM `PendingWrites`; mất app → resume lấy câu đã lưu từ server.
- Q: Thông báo bài mới khi M-C3 chưa có realtime (F090) và inbox notification (module chat)? → A: Thông báo trong game dựa trên làm mới `listMyAssignments` tại các điểm kích hoạt (vào Hub, đóng màn học tập/thi, mở 🔔); so sánh với danh sách trong RAM của phiên để phát toast + badge. Push realtime và inbox `/notifications`: F092, F115+.
- Q: Bài giao dạng đề liên kết exam session thế nào? → A: `MyAssignment.exam_session_id` (delta chung với F062); Exam lobby cũng liệt kê `listExamSessions` cho học sinh.
- Q: "Được quay lại câu trước không"? → A: Theo `navigation` của blueprint (`free`/`sequential_no_return`); `ExamSession` cần trường `navigation` (delta).

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Làm bài kiểm tra có giờ theo đồng hồ server (Priority: P1)

Học sinh mở đề từ tab "Nhà trường giao" hoặc Exam lobby, đọc quy chế, bắt đầu; đồng hồ đếm theo server, chuyển câu, đánh dấu, đổi đáp án được lưu ngay, nộp có xác nhận (US-EXM-001).

**Why this priority**: Là nửa còn lại của vòng School Loop (M-C3: đề "Phép nhân" 15 phút).

**Independent Test**: `hs.an.3a` làm đề 15 phút được giao cho 3A, trả lời 10/15 câu, nộp; xác nhận liệt kê 5 câu chưa làm; server có attempt `submitted`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F064-AC-01 | KHI mở một đề, game client PHẢI gọi `getExamSession` và hiện màn quy chế: số câu, thời lượng, `navigation`, thời điểm xem kết quả (`result_release`), câu "Thời gian vẫn chạy nếu mất mạng" | FR-EXM-003 · [STUDENT_GAME_UX_LEARNING §9.1](../../docs/05-ux/STUDENT_GAME_UX_LEARNING.md) | — |
| F064-AC-02 | KHI học sinh bấm `ui.exam.rules_start`, `ExamService` PHẢI gọi `startExamAttempt` với `Idempotency-Key`, PHẢI mở `ExamRunner` với `QualityManager.suspend_world("exam")` và renderer câu hỏi của F063 ở chế độ exam (không phản hồi đúng/sai, không gợi ý, không NPC) | FR-EXM-003 · [EXAM_ENGINE §6](../../docs/02-learning/EXAM_ENGINE.md) | TC-EXM-002 |
| F064-AC-03 | TRONG KHI làm bài, game client PHẢI hiển thị thời gian còn lại = `deadline_at − Clock.server_now()` (bù theo `server_time`), cập nhật 1 lần/giây, sai lệch ≤ 1 s; `g1_3` PHẢI dùng đồng hồ "cát chảy" không số giây | FR-EXM-005 · [GAME_NETWORKING §6](../../docs/08-game-client/GAME_NETWORKING.md) · [LEARNING_EXPERIENCE §8](../../docs/02-learning/LEARNING_EXPERIENCE.md) | TC-EXM-002 |
| F064-AC-04 | KHI còn 5 phút và 1 phút, game client PHẢI đổi màu cảnh báo nhẹ và hiện toast đúng một lần mỗi mốc, không nhấp nháy liên tục | [STUDENT_GAME_UX_LEARNING §9.4](../../docs/05-ux/STUDENT_GAME_UX_LEARNING.md) | — |
| F064-AC-05 | KHI học sinh chọn/đổi đáp án, `ExamService` PHẢI gọi `saveExamAnswer` (`PUT …/answers/{position}`, body `ActivityResponseSubmission` `kind = question`) và PHẢI không hiển thị `is_correct`/giải thích khi đang làm | FR-EXM-003 · AGENTS R6 | TC-EXM-002 |
| F064-AC-06 | Ở NƠI `navigation = sequential_no_return`, game client PHẢI ẩn nút "Câu trước" và lưới câu chỉ cho đi tới; Ở NƠI `navigation = free`, PHẢI cho chuyển tự do qua lưới câu (đã làm/chưa làm/đánh dấu) | FR-EXM-003 · [EXAM_ENGINE §3](../../docs/02-learning/EXAM_ENGINE.md) | — |
| F064-AC-07 | KHI học sinh bấm Nộp, game client PHẢI hiện hộp xác nhận liệt kê câu chưa làm và câu đánh dấu; KHI xác nhận, PHẢI gọi `submitExamAttempt` với `Idempotency-Key` (giữ nguyên khi retry) | FR-EXM-005 · AC-EXR-003 | TC-EXM-003 |
| F064-AC-08 | NẾU `submitExamAttempt` trả `409 EXAM_ALREADY_SUBMITTED`, THÌ game client PHẢI coi như đã nộp và chuyển màn chờ/kết quả, không cho sửa đáp án | FR-EXM-005 | TC-EXM-003 |
| F064-AC-09 | NẾU server trả `403 EXAM_NOT_ASSIGNED`, `403 EXAM_NOT_OPEN`, `403 EXAM_WINDOW_CLOSED`, `409 EXAM_MAX_ATTEMPTS`, THÌ game client PHẢI hiện `ui.error.<CODE>` trên màn quy chế và không mở runner | [EXAM_ENGINE §11](../../docs/02-learning/EXAM_ENGINE.md) · [API_DESIGN §4.1](../../docs/07-backend/API_DESIGN.md) | TC-EXM-002 |
| F064-AC-10 | NẾU token không phải học sinh (`403 GAME_ACCESS_STUDENT_ONLY`) hoặc attempt/đề thuộc tenant khác (`404`), THÌ game client PHẢI không hiển thị đề và về trạng thái lỗi/Login theo F061 | AGENTS R7, R8, R23 · ADR-0031 · ADR-0065 | TC-AUTH-014, TC-TEN-001 |
| F064-AC-11 | Câu trả lời dạng nhập PHẢI giới hạn độ dài theo render payload; client PHẢI không gửi điểm hay thời gian tự tính | ADR-0041 · ADR-0016 | TC-TEN-024 |

---

### User Story 2 — Tự nộp khi hết giờ và làm tiếp sau rớt mạng (Priority: P1)

Hết giờ thì bài bị khóa và nộp tự động; rớt mạng thì thời gian vẫn chạy, câu trả lời giữ trong máy cho tới khi có mạng; mở lại game thì làm tiếp với thời gian còn lại đúng (FR-EXM-005).

**Why this priority**: Kịch bản demo M-C3 có học sinh rớt mạng và resume; chống sửa thời gian.

**Independent Test**: Bắt đầu đề, tắt mạng 2 phút, trả lời 3 câu, bật mạng → 3 câu đồng bộ; thoát game, mở lại sau 5 phút → đồng hồ còn đúng thời lượng − 7 phút.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F064-AC-12 | KHI `Clock.server_now()` ≥ `deadline_at`, game client PHẢI khóa nhập, hiện `ui.exam.auto_submitted`, gửi các câu còn trong hàng đợi rồi gọi `submitExamAttempt` một lần (best effort) và chuyển màn chờ kết quả; client PHẢI không quyết định điểm | FR-EXM-005 · AC-EXM-001 · [EXAM_ENGINE §7.2](../../docs/02-learning/EXAM_ENGINE.md) | TC-EXM-002 |
| F064-AC-13 | NẾU `saveExamAnswer`/`submitExamAttempt` trả `422 EXAM_TIME_OVER`, THÌ game client PHẢI coi bài đã đóng, hiện `ui.exam.auto_submitted` và không retry | FR-EXM-005 · [EXAM_ENGINE §7.2](../../docs/02-learning/EXAM_ENGINE.md) | TC-EXM-003 |
| F064-AC-14 | TRONG KHI mất mạng, game client PHẢI hiện banner mất kết nối, tiếp tục đếm giờ theo offset `Clock` đã đồng bộ, giữ mỗi lần lưu đáp án (bản mới nhất theo `position`) trong `PendingWrites` RAM và đồng bộ khi có mạng; PHẢI không ghi đề/câu trả lời xuống đĩa | FR-EXM-005 · [GODOT_ARCHITECTURE §8.2](../../docs/08-game-client/GODOT_ARCHITECTURE.md) | TC-LRN-009 |
| F064-AC-15 | KHI học sinh mở lại đề có attempt `in_progress`, `ExamService` PHẢI gọi lại `startExamAttempt` và dựng runner từ attempt trả về (items, `saved_responses`, `deadline_at` không đổi) | FR-EXM-005 · AC-EXM-009 trong [EXAM_ENGINE §13](../../docs/02-learning/EXAM_ENGINE.md) | TC-EXM-002 |
| F064-AC-16 | NẾU server trả `409 EXAM_ACTIVE_ON_OTHER_DEVICE`, THÌ game client PHẢI hiện thông báo attempt đang mở ở thiết bị khác và không mở runner | [EXAM_ENGINE §12](../../docs/02-learning/EXAM_ENGINE.md) · ADR-0031 | — |

---

### User Story 3 — Xem kết quả và phân tích câu sai (Priority: P2)

Khi kết quả được công bố, học sinh xem điểm, phân tích theo LO, câu sai (khi được phép xem đáp án), đề xuất ôn tập và bấm "Luyện LO này" (FR-EXM-003).

**Why this priority**: Khép vòng học → ôn; phụ thuộc công bố kết quả của nhà trường.

**Independent Test**: Đề `result_release = immediate`: nộp → xem kết quả ngay; đề `after_close`: nộp sớm → thấy `ui.exam.results_pending`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F064-AC-17 | KHI mở kết quả, game client PHẢI gọi `getExamResult`; NẾU `403 EXAM_RESULTS_NOT_RELEASED`, THÌ PHẢI hiện `ui.exam.results_pending` | FR-EXM-003 · [EXAM_ENGINE §8](../../docs/02-learning/EXAM_ENGINE.md) | TC-EXM-005 |
| F064-AC-18 | KHI có kết quả, game client PHẢI hiện điểm theo grade band, kết quả theo LO, danh sách câu sai với đáp án/giải thích/phản hồi theo phương án chỉ khi response có các trường đó, và PHẢI không xếp hạng/so sánh với bạn | FR-EXM-003 · [LEARNING_EXPERIENCE §7, §9](../../docs/02-learning/LEARNING_EXPERIENCE.md) · AGENTS R16 | TC-EXM-004 |
| F064-AC-19 | KHI kết quả có `recommendations[]`/`weak_objectives`, game client PHẢI hiện nút "Luyện LO này" gọi `startAdHocAttempt` (`source.kind = selection`, `objective_id`) và mở activity player F063 | FR-EXM-003 · [EXAM_ENGINE §10](../../docs/02-learning/EXAM_ENGINE.md) | TC-EXM-004 |

---

### User Story 4 — Thông báo bài mới được giao (Priority: P2)

Khi nhà trường giao bài/đề mới, học sinh thấy toast và badge 🔔 trên HUD lần kế tiếp game làm mới danh sách, mở thẳng tab "Nhà trường giao" (FR-NTF-001).

**Why this priority**: Giúp học sinh biết việc cần làm; chưa có realtime nên dựa trên làm mới theo điểm kích hoạt.

**Independent Test**: `hs.an.3a` đang ở Hub; `ad.truong.a` giao bài mới; học sinh đóng một màn rồi quay lại Hub → toast "Em có bài mới…" + badge 🔔 = 1; bấm toast mở tab.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F064-AC-20 | KHI `AssignmentService` làm mới (vào Hub, đóng màn học tập/thi, mở 🔔) và có `assignment_id` chưa có trong danh sách RAM của phiên với `status = not_started`, `NotificationService` PHẢI hiện toast `ui.notification.assignment_new` và tăng badge 🔔 | FR-NTF-001 · [STUDENT_GAME_UX_CORE S05](../../docs/05-ux/STUDENT_GAME_UX_CORE.md) | TC-LMS-002 |
| F064-AC-21 | KHI đăng nhập lần đầu trong phiên, `NotificationService` PHẢI hiện một toast gộp số bài `not_started` (không một toast mỗi bài) và tối đa 3 toast cùng lúc | FR-NTF-001 · [STUDENT_GAME_UX_CORE S03](../../docs/05-ux/STUDENT_GAME_UX_CORE.md) | — |
| F064-AC-22 | KHI học sinh chạm toast/badge, game client PHẢI mở tab "Nhà trường giao" và xóa badge; thông báo PHẢI không chứa tên học sinh khác hay điểm | FR-NTF-001 · AGENTS R16 | — |
| F064-AC-23 | game client PHẢI không gửi push ngoài game và không tự làm mới định kỳ khi app ở nền | FR-NTF-001 · [PERFORMANCE_BUDGET §12](../../docs/08-game-client/PERFORMANCE_BUDGET.md) | — |

### Edge Cases

- NẾU học sinh có `accommodations[]`, THÌ `deadline_at` server đã tính; client chỉ hiển thị (FR-EXM-009 phía server).
- NẾU đồng hồ thiết bị lệch, THÌ dùng offset `Clock` (F064-AC-03); không dùng giờ hệ điều hành.
- NẾU app mobile vào nền giữa giờ thi, THÌ thời gian vẫn chạy; khi quay lại đồng bộ lại qua `startExamAttempt` (F064-AC-15).
- NẾU cửa sổ thi đóng khi học sinh chưa bắt đầu, THÌ hiện `ui.error.EXAM_WINDOW_CLOSED` (F064-AC-09).
- NẾU locale đổi giữa giờ thi, THÌ chuỗi UI đổi, câu hỏi giữ `display_language` của attempt.
- NẾU đề có `is_official`, THÌ không có XP theo điểm (phần thưởng chỉ hiển thị nếu server trả).

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-EXM-003 | Quy chế, làm bài, nộp, hiển thị kết quả theo công bố, phân tích câu sai, đề xuất ôn tập | [EXAM_ENGINE §6–§10](../../docs/02-learning/EXAM_ENGINE.md), [STUDENT_GAME_UX_LEARNING §9, §10](../../docs/05-ux/STUDENT_GAME_UX_LEARNING.md) |
| FR-EXM-005 | Nộp idempotent, không gửi điểm, khóa + tự nộp theo `deadline_at`, resume, xử lý `EXAM_TIME_OVER` | [EXAM_ENGINE §7, §12](../../docs/02-learning/EXAM_ENGINE.md) |
| FR-NTF-001 | Toast + badge bài/đề mới được giao trong game (phần bài được giao) | [STUDENT_GAME_UX_CORE S03, S05](../../docs/05-ux/STUDENT_GAME_UX_CORE.md) |

### Key Entities

- **Exam session** (đọc): đề được giao — tên, loại, `is_official`, cửa sổ, thời lượng, số câu, `navigation`, FeedbackTiming/ResultRelease, trạng thái.
- **Exam attempt** (đọc/ghi qua API): lượt làm — items, câu đã lưu, `deadline_at`, `server_time`.
- **Exam result** (đọc): điểm, theo LO, câu sai, đề xuất ôn tập.
- **In-game notice** (RAM): toast/badge bài mới, chỉ trong phiên.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `load_time`, `ui_cpu_budget` | Mở runner nhanh; đồng hồ cập nhật 1 lần/giây trong ngân sách UI |
| `deadline_grace` | Tham khảo: ân hạn mạng phía server cho attempt; ân hạn đồng bộ offline của exam chưa đăng ký (xem Assumptions) |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Thời gian do server; client không quyết định hết giờ hay điểm | ADR-0016 · AGENTS R5 · [GAME_NETWORKING §6](../../docs/08-game-client/GAME_NETWORKING.md) |
| Không cache bền đề kiểm tra/câu trả lời | [GODOT_ARCHITECTURE §8.2](../../docs/08-game-client/GODOT_ARCHITECTURE.md) |
| Runner không hiệu ứng, ≥ 30 FPS mobile/web | NFR-COMP-001 · [STUDENT_GAME_UX_LEARNING §9.4](../../docs/05-ux/STUDENT_GAME_UX_LEARNING.md) |
| Không push ngoài game | FR-NTF-003 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: Đồng hồ trên client lệch ≤ 1 s so với `deadline_at` của server trong 100% phép đo trên thiết bị lab.
- **SC-002**: 0 câu trả lời bị mất khi mất mạng ≤ thời lượng đề nếu mạng trở lại trước `deadline_at`.
- **SC-003**: Học sinh rớt mạng và mở lại làm tiếp với đúng thời gian còn lại trong 100% lần thử.
- **SC-004**: Bài mới được giao hiện thông báo trong game ở lần quay về Hub kế tiếp, không cần khởi động lại.

## Ngoài phạm vi

- Sinh đề, chấm, tự nộp phía server, công bố kết quả, gia hạn `accommodations`: F042. Báo cáo đề: F044.
- Scene 3D `exam_zone` (Tháp Thử Thách) hoàn chỉnh: F071; F064 chỉ có điểm vào từ HUD/journal và một `Interactable` greybox trong Main Hub.
- Inbox thông báo `/notifications`, realtime `notification.item.created`, thông báo party/bạn bè/phần thưởng: F092, F115+, F119. Thi có giá trị pháp lý/proctoring: không thuộc Release (OQ-18).

## Assumptions

- F063 cung cấp renderer câu hỏi (chế độ exam), `PendingWrites`; F062 cung cấp `AssignmentService`, HUD, `suspend_world`; F061 cung cấp `Clock`.
- Delta [Confirmed]: `MyAssignment.exam_session_id` (chung F062), `ExamSession.navigation`, tham số `takeover` cho `startExamAttempt` (contracts §6).
- Ân hạn đồng bộ offline 30 giây của exam ([EXAM_ENGINE §6](../../docs/02-learning/EXAM_ENGINE.md)) chưa có trong CONFIG_DEFAULTS; client không dựa vào giá trị này (server quyết định qua `EXAM_TIME_OVER`).
