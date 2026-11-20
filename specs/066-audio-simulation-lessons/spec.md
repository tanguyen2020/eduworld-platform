# Feature Specification: F066 — Bài nghe và mô phỏng: plugin `audio_lesson`, `simulation` + checker Go, template Godot

**Feature Branch**: `066-audio-simulation-lessons` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: module `learning` + `src/game/` (+ editor `src/cms/frontend/apps/portal` workspace B) · **Mốc**: M4a · **Phụ thuộc**: F063 (và F030 attempt engine, F031 learning events/mastery, F065 cơ chế tiến độ media, F027 media, F058 Activity Builder)
**Input**: Hai loại hoạt động mới trong Activity Registry: `audio_lesson` — bài nghe hiểu/nghe chép có câu hỏi server tự chấm, giới hạn lượt nghe, resume; `simulation` — template Godot đóng gói sẵn, CMS chỉ tham số hóa, client ghi action log, server kiểm chứng xác định bằng checker Go. Kèm renderer Godot, template tham chiếu `physics.lever` v1 và editor tương ứng trong Learning Activity Builder. FR nguồn: FR-CNT-015 (chỉ phần nghe), FR-CNT-016 (kèm FR-CNT-013 Registry).

## Clarifications

### Session 2026-11-16

- Q: "Lượt nghe" (`max_plays`) do ai đếm? → A: Server đếm trong tiến độ media; client báo `play_started` khi phát từ đầu hoặc phát lại sau khi hết bài; vượt lượt → server từ chối, client không phát ([research.md](research.md) R1). Server không ngăn được phát cục bộ, nên số lượt server ghi là nguồn sự thật cho báo cáo.
- Q: Transcript bắt buộc? AUDIO_LANGUAGE_LEARNING §2 ghi `transcript_asset_id?`, ADMIN_PORTAL §3.13 (Audio Manager) ghi "Transcript bắt buộc cho bài nghe". → A: [Confirmed] Lưu Draft cho phép thiếu; publish bắt buộc có transcript (trợ năng khi phòng máy không có tai nghe — STUDENT_GAME_UX_LEARNING §6.3). Đã báo mâu thuẫn để chốt ở tài liệu sở hữu.
- Q: Render payload mô phỏng có `goals[].check` (metric/operator/value)? → A: Không. `check` là tham số kiểm chứng (ACTIVITY_ENGINE §5) — client chỉ nhận `goal_id` + `description` ([research.md](research.md) R3).
- Q: Server dùng tham số/seed client gửi? → A: Không. Checker dùng tham số của version đã ghim và seed server cấp lúc start; giá trị client gửi lệch → `422 SIMULATION_LOG_INVALID`.
- Q: Điểm phần mô phỏng lấy từ lần chạy nào? → A: Lần chạy `verified` cuối cùng trước submit (nhất quán ACTIVITY_ENGINE §7.2).
- Q: Template nào trong phạm vi? → A: Khung template + checker và một template tham chiếu `physics.lever` v1 (ví dụ của SIMULATION_ENGINE). Template khác là sản xuất nội dung, theo cùng khung.

### Session 2026-11-20 (triển khai — các điểm spec/plan chưa chốt, dùng mặc định [Proposed])

