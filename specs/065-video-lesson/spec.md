# Feature Specification: F065 — Bài học video: plugin `video_lesson` và renderer Godot

**Feature Branch**: `065-video-lesson` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: module `learning` + `src/game/` · **Mốc**: M-C3 · **Phụ thuộc**: F063 (và F030 attempt engine, F027 media/transcode Theora, F031 learning events)
**Input**: Activity `video_lesson`: video có chương, phụ đề WebVTT vi/en, resume, câu hỏi chèn tự chấm tại mốc thời gian, nhánh tương tác, hoàn thành theo phần đã xem thật — plugin Go trong Activity Registry và renderer trong activity player Godot. FR nguồn: FR-CNT-014 (kèm FR-EXR-004 resume, FR-CNT-011 ghim version).

## Clarifications

### Session 2026-11-16

- Q: Câu trả lời câu chèn lưu ở bảng nào? VIDEO_LEARNING §8/DATABASE_DESIGN §2.6a nêu `activity_attempt_responses` (part `cue_id`), nhưng gợi ý (`hints:next` theo `position`), `hints_used`, `rapid_guess` chỉ có ở `attempt_items`/`attempt_answers`. → A: Mỗi câu chèn là một item (`attempt_items.position` theo thứ tự `at_ms`); câu trả lời ghi `assessment.attempt_answers` như câu hỏi thường (chung scorer F030). Lý do và phương án loại: [research.md](research.md) R1.
- Q: "Sau 2 lần sai được đi tiếp" lấy số ở đâu? → A: Theo `attempts_per_question` ([LEARNING_EXPERIENCE §4.2](../../docs/02-learning/LEARNING_EXPERIENCE.md) là nguồn duy nhất số lượt thử mỗi câu).

### Session 2026-11-20 (triển khai — giả định [Confirmed], chờ owner xác nhận)

- Cue "đã xử lý" lưu trong `assessment.video_progress.processed_cue_ids` khi chấm câu trả lời: đúng, hoặc sai đủ `attempts_per_question` với `allow_skip_after_wrong` (lượt thử lại không chấm điểm của engine không ghi đúng/sai vào item). Cue `allow_skip_after_wrong = false` sai đủ số lượt → không trả lời thêm được (`ANSWER_LIMIT_REACHED`) và chặn hoàn thành.
- Video không có câu chèn: điểm theo chiến lược `completion` — `max_score` của version (≥ 1) khi đạt quy tắc hoàn thành, 0 khi không (learning event yêu cầu `score.max > 0`).
- `ActivityResponseSubmission.response` không còn `required` trong contract: `branch_choice` gửi `branch_point_id` + `option_id`; kind câu hỏi thiếu `response` vẫn là `422 INVALID_RESPONSE` (nới ràng buộc request, không phá vỡ).
- Video nhánh: hoàn thành khi lựa chọn dẫn tới đoạn `is_ending` (+ cue bắt buộc), không xét `watched_ratio` của video chính.
- `assessment.integrity_signals` tạo ở F065, append-only, chưa partition.
- Biến thể: pipeline F027 thêm `ogv_480` (cạnh `ogv_720`); render chỉ trả biến thể `ogv_*` và phụ đề; phụ đề chọn biến thể `vtt`/`original`. Pipeline media nhận `text/vtt` từ follow-up F027 T040 (2026-11-20: biến thể `vtt`).
- `client_seq` client đặt bằng unix ms tăng dần (server giữ số lớn nhất, kể cả qua thiết bị/resume); heartbeat gửi toàn bộ `watched_ranges` nên mất một heartbeat không mất tiến độ (thay `PendingWrites` cho tiến độ).
- Q: Tua vượt phần đã xem xử lý thế nào? → A: Server cắt phần `watched_ranges` tăng nhanh hơn thời gian thực × tốc độ phát tối đa (integrity signal); với `allow_seek = watched_only`, `position_ms` vượt mép đã xem được chấp nhận → `422 VIDEO_SEEK_NOT_ALLOWED`, trạng thái không đổi ([research.md](research.md) R2).

### Session 2026-11-28 (dev seed cho T030/T041 — [Confirmed — owner "chốt hết"])