- Q: Câu hỏi bài nghe có phải "câu chèn bắt buộc" như video (chặn submit bằng `CUE_ANSWER_REQUIRED`)? → A: [Proposed] Không. Câu hỏi bài nghe là cue không bắt buộc của timeline dùng chung với video; câu chưa trả lời được 0 điểm, không chặn nộp; hoàn thành theo `min_watched_ratio` (mặc định `video_completion`). Câu không có `at_ms` hỏi sau khi nghe hết, xếp sau mọi câu có mốc, giữ thứ tự khai báo.
- Q: Tua trong bài nghe và tốc độ? → A: [Proposed] Tua tự do trong một lượt (lượt chỉ tính khi phát từ đầu/phát lại sau khi hết bài, research R1); tốc độ 0,75×/1× khi `allow_speed_change`, chỉ 1× khi tắt (STUDENT_GAME_UX_LEARNING §6). Tiến độ vẫn qua kiểm tra hợp lý của F065 (cắt phần nhanh hơn thời gian thực × tốc độ tối đa, integrity signal `audio_progress_implausible`).
- Q: Transcript trả về thế nào? → A: [Proposed] Là phần tử `ActivityRender.media[]` với `kind = transcript` (thêm giá trị enum), signed URL; có khi `show_transcript = always`, hoặc `after_complete` khi attempt đã `submitted`/`graded`; không bao giờ khi `never`.
- Q: Asset audio chưa sẵn sàng trả mã nào? (API_DESIGN §4.1 ghi `VIDEO_NOT_READY` cho "video/audio", spec F066-AC-03 ghi `MEDIA_NOT_READY`) → A: [Proposed] Theo spec: `422 MEDIA_NOT_READY` (thiếu biến thể `ogg`); video giữ `VIDEO_NOT_READY`.
- Q: Danh mục template lưu ở đâu? (research R2: bảng `content.simulation_templates` + job `simtemplates.Sync`) → A: [Proposed] Registry nhúng trong mã module learning (`internal/domain/activity/simulation/templates/<code>/v<n>/descriptor.json`, `go:embed`), trạng thái `active`/`retired` khai báo trong descriptor; `listSimulationTemplates` và kiểm cấu hình đọc thẳng registry. Bảng `content.simulation_templates` và job Sync hoãn tới khi có consumer đọc DB (không ảnh hưởng AC; DATABASE_DESIGN §2.6a ghi chú).
- Q: Template version nào được ghim? (attempt không có cột lưu template version) → A: [Proposed] Version `active` cao nhất ≥ `min_template_version` của nội dung, xác định từ registry trong bản phát hành; khi một template có version thứ hai sẽ thêm cột ghim (expand) — hiện chỉ có `physics.lever` v1.
- Q: `parameters` của `physics.lever` gồm mảng `masses_kg[]` (research R4) nhưng `simulation.schema.json` chỉ cho giá trị vô hướng? → A: [Proposed] Dùng tham số vô hướng `mass_1_kg`…`mass_4_kg` (vật 1–2 bắt buộc), `arm_length_m`, `slots_per_side`; `g = 9,8` trong descriptor.
- Q: Run bị từ chối có tính vào `max_runs`? Thiếu `max_runs`? → A: [Proposed] Mọi run đã lưu (kể cả `rejected`) tính lượt — giới hạn số dòng; thiếu `max_runs` dùng trần 50 của schema.
- Q: Điểm phần mô phỏng? → A: [Proposed] Mỗi mục tiêu đạt ở run `verified` cuối = 1 điểm, cộng điểm câu hỏi kết luận; hoàn thành (`goals_met`) khi run `verified` cuối đạt mọi mục tiêu; template `checker_available = false`: hoàn thành khi có ít nhất một run `not_verifiable`, không điểm/evidence từ mô phỏng.
- Q: Editor portal (US4) phụ thuộc F058 (editor registry của Activity Builder) đang `Ready`, chưa có mã? → A: Không làm US4 trong lần này (R34: không tự dựng Activity Builder); T044, T045, T052, T053 để mở, chờ F058.

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Server phục vụ và chấm bài nghe (Priority: P1) 🎯 MVP

Biên tập viên lưu bài nghe; học sinh bắt đầu, server ghim version, trả payload không đáp án, đếm lượt nghe, chấm câu nghe hiểu/nghe chép và tính hoàn thành.

**Why this priority**: Bài nghe dùng lại gần như toàn bộ cơ chế video (F065) nên là phần tăng trưởng nhỏ nhất có giá trị; server-authoritative là điều kiện cho renderer.