- Q: Dữ liệu cho T030/T041 (quickstart §1 nêu video 7:30, phụ đề vi/en, chương 3, bản 2 nhánh)? → A: [Confirmed — owner "chốt hết"] `make seed-media` ([LOCAL_DEVELOPMENT §5.2](../../docs/11-operations/LOCAL_DEVELOPMENT.md#52-seed-mở-rộng-media-golden-dataset-make-seed-media-make-seed-attempts)) nạp activity `video_lesson` "[DEV] Video bảng nhân (3:20)": clip **tổng hợp** 3:20 (640×360, testsrc2 + sine, sinh bằng ffmpeg trong container dùng một lần lúc seed, không commit) đi qua pipeline F027 thật (`ogv_480`, `ogv_720`, `mp4_720`, `hls_*`), 2 chương (0–1:40, 1:40–3:20), phụ đề `vi` (mặc định) + `en`, câu chèn bắt buộc `cue-0130` ở 01:30, `watched_only`, tốc độ 1/1,5, hoàn thành 90 % + cue. Đủ cho kịch bản 2–12 (thoát ở 03:12 < 3:20); thời lượng ngắn hơn 7:30 để test T030 (heartbeat theo thời gian thực) chạy ~3,5 phút. Bản 2 nhánh (kịch bản 13) **không seed** trong lần này (giữ phạm vi seed nhỏ) — kịch bản 13 tự tạo activity nhánh trong Content Studio hoặc chờ seed sau.
- Q: Ai được giao? → A: [Confirmed] Bài giao "[DEV] Xem video bảng nhân" cho `DEMOA1` khối 3 **trừ `hs.chi.3b`** (`exclude_student_ids` — "3A" cũ của quickstart, ADR-0095); `hs.an.3a` thấy và làm được; `hs.dung.7a` (kịch bản 12, `pgd-demo`) vẫn nhận `404` với attempt của người khác.
- Q: Nội dung platform hay tenant? → A: [Confirmed — owner "chốt hết"] **Tenant `th-demo-a`** (tác giả `qlcm.truong.a`, người duyệt `sa.admin`; `sa.admin` upload media cho tenant vì không user seed nào của tenant, kể cả `content_admin` ở platform, có `media.upload`): media platform chưa gán được cho tenant — `assignToTenants` của module learning chưa có handler `media` (`422 resource_type`), mà `MediaRepo.Assets` đọc theo HybridContent nên học sinh chỉ thấy media của tenant mình hoặc media platform đã gán → activity platform có media trả `VIDEO_NOT_READY` cho học sinh. Đây là **câu hỏi mở cho owner** (gán media platform cho tenant, hoặc tự gán media theo activity), không sửa trong seed.
- Q: Phát video thật ở local? → A: Chưa được: CDN local (`cdn` nginx → floci `FLOCI_SERVICES_S3_ENFORCE_AUTH=true`, không bucket policy — LOCAL_DEVELOPMENT §3.1) trả `403 AccessDenied` cho mọi object kể cả URL đã ký đúng; API attempt/heartbeat/cue không cần tải media. Kịch bản phát thật (T041 bước 3–4, 7) kiểm trên staging hoặc sau khi bổ sung đường đọc có ký cho CDN local [Proposed]. Mặc định khóa ký CDN của compose đã thống nhất `change-me-local` (trước đó `cdn` mặc định khác `core-api`).

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Server phục vụ và chấm bài học video (Priority: P1)

Nội dung video được kiểm khi lưu/publish; khi học sinh bắt đầu, server ghim version, trả payload không đáp án, nhận heartbeat tiến độ, chấm câu chèn và quyết định hoàn thành theo phần đã xem thật.

**Why this priority**: Server-authoritative là điều kiện để renderer có ý nghĩa; chặn báo khống hoàn thành.

**Independent Test**: Bằng API (Postman/integration test): start activity video có câu chèn 01:30, gửi heartbeat nhảy tới cuối → `watched_ratio` không tăng phần bất hợp lý, submit → không hoàn thành; xem thật + trả lời câu chèn → hoàn thành.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F065-AC-01 | KHI lưu Draft hoặc publish activity `video_lesson`, plugin PHẢI kiểm theo `video-lesson.schema.json` và quy tắc: `at_ms` < `duration_ms`; chương không chồng nhau, `start_ms < end_ms`; `question_version_id` của câu chèn đã publish và chấm tự động; `target_segment_id` tồn tại; NẾU sai, THÌ trả `422 ACTIVITY_CONFIG_INVALID` | FR-CNT-014 · [VIDEO_LEARNING §3](../../docs/02-learning/VIDEO_LEARNING.md) · ADR-0048 | TC-ACT-003 |
| F065-AC-02 | KHI học sinh bắt đầu attempt, module learning PHẢI ghim `activity_version_id` và `question_version_id` của mọi câu chèn, tạo item theo thứ tự `at_ms` và trả render payload gồm biến thể Theora 480p/720p, phụ đề WebVTT, chương, cue (`cue_id`, `at_ms`, `position`, `required`, `allow_skip_after_wrong`), nhánh (không có `target_segment_id`), `interaction`, `resume_position_ms`, `progress.watched_ranges` | FR-CNT-014 · FR-CNT-011 · [VIDEO_LEARNING §2–§5](../../docs/02-learning/VIDEO_LEARNING.md) | TC-VID-005 |
| F065-AC-03 | Render payload và mọi item câu chèn PHẢI không chứa `answer_key`, `correct_option_id(s)`, `explanation`, `feedback`, `hints`, `misconception_tag`; media riêng tenant PHẢI dùng signed URL | AC-EXR-001 · ADR-0016 · ADR-0044 · AGENTS R6, R31 | TC-ACT-001 |
| F065-AC-04 | NẾU asset video chưa có biến thể Theora sẵn sàng, THÌ start PHẢI trả `422 VIDEO_NOT_READY` và không tạo attempt | FR-CNT-014 · [VIDEO_LEARNING §12](../../docs/02-learning/VIDEO_LEARNING.md) | — |
| F065-AC-05 | KHI nhận `reportActivityProgress`, plugin PHẢI bỏ qua `client_seq` ≤ giá trị đã lưu (trả trạng thái hiện tại), gộp `watched_ranges` (≤ 64 khoảng), cắt phần tăng nhanh hơn thời gian thực × tốc độ tối đa trong `interaction.playback_rates` và ghi integrity signal, tính `watched_ratio` = tổng đã xem ÷ `duration_ms` | FR-CNT-014 · AC-CNT-006 · [VIDEO_LEARNING §4](../../docs/02-learning/VIDEO_LEARNING.md) | TC-VID-002 |
| F065-AC-06 | Ở NƠI `allow_seek = watched_only` và attempt chưa hoàn thành, NẾU `position_ms` vượt mép phần đã xem được chấp nhận, THÌ module learning PHẢI trả `422 VIDEO_SEEK_NOT_ALLOWED` và không đổi `video_progress` | FR-CNT-014 · [VIDEO_LEARNING §7, §12](../../docs/02-learning/VIDEO_LEARNING.md) | TC-VID-002 |
| F065-AC-07 | KHI `watched_ratio` vượt mốc 25/50/75/100%, module learning PHẢI ghi learning event `video.progressed` đúng một lần mỗi mốc mỗi attempt, không phát theo từng heartbeat | FR-CNT-014 · [VIDEO_LEARNING §10](../../docs/02-learning/VIDEO_LEARNING.md) | — |
| F065-AC-08 | KHI nhận `submitActivityResponse` `kind = cue_answer`, plugin PHẢI chấm server-side theo question version đã ghim, ghi `attempt_answers` (kèm `hints_used`, `response_ms`, `rapid_guess`), ghi `video.cue_answered` và `question.answered`, trả `ActivityResponseResult` theo FeedbackTiming | FR-CNT-014 · AC-EXR-002 · [VIDEO_LEARNING §5](../../docs/02-learning/VIDEO_LEARNING.md) | TC-VID-001 |
| F065-AC-09 | KHI câu chèn có `allow_skip_after_wrong = true` và học sinh đã sai đủ `attempts_per_question`, plugin PHẢI coi cue là đã xử lý (không chặn hoàn thành) và trả giải thích nếu FeedbackTiming cho phép | [VIDEO_LEARNING §5](../../docs/02-learning/VIDEO_LEARNING.md) · [LEARNING_EXPERIENCE §4.2](../../docs/02-learning/LEARNING_EXPERIENCE.md) | TC-VID-001 |
| F065-AC-10 | KHI `submitActivityAttempt`, NẾU `require_required_cues` và còn cue `required` chưa xử lý, THÌ PHẢI trả `422 CUE_ANSWER_REQUIRED`; ngược lại PHẢI tính `completed` = `watched_ratio` ≥ `min_watched_ratio` ∧ điều kiện cue, `passed` theo `min_score_ratio`, và chỉ ghi `activity.completed` khi `completed` | FR-CNT-014 · AC-CNT-006 · [VIDEO_LEARNING §7](../../docs/02-learning/VIDEO_LEARNING.md) · `video_completion` | TC-VID-002 |
| F065-AC-11 | Nộp attempt video PHẢI idempotent theo `Idempotency-Key` (cùng key → cùng kết quả; khác key → `409 ATTEMPT_ALREADY_SUBMITTED`) và có audit + event `activity.submitted` qua outbox của engine (F030) | AC-EXR-003 · AGENTS R10 · ADR-0042 | TC-LRN-008 |
| F065-AC-12 | KHI học sinh bắt đầu lại trong `attempt_resume_window`, module learning PHẢI trả attempt `in_progress` cũ với `resume_position_ms` và `watched_ranges` đã lưu, trên đúng version đã ghim | FR-EXR-004 · FR-CNT-011 · [ACTIVITY_ENGINE §6.3](../../docs/02-learning/ACTIVITY_ENGINE.md) | TC-VID-003 |
| F065-AC-13 | NẾU attempt thuộc học sinh khác hoặc tenant khác, THÌ mọi endpoint attempt PHẢI trả `404`; NẾU token không phải học sinh game, THÌ PHẢI trả `403 GAME_ACCESS_STUDENT_ONLY`; mọi truy vấn `video_progress` PHẢI lọc `tenant_id` + `deleted_at IS NULL` | AGENTS R7, R8, R23, R27 · ADR-0005 · ADR-0039 | TC-TEN-001, TC-AUTH-014 |
| F065-AC-14 | NẾU `cue_id`/`branch_point_id`/`option_id` > 64 ký tự, `watched_ranges` > 64 khoảng hoặc `playback_rate` ngoài [0.5, 2], THÌ PHẢI trả `422 VALIDATION_FAILED` | ADR-0041 · AGENTS R28 | TC-TEN-024 |

---

### User Story 2 — Học sinh xem video trong game (Priority: P1)

Học sinh mở bài video từ bài giao/quest: video Theora phát theo chất lượng thiết bị, phụ đề vi/en theo Theme, danh sách chương, không tua qua phần chưa xem, video dừng ở câu chèn, thoát rồi xem tiếp đúng chỗ.

**Why this priority**: Giá trị trực tiếp cho học sinh; exit M-C3 có định dạng video.

**Independent Test**: `hs.an.3a` mở video có câu chèn 01:30 trong Godot Android và Web (ADR-0101): dừng đúng mốc, chưa trả lời không phát tiếp; bật/tắt phụ đề vi/en; thoát ở 03:12, mở lại → "Xem tiếp từ 03:12?".

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F065-AC-15 | game client PHẢI đăng ký renderer `video_lesson` trong `ActivityRendererRegistry` và phát bằng `VideoStreamPlayer` biến thể 480p ở preset Thấp/Web, 720p ở preset Trung bình/Cao; KHI đệm > 3 s lặp lại 2 lần, PHẢI hạ 720p → 480p giữ vị trí | FR-CNT-014 · [VIDEO_LEARNING §2](../../docs/02-learning/VIDEO_LEARNING.md) · [STUDENT_GAME_UX_LEARNING §5.4](../../docs/05-ux/STUDENT_GAME_UX_LEARNING.md) | — |
| F065-AC-16 | game client PHẢI tự parse WebVTT, vẽ phụ đề bằng Label theo Theme (cỡ ≥ chữ cơ sở của khối, nền mờ), cho chọn vi/en/tắt, mặc định theo ngôn ngữ giao diện | FR-CNT-014 · AC-VID-004 trong [VIDEO_LEARNING §13](../../docs/02-learning/VIDEO_LEARNING.md) · ADR-0047 | TC-VID-004 |
| F065-AC-17 | TRONG KHI `allow_seek = watched_only` và chưa hoàn thành, thanh tua PHẢI không cho kéo qua mép phần đã xem và hiện toast `ui.video.seek_locked` | FR-CNT-014 · [STUDENT_GAME_UX_LEARNING §5.4](../../docs/05-ux/STUDENT_GAME_UX_LEARNING.md) | TC-VID-002 |
| F065-AC-18 | KHI vị trí phát đạt `at_ms` của cue, game client PHẢI tạm dừng, làm mờ khung video và mở overlay câu hỏi bằng renderer câu hỏi F063; cue `required` PHẢI không có nút bỏ qua; sau phản hồi PHẢI tiếp tục phát | FR-CNT-014 · [STUDENT_GAME_UX_LEARNING §5.3](../../docs/05-ux/STUDENT_GAME_UX_LEARNING.md) | TC-VID-001 |
| F065-AC-19 | TRONG KHI phát, game client PHẢI gửi heartbeat `reportActivityProgress` theo chu kỳ của `video_completion` và khi tạm dừng/thoát, với `client_seq` tăng dần, `position_ms`, `watched_ranges`, `playback_rate` | FR-CNT-014 · [VIDEO_LEARNING §4](../../docs/02-learning/VIDEO_LEARNING.md) | TC-VID-002 |
| F065-AC-20 | KHI attempt có `resume_position_ms` > 0, game client PHẢI hỏi "Xem tiếp từ mm:ss?" và tua tới vị trí đó nếu học sinh đồng ý | FR-EXR-004 · AC-VID-003 trong [VIDEO_LEARNING §13](../../docs/02-learning/VIDEO_LEARNING.md) | TC-VID-003 |
| F065-AC-21 | KHI video kết thúc mà `pending_cue_ids` còn cue bắt buộc, game client PHẢI tua về cue chưa xử lý đầu tiên; ngược lại PHẢI gọi `submitActivityAttempt` và mở màn kết quả F063 ("Hoàn thành!" + điểm câu chèn) | FR-CNT-014 · [STUDENT_GAME_UX_LEARNING §5.4](../../docs/05-ux/STUDENT_GAME_UX_LEARNING.md) | TC-VID-001 |
| F065-AC-22 | NẾU start trả `422 VIDEO_NOT_READY`, THÌ game client PHẢI hiện `ui.video.not_ready`; NẾU heartbeat trả `422 VIDEO_SEEK_NOT_ALLOWED`, THÌ PHẢI tua về mép đã xem | [VIDEO_LEARNING §12](../../docs/02-learning/VIDEO_LEARNING.md) | — |
| F065-AC-23 | KHI mở player video, game client PHẢI `suspend_world("video")` và giữ ≥ 30 FPS UI khi phát 720p trên thiết bị tầm trung (480p trên Web/preset Thấp) | NFR-COMP-001 · [STUDENT_GAME_UX_LEARNING §5.5](../../docs/05-ux/STUDENT_GAME_UX_LEARNING.md) | TC-PERF-020 |

---

### User Story 3 — Video tương tác có nhánh (Priority: P3)

Tại điểm rẽ nhánh, video dừng, học sinh chọn hướng; server ghi lựa chọn và trả đoạn tiếp theo; hoàn thành khi tới đoạn kết thúc.

**Why this priority**: Tính năng [Confirmed] nhưng chi tiết UX [Confirmed]; không chặn vòng School Loop.

**Independent Test**: Video 2 nhánh: chọn "vào nước" → phát đoạn `water`, tới `is_ending` → hoàn thành; lựa chọn có trong `video_branch_choices`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F065-AC-24 | KHI tới `at_ms` của điểm rẽ nhánh, game client PHẢI dừng và hiện `prompt` + lựa chọn; KHI chọn, PHẢI gửi `submitActivityResponse` `kind = branch_choice` (`branch_point_id`, `option_id`) và phát đoạn theo `next_segment_id` server trả | FR-CNT-014 · [VIDEO_LEARNING §6](../../docs/02-learning/VIDEO_LEARNING.md) | — |
| F065-AC-25 | KHI nhận `branch_choice`, plugin PHẢI ghi `video_branch_choices` (một lựa chọn mỗi điểm rẽ trong attempt, idempotent theo `Idempotency-Key`), không chấm đúng/sai, và chỉ tính hoàn thành khi đi tới đoạn `is_ending` | FR-CNT-014 · [VIDEO_LEARNING §6](../../docs/02-learning/VIDEO_LEARNING.md) | — |

### Edge Cases

- NẾU publish version video mới khi đang xem, THÌ học sinh xem hết version đã ghim (F065-AC-02, TC-VID-005).
- NẾU heartbeat đến lệch thứ tự, THÌ `client_seq` cũ bị bỏ qua (F065-AC-05).
- NẾU mất mạng khi phát, THÌ video tiếp tục (đã đệm), heartbeat gộp `watched_ranges` gửi khi có mạng qua `PendingWrites`; câu trả lời cue giữ trong hàng đợi.
- NẾU thiếu phụ đề ngôn ngữ đã chọn, THÌ dùng phụ đề `is_default` và nhãn `ui.learning.no_translation`.
- NẾU học sinh xem lại sau khi hoàn thành, THÌ tua tự do (`allow_seek` chỉ áp lần đầu — VIDEO_LEARNING §7).
- NẾU `duration_ms` = 0 hoặc media lỗi giải mã, THÌ hiện lỗi tải + telemetry, không crash.

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-CNT-014 | Plugin `video_lesson` (validate, render payload, progress, cue, nhánh, evaluate, evidence) và renderer Godot | [VIDEO_LEARNING](../../docs/02-learning/VIDEO_LEARNING.md), [ACTIVITY_ENGINE §5](../../docs/02-learning/ACTIVITY_ENGINE.md) |
| FR-EXR-004 | Resume vị trí xem trong resume window | [ACTIVITY_ENGINE §6.3](../../docs/02-learning/ACTIVITY_ENGINE.md) |
| FR-CNT-011 | Ghim activity version + question version câu chèn khi start | [CONTENT_MODEL](../../docs/02-learning/CONTENT_MODEL.md), ADR-0043 |

### Key Entities

- **Video lesson version**: video, thời lượng, chương, phụ đề, câu chèn, nhánh/đoạn, cách tua, quy tắc hoàn thành.
- **Video progress**: vị trí, khoảng đã xem (đã gộp), tỉ lệ đã xem thật, heartbeat cuối, số thứ tự heartbeat — một bản ghi mỗi attempt.
- **Cue item**: câu hỏi chèn là một item của attempt, có câu trả lời và evidence như câu hỏi thường.
- **Branch choice**: lựa chọn của học sinh tại một điểm rẽ nhánh.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `video_completion` | Mặc định quy tắc hoàn thành (khi version không đặt) và chu kỳ heartbeat |
| `attempts_per_question` | Ngưỡng sai cho `allow_skip_after_wrong` |
| `attempt_resume_window`, `attempt_idle_timeout` | Resume/pause attempt video |
| `quality_presets` | Chọn biến thể 480p/720p theo preset |
| `video_recommended_length` | Cảnh báo biên tập (không chặn) khi validate |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Hoàn thành và điểm do server; không tin tiến độ client | ADR-0016 · AGENTS R5 |
| Plugin trong module learning, Clean Architecture + CQRS, ent + Atlas, outbox | ADR-0074 · ADR-0038 · ADR-0039 · ADR-0042 · ADR-0048 |
| Media chỉ ở object storage, khóa `media/<sha256>/…`; signed URL cho nội dung riêng tenant | ADR-0044 · AGENTS R31 |
| Chỉ soft-delete; mọi bảng mới có `tenant_id`, `deleted_at`; chuỗi `varchar(n)` | AGENTS R21, R27, R28 · ADR-0041 |
| Phụ đề/UI theo Theme token; chuỗi qua `.po` | ADR-0047 · ADR-0056 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: 0 attempt video được tính hoàn thành khi phần đã xem thật < ngưỡng hoàn thành (kiểm bằng heartbeat giả mạo).
- **SC-002**: 100% câu chèn bắt buộc dừng video đúng mốc (± 1 khung hình phát) trên Android và Web (ADR-0101).
- **SC-003**: Học sinh thoát giữa chừng tiếp tục đúng vị trí (± 10 s) khi mở lại trong resume window.
- **SC-004**: Phát 720p ổn định ≥ 30 FPS trên thiết bị tầm trung tham chiếu.

## Ngoài phạm vi

- Upload/transcode video Theora/HLS, signed URL, media pipeline: F027. Editor video trong portal (form sinh từ JSON Schema, preview HLS): F054 (Workspace B) — F065 chỉ cung cấp plugin validate và `listActivityTypes`.
- Attempt engine chung, outbox, audit, review: F030; mastery từ evidence: F031. `audio_lesson`: F066. Video upload của học sinh: không có (VIDEO_LEARNING §1).
- Phân tích xem video trên dashboard: F043/F044.

## Assumptions

- F030 định nghĩa `activity.Plugin`/`Registry` (ACTIVITY_ENGINE §5), bảng `attempts`/`attempt_items`/`attempt_answers`, command/query handler và outbox; F027 cung cấp port lấy biến thể media + signed URL; F031 cung cấp port ghi learning event.
- Delta contract [Confirmed]: `$defs.render` trong `video-lesson.schema.json`; `ActivityRender.media[].variant/language/segment_id`; `ActivityProgressState.watched_ranges` (contracts §6).
- Nhánh tương tác chi tiết UX [Confirmed] (VIDEO_LEARNING §6).