**Independent Test**: Bằng API: tạo bài nghe `max_plays = 2`, start, báo `play_started` 3 lần → lần 3 bị từ chối; trả lời câu `fill_in_blank` có dấu/khoảng trắng thừa → chấm đúng theo chuẩn hóa; submit → hoàn thành.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F066-AC-01 | KHI lưu Draft hoặc publish activity `audio_lesson`, plugin PHẢI kiểm theo `audio-lesson.schema.json` và quy tắc: đoạn (`segments`) `start_ms < end_ms` ≤ thời lượng audio, không chồng nhau; `at_ms` < thời lượng; mọi `question_version_id` đã publish và thuộc loại tự chấm; KHI publish PHẢI có `transcript_asset_id`; NẾU sai, THÌ trả `422 ACTIVITY_CONFIG_INVALID` | FR-CNT-015 · [AUDIO_LANGUAGE_LEARNING §2](../../docs/02-learning/AUDIO_LANGUAGE_LEARNING.md) · [ADMIN_PORTAL §3.13](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-ACT-003 |
| F066-AC-02 | KHI học sinh bắt đầu attempt `audio_lesson`, module learning PHẢI ghim `activity_version_id` và `question_version_id` của mọi câu, tạo item theo thứ tự (`at_ms` rồi thứ tự khai báo) và trả render payload gồm biến thể Ogg Vorbis (signed URL nếu riêng tenant), đoạn, câu (`position`, `at_ms`), `interaction` (`max_plays`, `allow_speed_change`, `show_transcript`), `plays_used`, `resume_position_ms`; transcript chỉ có khi `show_transcript = always` | FR-CNT-015 · FR-CNT-011 · [AUDIO_LANGUAGE_LEARNING §2](../../docs/02-learning/AUDIO_LANGUAGE_LEARNING.md) · ADR-0044 | TC-ACT-005 |
| F066-AC-03 | Render payload và mọi item PHẢI không chứa `answer_key`, `accepted_answers`, `correct_option_id(s)`, `explanation`, `feedback`, `hints`; NẾU asset audio chưa có biến thể sẵn sàng, THÌ start PHẢI trả `422 MEDIA_NOT_READY` và không tạo attempt | AC-EXR-001 · ADR-0016 · AGENTS R6 | TC-ACT-001 |
| F066-AC-04 | KHI nhận `reportActivityProgress` có `play_started = true` mà `plays_used` đã bằng `max_plays`, THÌ module learning PHẢI trả `422 AUDIO_PLAY_LIMIT_REACHED` và không đổi tiến độ; ngược lại PHẢI tăng `plays_used` đúng một lần cho mỗi `client_seq` mới và áp dụng quy tắc tiến độ/hợp lý của cơ chế media chung (gộp khoảng đã nghe, cắt phần nhanh hơn thời gian thực × tốc độ tối đa, integrity signal) | FR-CNT-015 · AC-AUD-001 trong [AUDIO_LANGUAGE_LEARNING §5](../../docs/02-learning/AUDIO_LANGUAGE_LEARNING.md) · [VIDEO_LEARNING §4](../../docs/02-learning/VIDEO_LEARNING.md) | TC-AUD-001 |
| F066-AC-05 | KHI nhận `submitActivityResponse` cho câu nghe hiểu/nghe chép, plugin PHẢI chấm server-side theo question version đã ghim (nghe chép `fill_in_blank`/`short_answer` chuẩn hóa theo QUESTION_BANK §4), ghi `attempt_answers` kèm `hints_used`, `response_ms`, `rapid_guess`, ghi learning event `question.answered`, trả `ActivityResponseResult` theo `FeedbackTiming`; câu trả lời vẫn được nhận khi đã hết lượt nghe | FR-CNT-015 · AC-AUD-002 trong [AUDIO_LANGUAGE_LEARNING §5](../../docs/02-learning/AUDIO_LANGUAGE_LEARNING.md) · [QUESTION_BANK §4](../../docs/02-learning/QUESTION_BANK.md) | TC-LRN-003 |
| F066-AC-06 | KHI `submitActivityAttempt` cho `audio_lesson`, plugin PHẢI tính `completed` theo `completion_rule` của version (thiếu thì theo `video_completion`), `passed` theo `min_score_ratio`, ghi `activity.completed` chỉ khi `completed`; nộp PHẢI idempotent theo `Idempotency-Key` (khác key → `409 ATTEMPT_ALREADY_SUBMITTED`) và có audit + outbox `activity.submitted` của engine | FR-CNT-015 · AC-EXR-003 · AGENTS R10 | TC-LRN-008 |
| F066-AC-07 | KHI học sinh bắt đầu lại trong `attempt_resume_window`, module learning PHẢI trả attempt `in_progress` cũ trên đúng version đã ghim với `resume_position_ms`, `plays_used` và câu đã trả lời giữ nguyên | FR-EXR-004 · AC-AUD-004 trong [AUDIO_LANGUAGE_LEARNING §5](../../docs/02-learning/AUDIO_LANGUAGE_LEARNING.md) | TC-LRN-009 |
| F066-AC-08 | HỆ THỐNG PHẢI không có endpoint upload ghi âm/giọng nói hay phản hồi phát âm; KHI lưu hoặc publish activity `speaking`, module learning PHẢI trả `422 ACTIVITY_TYPE_NOT_SUPPORTED` và `listActivityTypes` PHẢI không trả `speaking` | FR-CNT-015 · AC-AUD-003 · ADR-0059 · ADR-0072 | TC-AUD-004 |

---

### User Story 2 — Server kiểm chứng mô phỏng bằng checker Go (Priority: P1)

Học sinh cân bằng đòn bẩy; client gửi action log; server tính lại bằng checker Go từ tham số đã ghim + seed + action log, ghi evidence cho LO khi đạt; client báo khống bị bác.

**Why this priority**: Đây là lõi chống gian lận của FR-CNT-016 (AC-CNT-007); renderer mô phỏng vô nghĩa nếu chưa có.

**Independent Test**: Bằng API: start activity `physics.lever` v1, gửi action log cân bằng → `verified`, `goal_met = true`, learning event `simulation.goal_met`; gửi log không cân bằng kèm `client_outcome` "đạt" → `goal_met = false`, integrity signal.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F066-AC-09 | KHI lưu Draft hoặc publish activity `simulation`, plugin PHẢI kiểm theo `simulation.schema.json`, kiểm `template_code` + `min_template_version` có trong danh mục template `active` và `parameters` hợp lệ theo `parameter_schema` của template; NẾU có trường ngoài schema (kể cả trường script/code) hoặc tham số ngoài khoảng, THÌ trả `422 ACTIVITY_CONFIG_INVALID` | FR-CNT-016 · AC-SIM-003 trong [SIMULATION_ENGINE §9](../../docs/02-learning/SIMULATION_ENGINE.md) · ADR-0052 | TC-SIM-004 |
| F066-AC-10 | KHI học sinh bắt đầu attempt `simulation`, module learning PHẢI ghim version, cấp `seed` (`fixed` → seed của version; `per_attempt` → seed mới lưu trong attempt) và trả render payload gồm `template_code`, `template_version`, `parameters`, `goals[]` chỉ `{goal_id, description}`, `steps`, `interaction`, `seed`, `runs_used`, câu hỏi không đáp án; PHẢI không chứa `goals[].check` | FR-CNT-016 · [SIMULATION_ENGINE §3–§4](../../docs/02-learning/SIMULATION_ENGINE.md) · AGENTS R6 | TC-ACT-001 |
| F066-AC-11 | KHI nhận `submitSimulationRun`, module learning PHẢI chạy checker của `template_code`/`template_version` với tham số đã ghim, seed đã cấp và `action_log`, lưu run với `verified_outcome`, `goals_met`, `verification_status = verified`, ghi learning event `simulation.goal_met` cho mỗi mục tiêu đạt, phát `simulation.run_verified` qua outbox và trả `SimulationRunResult` | FR-CNT-016 · AC-SIM-001 · [SIMULATION_ENGINE §5](../../docs/02-learning/SIMULATION_ENGINE.md) | TC-SIM-001 |
| F066-AC-12 | NẾU `client_outcome` khác `verified_outcome`, THÌ module learning PHẢI trả kết quả theo `verified_outcome` (không đạt nếu checker nói không đạt) và ghi integrity signal `simulation_outcome_mismatch`, không tự phạt | FR-CNT-016 · AC-CNT-007 · ADR-0016 · [ANTI_CHEAT §6](../../docs/10-security/ANTI_CHEAT.md) | TC-SIM-002 |
| F066-AC-13 | NẾU `action_log` > 500 hành động hoặc > 64 KB, có `t_ms` giảm dần/âm, hành động không có trong template, đối số ngoài phạm vi, hoặc `template_code`/`template_version`/`seed`/`parameters` khác giá trị đã ghim, THÌ module learning PHẢI trả `422 SIMULATION_LOG_INVALID`, lưu run `rejected`, phát `simulation.run_rejected` và ghi integrity signal `simulation_log_invalid` | FR-CNT-016 · [SIMULATION_ENGINE §5](../../docs/02-learning/SIMULATION_ENGINE.md) | TC-SIM-002 |
| F066-AC-14 | NẾU số run của attempt đã bằng `interaction.max_runs`, THÌ `submitSimulationRun` PHẢI trả `422 SIMULATION_RUN_LIMIT_REACHED`; `submitSimulationRun` PHẢI idempotent theo `Idempotency-Key` (cùng key → cùng run, không tính thêm lượt) | FR-CNT-016 · AGENTS R10 · [SIMULATION_ENGINE §4, §8](../../docs/02-learning/SIMULATION_ENGINE.md) | TC-LRN-008 |
| F066-AC-15 | Ở NƠI template có `checker_available = false`, module learning PHẢI lưu run `not_verifiable`, tính hoàn thành theo các bước đã làm, chỉ chấm câu hỏi kèm theo và PHẢI không ghi evidence mastery từ kết quả mô phỏng | FR-CNT-016 · [SIMULATION_ENGINE §2](../../docs/02-learning/SIMULATION_ENGINE.md) mục 5 | — |
| F066-AC-16 | KHI `submitActivityAttempt` cho `simulation`, plugin PHẢI tính điểm = câu hỏi (`kind = simulation_question`, chấm `auto`) + phần `deterministic_check` theo lần run `verified` cuối cùng, `completed` theo `completion_rule` (`goals_met`), ghi evidence cho `objective_ids` khi đạt mục tiêu, idempotent + audit + outbox như F066-AC-06 | FR-CNT-016 · [ACTIVITY_ENGINE §7](../../docs/02-learning/ACTIVITY_ENGINE.md) | TC-SIM-001 |
| F066-AC-17 | Mỗi checker PHẢI cho kết quả giống hệt bộ vector kiểm thử chung của template (tham số + seed + action log → kết quả mong đợi) và p95 thời gian kiểm chứng PHẢI trong mục tiêu của [SIMULATION_ENGINE §6](../../docs/02-learning/SIMULATION_ENGINE.md) | FR-CNT-016 · AC-SIM-004 | TC-SIM-003 |

---

### User Story 3 — Học sinh nghe bài và làm mô phỏng trong game (Priority: P1)

Học sinh mở bài nghe: thấy lượt nghe còn lại, chỉnh tốc độ, transcript theo cấu hình, trả lời bằng renderer câu hỏi F063. Học sinh mở mô phỏng đòn bẩy: template nạp trong ≤ 3 s, thao tác, bấm "Thả", chỉ thấy "Đạt mục tiêu!" sau khi server xác nhận, rồi trả lời câu kết luận.

**Why this priority**: Giá trị trực tiếp cho học sinh; exit M4a có bài nghe/mô phỏng.

**Independent Test**: `hs.an.3a` trên Godot Windows và Web: bài nghe `max_plays = 2` → sau lượt 2 nút phát mờ; mô phỏng đòn bẩy cân bằng → "Đạt mục tiêu!" sau phản hồi server; tắt mạng giữa chừng → run được gửi lại cùng key.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F066-AC-18 | game client PHẢI đăng ký renderer `audio_lesson` trong `ActivityRendererRegistry`, phát bằng `AudioStreamPlayer` (Ogg Vorbis), tắt nhạc nền khi phát, hiện "Lượt nghe: n/max", tốc độ 0.75×/1× khi `allow_speed_change`, transcript theo `show_transcript`, câu hỏi qua renderer câu hỏi F063 | FR-CNT-015 · [STUDENT_GAME_UX_LEARNING §6](../../docs/05-ux/STUDENT_GAME_UX_LEARNING.md) | TC-AUD-001 |
| F066-AC-19 | KHI học sinh bấm phát từ đầu hoặc phát lại sau khi hết bài, game client PHẢI gửi `reportActivityProgress` `play_started = true` và chỉ phát sau khi server chấp nhận; NẾU trả `422 AUDIO_PLAY_LIMIT_REACHED` hoặc `plays_used = max_plays`, THÌ nút phát PHẢI mờ với `ui.audio.plays_exhausted` và vẫn cho trả lời | FR-CNT-015 · AC-UX-216 trong [STUDENT_GAME_UX_LEARNING §12](../../docs/05-ux/STUDENT_GAME_UX_LEARNING.md) | TC-AUD-001 |
| F066-AC-20 | Màn học tập của game client PHẢI không có nút ghi âm, không xin quyền micro và không gửi dữ liệu âm thanh lên server | FR-CNT-015 · ADR-0059 · ADR-0072 | TC-AUD-004 |
| F066-AC-21 | game client PHẢI đăng ký renderer `simulation`, `SimulationHost` PHẢI nạp template theo `template_code`/`template_version` (trong build hoặc gói PCK có `sha256` khớp manifest), áp `parameters` + `seed`, `suspend_world("simulation")`; NẾU thiếu template hoặc phiên bản thấp hơn `min_template_version`, THÌ PHẢI hiện `ui.activity_player.update_required` và gửi telemetry, không crash | FR-CNT-016 · FR-EXR-015 · [GODOT_ARCHITECTURE §6.2, §8](../../docs/08-game-client/GODOT_ARCHITECTURE.md) | TC-ACT-004 |
| F066-AC-22 | TRONG KHI học sinh thao tác, game client PHẢI ghi action log (`t_ms` tương đối, `action`, `args`) theo danh sách hành động của template, không vượt giới hạn của SIMULATION_ENGINE §5, và gửi `submitSimulationRun` với `Idempotency-Key` giữ nguyên khi thử lại (`PendingWrites`) | FR-CNT-016 · [SIMULATION_ENGINE §3](../../docs/02-learning/SIMULATION_ENGINE.md) | TC-SIM-003 |
| F066-AC-23 | game client PHẢI chỉ hiện `ui.simulation.goal_reached` và tick mục tiêu theo `SimulationRunResult` server trả; NẾU `422 SIMULATION_LOG_INVALID`, THÌ hiện `ui.simulation.log_invalid` + nút làm lại; NẾU `422 SIMULATION_RUN_LIMIT_REACHED`, THÌ khóa nút chạy lại và mở câu hỏi kết luận | FR-CNT-016 · AC-UX-210 trong [STUDENT_GAME_UX_LEARNING §12](../../docs/05-ux/STUDENT_GAME_UX_LEARNING.md) | TC-SIM-002 |
| F066-AC-24 | Template `physics.lever` v1 PHẢI dùng cùng mô hình toán đơn giản hóa với checker (physics engine chỉ để hiển thị), cho kết quả khớp bộ vector chung trong GdUnit4, và đạt mục tiêu FPS/kích thước gói/thời gian nạp của SIMULATION_ENGINE §6 trên Web export (bản giản lược ở preset Thấp) | FR-CNT-016 · AC-SIM-004 · AC-SIM-005 · ADR-0057 | TC-SIM-005 |

---

### User Story 4 — Biên tập viên soạn bài nghe và cấu hình mô phỏng (Priority: P2)

`cm.noidung` mở Learning Activity Builder, chọn `audio_lesson`: chọn audio + transcript, chia đoạn, chèn câu hỏi tự chấm theo mốc, đặt lượt nghe. Chọn `simulation`: chọn template từ danh mục, form tham số sinh từ `parameter_schema`, mục tiêu, bước, câu hỏi kết luận.

**Why this priority**: Không có editor thì content team phải nhập JSON; nhưng backend + client kiểm được độc lập bằng fixture nên P2.

**Independent Test**: Tạo bài nghe và mô phỏng qua portal tới Draft hợp lệ; thử nhập tham số ngoài khoảng → bị chặn ở form và server; không có ô script.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F066-AC-25 | KHI người dùng có `learning_content.activity.write` chọn `audio_lesson` trong Activity Builder, Portal PHẢI hiển thị editor gồm chọn audio/transcript từ thư viện media (`listMediaAssets`, upload cần `media.upload`), đoạn, câu hỏi theo mốc thời gian (chọn từ Question Bank, chỉ loại tự chấm), `max_plays`, `allow_speed_change`, `show_transcript`, kiểm client theo `audio-lesson.schema.json` và hiển thị `ACTIVITY_CONFIG_INVALID` theo trường | FR-CNT-015 · [ADMIN_PORTAL §3.13](../../docs/04-admin-portal/ADMIN_PORTAL.md) Audio Manager | — |
| F066-AC-26 | KHI chọn `simulation`, Portal PHẢI lấy template qua `listSimulationTemplates`, sinh form tham số từ `parameter_schema` (không có ô script/code), soạn `goals` (metric chọn từ danh sách metric của template), `steps`, câu hỏi, `seed_policy`, `max_runs`; NẾU người dùng không có `learning_content.activity.write`, THÌ Portal PHẢI không hiển thị nút tạo/sửa và server PHẢI trả `403` | FR-CNT-016 · AC-SIM-003 · [ADMIN_PORTAL §3.13](../../docs/04-admin-portal/ADMIN_PORTAL.md) Simulation Configuration · AGENTS R7 | TC-SIM-004 |

---

### User Story 5 — Bảo vệ, cô lập tenant, giới hạn input (Priority: P1)

**Why this priority**: Constitution điều III, IV, V.

**Independent Test**: Học sinh tenant B gọi attempt/run của tenant A; nhân sự gọi route người chơi; gửi chuỗi quá dài.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F066-AC-27 | NẾU attempt thuộc học sinh khác hoặc tenant khác, THÌ `startActivityAttempt` (resume), `getActivityAttempt`, `reportActivityProgress`, `submitActivityResponse`, `submitSimulationRun`, `submitActivityAttempt` PHẢI trả `404`; NẾU token không phải học sinh game, THÌ PHẢI trả `403 GAME_ACCESS_STUDENT_ONLY`; mọi truy vấn `simulation_runs`, `video_progress` PHẢI lọc `tenant_id` + `deleted_at IS NULL` | AGENTS R7, R8, R23, R27 · ADR-0005 · ADR-0039 | TC-TEN-001, TC-AUTH-014 |
| F066-AC-28 | NẾU `action` > 64 ký tự, `template_code` > 64 ký tự, `args` > 20 thuộc tính, `parameters`/`client_outcome` > 50 thuộc tính, hoặc `title` đoạn vượt `maxLength` của schema, THÌ PHẢI trả `422 VALIDATION_FAILED` | ADR-0041 · AGENTS R28 | TC-TEN-024 |
| F066-AC-29 | KHI menu/action của feature (`menu.content_studio.activities`, `menu.content_studio.simulations` và action con) đang tắt cho phạm vi phiên (`disabled_menus`/`disabled_actions` của `getMyPermissions`), Portal PHẢI ẩn menu và ẩn/khóa nút tương ứng (`<ActionGate actionKey>`); NẾU API trả `403 FEATURE_DISABLED`, Portal PHẢI hiện thông báo tính năng tạm tắt và tải lại quyền | FR-CMS-018 · ADR-0090 | TC-CMS-MENU-02 |

### Edge Cases

- NẾU publish version mới khi học sinh đang làm, THÌ học sinh hoàn thành trên version đã ghim, kể cả template/tham số mô phỏng (F066-AC-02, F066-AC-10, TC-ACT-005).
- NẾU `max_plays` không đặt, THÌ không giới hạn lượt và client không hiện bộ đếm lượt.
- NẾU mất mạng khi đang nghe, THÌ audio đã tải tiếp tục phát trong lượt hiện tại; lượt mới chỉ bắt đầu khi server chấp nhận `play_started` (F066-AC-19).
- NẾU heartbeat/run đến lệch thứ tự hoặc gửi lại, THÌ `client_seq` cũ bị bỏ qua, run cùng `Idempotency-Key` trả kết quả cũ (F066-AC-04, F066-AC-14).
- NẾU template bị chuyển `retired` trong danh mục, THÌ attempt đang chạy vẫn kiểm chứng được bằng checker của version đã ghim; lưu Draft mới với template đó bị từ chối (F066-AC-09).
- NẾU thiết bị ở preset Thấp/Web, THÌ dùng bản giản lược của template; tốc độ ×2 tắt nếu FPS < 30 ([STUDENT_GAME_UX_LEARNING §8](../../docs/05-ux/STUDENT_GAME_UX_LEARNING.md)).
- NẾU checker lỗi nội bộ (panic/timeout), THÌ trả `500`/`503` theo Problem Details, không lưu run `verified`, không tính lượt.

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-CNT-015 | Plugin `audio_lesson` (chỉ nghe hiểu/nghe chép tự chấm), renderer Godot, editor bài nghe; không có đường thu giọng nói | [AUDIO_LANGUAGE_LEARNING](../../docs/02-learning/AUDIO_LANGUAGE_LEARNING.md) |
| FR-CNT-016 | Plugin `simulation`, khung checker Go + template `physics.lever` v1, danh mục template, `SimulationHost` Godot, editor cấu hình mô phỏng | [SIMULATION_ENGINE](../../docs/02-learning/SIMULATION_ENGINE.md), ADR-0052 |
| FR-CNT-013 | Đăng ký hai plugin vào Activity Registry, không sửa quest/assignment/exam | [ACTIVITY_ENGINE §3, §5, §12](../../docs/02-learning/ACTIVITY_ENGINE.md), ADR-0048 |
| FR-EXR-004 | Resume bài nghe (vị trí, lượt nghe, câu đã trả lời) | [ACTIVITY_ENGINE §6.3](../../docs/02-learning/ACTIVITY_ENGINE.md) |
| FR-EXR-015 | `min_client_version`/`min_template_version` → màn cần cập nhật | [ACTIVITY_ENGINE §12](../../docs/02-learning/ACTIVITY_ENGINE.md) |
| FR-CMS-018 | Menu/action của feature theo danh mục `specs/portal/menu-catalog.yaml`, ẩn khi tắt, xử lý `403 FEATURE_DISABLED` | [ADMIN_PORTAL §2.7](../../docs/04-admin-portal/ADMIN_PORTAL.md) |

### Key Entities

- **Audio lesson version**: audio, transcript, đoạn, câu hỏi theo mốc, lượt nghe tối đa, tốc độ, cách hiện transcript, quy tắc hoàn thành.
- **Media progress**: tiến độ nghe một attempt (vị trí, khoảng đã nghe, tỉ lệ, số lượt nghe) — dùng chung với video.
- **Simulation template**: mô phỏng đóng gói sẵn có mã, phiên bản, loại, schema tham số, danh sách hành động/metric, có checker hay không, trạng thái.
- **Simulation version**: template + tham số + mục tiêu (có điều kiện kiểm chứng, không gửi client) + bước + câu hỏi + chính sách seed + số lần chạy.
- **Simulation run**: một lần chạy của học sinh: seed, action log, kết quả client tự báo, kết quả đã kiểm chứng, mục tiêu đạt, trạng thái kiểm chứng.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `video_completion` | Quy tắc hoàn thành mặc định và chu kỳ heartbeat của bài nghe (cơ chế media chung) |
| `attempt_resume_window`, `attempt_idle_timeout` | Resume/pause attempt bài nghe và mô phỏng |
| `rapid_guess_min_response_ms` | Cờ `rapid_guess` cho câu nghe hiểu và câu kết luận |
| `attempts_per_question` | Số lượt thử mỗi câu |
| `quality_presets` | Chọn bản đầy đủ/giản lược của template |

Giới hạn action log (≤ 500 hành động, ≤ 64 KB) và mục tiêu hiệu năng template nằm ở [SIMULATION_ENGINE §5–§6](../../docs/02-learning/SIMULATION_ENGINE.md) (chưa đăng ký CONFIG_DEFAULTS — task T049).

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Điểm, hoàn thành, mục tiêu mô phỏng do server; không tin `client_outcome` | ADR-0016 · ADR-0052 · AGENTS R5 |
| CMS chỉ tham số hóa template, không nạp script/code | ADR-0052 |
| Không ghi âm, không AI giọng nói trong Release | ADR-0059 · ADR-0072 |
| Plugin trong module learning, Clean Architecture + CQRS, ent + Atlas, outbox | ADR-0074 · ADR-0038 · ADR-0039 · ADR-0042 · ADR-0048 |
| Media chỉ ở object storage, signed URL cho nội dung riêng tenant | ADR-0044 · AGENTS R31 |
| Chỉ soft-delete; bảng mới có `tenant_id`, `deleted_at`; chuỗi `varchar(n)` | AGENTS R21, R27, R28 · ADR-0041 |
| Mô phỏng chạy trên thiết bị chuẩn và Web export | ADR-0013 · ADR-0057 |
| UI theo token: game theo `game-tokens`, portal theo Admin/Ops UI | AGENTS R32 · ADR-0047 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: 0 run mô phỏng được tính đạt khi action log không đạt mục tiêu (kiểm bằng bộ log giả mạo).
- **SC-002**: 100% vector kiểm thử chung của mỗi template cho cùng kết quả ở server và client.
- **SC-003**: 0 lượt nghe vượt `max_plays` được server ghi nhận; học sinh vẫn trả lời được sau khi hết lượt.
- **SC-004**: Template tham chiếu nạp ≤ 3 s (đã cache ≤ 1 s) và chạy ≥ 30 FPS trên thiết bị tầm trung và Web.
- **SC-005**: Biên tập viên tạo một bài nghe 4 câu hoặc một mô phỏng đòn bẩy tới Draft hợp lệ trong ≤ 10 phút mà không nhập JSON.

## Ngoài phạm vi

- Bài nói (`speaking`), ghi âm, chấm phát âm, AI giọng nói: ngoài Release (ADR-0072, ADR-0082) — không thiết kế.
- Lồng tiếng đề bài/hướng dẫn `g1_3` (Audio Manager phần lồng tiếng): F025; upload/transcode media Ogg Vorbis: F027.
- Attempt engine chung, outbox, audit, `activity.Plugin`/Registry: F030; mastery từ evidence: F031; cơ chế tiến độ media và `video_progress`: F065.
- Khung Activity Builder (chọn loại, metadata, preview, publish): F058 — F066 chỉ cắm editor `audio_lesson`/`simulation` vào editor registry; workflow duyệt/publish: F025/F026/F054.
- Template mô phỏng ngoài `physics.lever` v1 (nước, quang hợp, hình khối…): sản xuất nội dung theo cùng khung, mỗi template một PR (checker + scene + vector).
- Phân phối PCK qua CDN và ký gói ở pipeline build: F133 (F066 chỉ kiểm `sha256` theo manifest).
- Quest tham chiếu activity mới (TC-ACT-002): F072.

## Assumptions

- F030 cung cấp `activity.Plugin`/`Registry`, bảng `attempts`/`attempt_items`/`attempt_answers`, command/query handler, outbox, Problem codes; F065 cung cấp domain `ranges`/`plausibility` và repository `video_progress` dùng lại cho audio; F031 cung cấp port ghi learning event và evidence; F063 cung cấp `ActivityRendererBase`, renderer câu hỏi, `PendingWrites`; F058 cung cấp editor registry của portal.
- Delta contract [Confirmed] ở [contracts/README.md](contracts/README.md) §6: `play_started`/`plays_used`, `AUDIO_PLAY_LIMIT_REACHED`, `SIMULATION_RUN_LIMIT_REACHED`, `$defs.render` cho hai schema, `SimulationRunResult.goals_met[]`/`verification_status`, `SimulationTemplate.actions/metrics/status`, event payload `simulation.run_verified`/`run_rejected`.
- [Confirmed] Danh mục template được đồng bộ từ descriptor trong mã nguồn module learning lúc triển khai (research R2), không có API ghi.
- ADR-0048, ADR-0052 đang Proposed nhưng được liệt kê là quyết định hiện hành ở chỉ mục ADR.
