# Learning Progress & Analytics

> Trạng thái: Draft · Phiên bản: 0.3 · Owner: EdTech Specialist + Backend Architect · Cập nhật: 2026-11-16
> Liên quan: [ADR-0017](../adr/ADR-0017-mastery-model-co-the-cau-hinh-mac-dinh-weighted-evidence.md) · [ADR-0018](../adr/ADR-0018-glicko-2-cho-game-rating-tach-biet-learning-mastery.md) · [ADR-0072](../adr/ADR-0072-bo-vai-tro-giao-vien-hoc-sinh-tu-hoc-kiem-duyet-tap-trung.md) · [EVENT_DESIGN](../07-backend/EVENT_DESIGN.md) · [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md) · [LEARNING_EFFICACY_PLAN](LEARNING_EFFICACY_PLAN.md) · [ASSESSMENT_REGULATION_PROFILE](ASSESSMENT_REGULATION_PROFILE.md) · [DATA_PRIVACY](../10-security/DATA_PRIVACY.md) · [DOMAIN_MODEL §10](../06-architecture/DOMAIN_MODEL.md#10-phân-biệt-năm-loại-số-liệu-bắt-buộc)

## 1. Phạm vi

- Module `learning_record` (schema `learning`): lưu **learning events** (LRS nội bộ, xAPI-like), tính **ObjectiveMastery**, **ProgressSnapshot**.
- Module `learning_analytics`: read model tổng hợp cho học sinh, nhà trường (trường, lớp), môn, chủ đề, LO.
- **[Confirmed]** Không đồng nhất điểm một lần làm bài với mastery; phương pháp mastery rõ ràng, cấu hình được.
- **[Confirmed]** Không dùng dữ liệu hành vi ngoài mục đích đã công bố.
- **[Confirmed — ADR-0072]** Học sinh tự học, không có người lớn theo dõi từng lớp: **gợi ý tự động (§6) là kênh chính** đưa học sinh tới đúng hoạt động ôn tập; dashboard (§7) phục vụ nhà trường quản trị ở cấp trường → lớp → học sinh.
- **Ranh giới:** tài liệu này chỉ về tiến bộ học tập (mastery, tiến độ, LO yếu). Chỉ số hành vi chơi game (DAU/retention, funnel quest, kinh tế, chơi lành mạnh, hiệu năng thiết bị) thuộc [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md); sức khỏe hệ thống thuộc [MONITORING](../11-operations/MONITORING.md). Đo hiệu quả học tập bằng bài kiểm tra độc lập (không dùng chính mastery) thuộc [LEARNING_EFFICACY_PLAN](LEARNING_EFFICACY_PLAN.md).

## 2. Nguồn dữ liệu

```mermaid
flowchart LR
  EX["exercise / exam (learning) · quest (game) · match (competition)"] -- outbox events --> ST[(NATS JetStream\nEVT_LEARNING · EVT_GAME · EVT_COMPETITION)]
  ST --> LR[learning_record consumer]
  LR --> LE[(learning.learning_events\npartition theo tháng)]
  LR --> MM[MasteryModel] --> OM[(learning.objective_masteries)]
  LR --> PS[(learning.progress_daily)]
  OM & PS & LE --> AG["worker: analytics rollups"]
  AG --> RM[(learning.rm_* read models)]
  RM --> API[/api/v1/learning/progress, /lms/dashboards/]
```

Learning event là append-only, có `idempotency_key` (= `<source_event_id>:<student_id>`), payload theo [EVENT_DESIGN](../07-backend/EVENT_DESIGN.md). `actor.type` ∈ {`student`, `system`, `staff`}. Consumer idempotent qua `platform.processed_events`.

**Ghi nhận version (ADR-0043):** mỗi learning event mang `content_version_id` đã ghim cho lượt đó (không phải version theo con trỏ lúc xử lý event). Vì nhiều version có thể cùng được dùng trong các lượt đang diễn ra sau một lần publish/rollback, báo cáo theo câu hỏi/bài học nhóm theo `item_id` và cho phép tách theo `version_no`; widget "chất lượng câu hỏi" (tỉ lệ đúng, thời gian) tính theo từng version để phát hiện version lỗi và hỗ trợ quyết định rollback. Bảng `rm_content_version_stats(item_type, item_id, version_id, attempts, correct_rate, avg_time_ms, last_used_at)` cập nhật bởi rollup.

## 3. Mastery model mặc định (ADR-0017)

### 3.1. Định nghĩa

Mastery `M ∈ [0, 1]` theo cặp `(student_id, learning_objective_id)`. Mỗi **evidence** là một item đã chấm (`question.answered` với `is_scored=true` **và** `rapid_guess=false`) gắn LO đó. Một item gắn nhiều LO → mỗi LO nhận một evidence.

- Lần trả lời gắn cờ `rapid_guess` ([ACTIVITY_ENGINE §11.1](ACTIVITY_ENGINE.md#111-cờ-rapid_guess--trả-lời-quá-nhanh-để-kịp-đọc-đề-proposed)) **không phải evidence**: không cập nhật `M`, không tăng `n`, không tính ngày cho `d_min`; vẫn được đếm trong chỉ số `rapid_guess_rate` (§4).
- Kết quả bài chẩn đoán (`mode = diagnostic`) không đi qua công thức EWMA mà đặt `M0` ban đầu (§3.5).

Tham số (lưu trong `learning.mastery_model_configs`, phiên bản hóa, `model_version` ghi vào từng bản ghi mastery):

| Tham số | Ký hiệu | Mặc định ([Proposed] trừ khi ghi khác) |
|---|---|---|
| Prior ban đầu | `M0` | 0.30 (hoặc theo bài chẩn đoán, §3.5) |
| Tốc độ học cơ sở | `α_base` | 0.35 |
| Trần tốc độ | `α_max` | 0.60 |
| Trọng số ngữ cảnh | `w_ctx` | exam 1.0 · assignment 0.9 · practice 0.7 · retry_wrong 0.7 · quest 0.6 · challenge 0.6 · minigame 0.6 (riêng mini-game `timed_challenge` 0.4) · competition 0.4; giá trị bổ sung `challenge` 0.6 và `timed_challenge` 0.4: [Confirmed — Technical Architect 2026-11-15] |
| Trọng số độ khó khi đúng | `w_diff⁺(d)` | `0.8 + 0.1·(d − 1)` (d=1 → 0.8, d=5 → 1.2) |
| Trọng số độ khó khi sai | `w_diff⁻(d)` | `1.2 − 0.1·(d − 1)` (sai câu dễ "nặng" hơn) |
| Trọng số lặp lại | `w_rep` | 0.5 nếu cùng `question_id` đã trả lời trong 24 giờ, ngược lại 1.0 |
| Trọng số gợi ý (chỉ khi đúng) | `w_hint(h)` | h = `hints_used`: 0 → 1.0 · 1 → 0.6 · ≥ 2 → 0.3 [Confirmed — Technical Architect 2026-11-15] |
| Hệ số đoán trúng theo loại câu | `c(type)` | `true_false` 0.5 · `single_choice`, `image_based` dạng chọn: `1/k` (k = số lựa chọn hiển thị) · `multiple_choice`, `matching`, `ordering`, `drag_drop` 0.1 · `fill_in_blank`, `short_answer`, `numeric`, `interactive`, `image_based` hotspot 0 [Confirmed — Technical Architect 2026-11-15] |
| Trọng số đoán trúng (chỉ khi đúng) | `w_guess` | `1 − c(type)` |
| Ngưỡng "đúng" cho partial | — | `o ≥ 0.5` |
| Ân hạn decay | `g` | 14 ngày |
| Chu kỳ bán rã decay | `h` | 90 ngày |
| Mastered: tối thiểu evidence / ngày khác nhau | `n_min`, `d_min` | 5 / 2 |

**Ngưỡng `rapid_guess` mặc định theo loại câu** (`min_response_ms`, dùng khi câu hỏi không khai báo; cơ chế đo và gắn cờ ở [ACTIVITY_ENGINE §11.1](ACTIVITY_ENGINE.md#111-cờ-rapid_guess--trả-lời-quá-nhanh-để-kịp-đọc-đề-proposed)) [Confirmed — Technical Architect 2026-11-15]:

| Loại câu (`QuestionType`) | `min_response_ms` |
|---|---|
| `true_false` | 1.500 |
| `single_choice`, `image_based` | 2.000 |
| `multiple_choice`, `fill_in_blank`, `short_answer`, `numeric` | 3.000 |
| `matching`, `ordering`, `drag_drop` | 4.000 |
| `interactive` | không áp dụng (validator kiểm chuỗi hành động) |

Nhóm `g1_3` nhân ngưỡng với 1,5 (đọc chậm hơn, có audio đọc đề). Mini-game và competition dùng ngưỡng trong cấu hình mini-game/trận, không dùng bảng này.

Lý do: câu đúng sau khi xem gợi ý, hoặc câu có xác suất đoán trúng cao (đúng/sai), mang ít thông tin về năng lực hơn; câu trả lời sai vẫn giữ trọng số đầy đủ. `timed_challenge` khuyến khích tốc độ hơn suy nghĩ nên trọng số thấp hơn mini-game khác ([EDU_FIT_REVIEW](../12-project/EDU_FIT_REVIEW.md) PED-09).

### 3.2. Công thức cập nhật

Với evidence thứ `n+1` (đã có `n` evidence), kết quả `o = score / max_score ∈ [0, 1]`, độ khó `d`:

```text
w      = w_ctx × w_diff(o, d) × w_rep × w_hint × w_guess    # w_hint, w_guess = 1 khi o < 0.5
α_w    = min(α_max, α_base × w)
α_cold = 1 / (n + 3)                 # khởi động lạnh: vài evidence đầu cập nhật mạnh hơn
α      = max(α_w, α_cold)
M_new  = M + α × (o − M)             # EWMA, M là giá trị hiện tại (lần đầu = M0)
```

Decay khi **đọc** (không ghi đè giá trị gốc):

```text
Δ      = số ngày từ last_evidence_at
M_eff  = M × (0.5 + 0.5 × 0.5^(max(0, Δ − g) / h))   # giảm dần, tối đa còn 50% M
```

Level từ `M_eff`:

| Level | Điều kiện | Hiển thị cho học sinh (lớp 1–5 / 6–9) |
|---|---|---|
| `not_started` | n = 0 | ⚪ "Chưa học" |
| `beginning` | M_eff < 0.40 | 🌱 "Mới bắt đầu" |
| `developing` | 0.40 ≤ M_eff < 0.60 | 🌿 "Đang tiến bộ" |
| `proficient` | 0.60 ≤ M_eff < 0.80, hoặc ≥ 0.80 nhưng chưa đủ `n_min`/`d_min` | 🌳 "Thành thạo" |
| `mastered` | M_eff ≥ 0.80 **và** n ≥ 5 **và** evidence trên ≥ 2 ngày khác nhau | ⭐ "Làm chủ" |

### 3.3. Ví dụ tính

Học sinh lớp 3, LO `MATH.G3.NUM.MUL_TABLE.2`; mọi câu là `numeric` (`c = 0` → `w_guess = 1`), không dùng gợi ý (`w_hint = 1`), không có `rapid_guess`:

| # | Ngày | Ngữ cảnh | d | o | w | α_w | α_cold | α | M sau | Level |
|---|---|---|---|---|---|---|---|---|---|---|
| 0 | — | prior | — | — | — | — | — | — | 0.300 | not_started |
| 1 | 1 | practice | 2 | 1 | 0.7×0.9 = 0.63 | 0.2205 | 1/3 = 0.3333 | 0.3333 | 0.5333 | developing |
| 2 | 1 | practice | 2 | 0 | 0.7×1.1 = 0.77 | 0.2695 | 1/4 = 0.25 | 0.2695 | 0.3896 | beginning |
| 3 | 1 | practice | 3 | 1 | 0.7×1.0 = 0.70 | 0.2450 | 1/5 = 0.20 | 0.2450 | 0.5391 | developing |
| 4 | 3 | quest | 2 | 1 | 0.6×0.9 = 0.54 | 0.1890 | 1/6 = 0.1667 | 0.1890 | 0.6262 | proficient |
| 5 | 5 | exam | 3 | 1 | 1.0×1.0 = 1.00 | 0.3500 | 1/7 = 0.1429 | 0.3500 | 0.7570 | proficient |
| 6 | 5 | exam | 4 | 1 | 1.0×1.1 = 1.10 | 0.3850 | 1/8 = 0.125 | 0.3850 | 0.8506 | **mastered** (n=6, 3 ngày) |

Nếu sau đó 104 ngày không có evidence: `M_eff = 0.8506 × (0.5 + 0.5 × 0.5^(90/90)) = 0.8506 × 0.75 = 0.638` → `proficient` → hệ thống gợi ý ôn lại (§6).

Ví dụ bổ sung (golden test): evidence #1 là `true_false` đúng sau 1 gợi ý → `w = 0.7 × 0.9 × 1.0 × 0.6 × 0.5 = 0.189`, `α_w = 0.0662`, `α = α_cold = 0.3333`, `M = 0.5333` (khởi động lạnh vẫn chi phối ở vài evidence đầu); cùng câu đó trả lời sau 900 ms (`rapid_guess`) → không có evidence, `M` giữ 0.300.

Lưu ý: điểm bài exam ở ngày 5 có thể là 10/10, nhưng mastery chỉ 0.85 — hai số liệu khác nhau theo thiết kế.

### 3.4. Triển khai và lộ trình mô hình

```go
type MasteryModel interface {
    Version() string
    Apply(state MasteryState, ev Evidence, cfg MasteryConfig) MasteryState
    Effective(state MasteryState, now time.Time, cfg MasteryConfig) (value float64, level MasteryLevel)
}
```

- Cập nhật tuần tự theo `(student, objective)` — consumer partition theo `student_id` để tránh race; ghi bằng `UPDATE ... WHERE version = $v` (optimistic).
- **Recompute**: job có thể tính lại toàn bộ mastery từ `learning_events` khi đổi `model_version` (chạy shadow, so sánh, rồi chuyển). Kích hoạt một `model_version` đã chạy shadow chỉ bằng **CLI vận hành có audit** (System Admin), không có API công khai hay màn portal; kích hoạt không phát lại `learning_objective.mastered` cho LO đã từng đạt.
- Regrade/re-evaluate ([EXAM_ENGINE §9](EXAM_ENGINE.md#9-chấm-lại-regrade), [ACTIVITY_ENGINE §7.3](ACTIVITY_ENGINE.md#73-thực-thể-evaluation)) phát event bù → recompute LO bị ảnh hưởng của học sinh đó.
- **Sự kiện đạt mức:** khi level của `(student, LO)` **lần đầu** chuyển sang `mastered`, learning_record phát `learning_objective.mastered` (`student_id`, `learning_objective_id`, `model_version`, `mastered_at`) qua outbox; module game cấp XP một lần cho mỗi LO ([PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md)). Decay, recompute hay re-evaluate làm level giảm rồi đạt lại không phát lần nữa và không thu hồi phần thưởng đã cấp.
- So sánh mô hình/tham số bằng thử nghiệm A/B có kiểm soát theo [LEARNING_EFFICACY_PLAN](LEARNING_EFFICACY_PLAN.md).

**Mốc dữ liệu để chuyển sang BKT/IRT** [Confirmed — Technical Architect 2026-11-15] — chỉ đánh giá khi đủ **tất cả** điều kiện, và quyết định bằng ADR mới:

| Điều kiện | Ngưỡng |
|---|---|
| Thời gian thu thập | ≥ 1 học kỳ dữ liệu thật |
| Dữ liệu theo LO (BKT) | Mỗi LO trong phạm vi có ≥ 200 học sinh với ≥ 5 evidence (đã loại `rapid_guess`) |
| Dữ liệu theo item (IRT 1PL/2PL) | Mỗi item ≥ 200 lần trả lời đầu tiên (đã loại `rapid_guess`) |
| Chất lượng dự đoán (shadow) | Log-loss dự đoán câu trả lời kế tiếp trên tập giữ lại tốt hơn EWMA ≥ 5% |
| Giá trị dự báo bên ngoài | Tương quan với bài kiểm tra độc lập ([LEARNING_EFFICACY_PLAN §5](LEARNING_EFFICACY_PLAN.md#5-đối-chiếu-mastery-với-bài-kiểm-tra-độc-lập-predictive-validity)) không thấp hơn EWMA |
| Minh bạch | Vẫn trả lời được "vì sao" (danh sách evidence) cho nhà trường và học sinh |

### 3.5. Bài chẩn đoán đặt M0 ban đầu [Proposed]

Thay vì mọi LO bắt đầu ở cùng `M0 = 0.30`, bài chẩn đoán ngắn (`mode = diagnostic`, [ACTIVITY_ENGINE §6.1](ACTIVITY_ENGINE.md#61-mode--hoạt-động-được-gọi-từ-đâu-trong-game)) đặt prior theo từng LO:

1. **Phạm vi bài:** đầu năm học (các LO trọng tâm của khối, gồm LO tiên quyết ở khối dưới) hoặc đầu chủ đề (LO của chủ đề + LO tiên quyết trực tiếp). Tối đa 15 câu cho `g1_3`, 20 câu cho khối khác; 2–3 câu mỗi LO; không gợi ý, không phản hồi từng câu.
2. **Đặt prior:** với LO có ≥ 2 câu được chấm (đã loại `rapid_guess`), `acc` = trung bình `o`; `M0_LO = clamp(0.15 + 0.6 × acc, 0.15, 0.75)`. LO có < 2 câu giữ `M0` mặc định.
3. **Lan tới LO tiên quyết:** LO tiên quyết chưa được chẩn đoán và chưa có evidence nhận `M0` không thấp hơn `M0` của LO đã chẩn đoán phụ thuộc nó (biết LO sau thì nhiều khả năng biết LO trước).
4. **Chỉ áp dụng khi LO chưa có evidence** (n = 0). Kết quả chẩn đoán không tính vào `n`, `d_min` và không làm level rời `not_started`; bản ghi mastery lưu `prior_source = diagnostic` và `prior_diagnostic_attempt_id`.
5. Recommendation (§6) dùng `M0` này để chọn điểm bắt đầu (LO tiên quyết yếu trước, LO đã vững thì bỏ qua bài nhập môn).

Giá trị ở bước 1–2 (`diagnostic_prior`: số câu tối đa, ≥ 2 câu/LO, công thức `M0_LO`): [Confirmed — Technical Architect 2026-11-15].

## 4. Progress metrics

| Metric | Định nghĩa | Granularity |
|---|---|---|
| `lessons_completed` | số lesson có `lesson.completed` | HS × subject × ngày |
| `exercises_completed` | số attempt `graded` (mode ≠ exam) | HS × subject × ngày |
| `items_answered` / `items_correct` | số item có tính điểm / đúng | HS × LO × ngày |
| `accuracy` | `items_correct / items_answered` (chỉ lần trả lời đầu) | mọi cấp |
| `rapid_guess_rate` | tỉ lệ lần trả lời có tính điểm bị gắn `rapid_guess`; báo cáo nhà trường hiển thị là **"làm quá nhanh"** (không gọi là gian lận) | HS × subject × tuần, lớp × tuần |
| `hint_usage_rate` | tỉ lệ item có tính điểm với `hints_used ≥ 1`; trung bình `hints_used` | HS × LO, lớp × LO |
| `active_learning_time_sec` | Σ thời gian attempt (trừ pause) + thời gian lesson active (heartbeat ≤ 60 giây) — **không** tính thời gian đi lại trong game | HS × ngày |
| `attempt_count` | số attempt bắt đầu | HS × source |
| `objective_mastery_distribution` | số LO theo level | HS, lớp × subject |
| `weak_objectives` | LO level ≤ `developing` có ≥ 3 evidence | HS |
| `progress_over_time` | snapshot tuần: mean mastery, số LO mastered | HS × subject × tuần |
| `assignment_completion_rate` | đã nộp / được giao | lớp × assignment |
| `recommendation_follow_rate` | tỉ lệ gợi ý (§6) được học sinh bắt đầu trong 7 ngày; tỉ lệ LO được gợi ý lên level sau 14 ngày | trường × khối × rule |
| `curriculum_coverage` | % LO của subject × grade có ≥ 1 evidence | HS, lớp |
| `last_game_login_at` | `identity.users.last_login_at` — lần vào game gần nhất (ADR-0031) | HS |
| `days_since_last_login` | số ngày từ `last_login_at` (chưa từng: từ `provisioned_at`, hiển thị "chưa vào game") | HS, lớp (phân bố 0–7 / 8–30 / > 30 ngày) |
| `game_logins` / `game_session_minutes` | số lần vào game, tổng thời gian phiên game từ `game.login_events` (tách khỏi `active_learning_time_sec`) | HS × ngày, lớp × tuần |
| `students_scheduled_for_cleanup` | số HS trong danh sách sắp bị dọn (ADR-0032) | tenant, trường |
| `video_completion_rate` / `video_watched_ratio` | tỷ lệ video đạt `completion_rule`; tỷ lệ khoảng đã xem thật (không tính tua) ([VIDEO_LEARNING](VIDEO_LEARNING.md)) | HS × activity, lớp × chapter |
| `video_rewatch_hotspots` | đoạn bị xem lại nhiều (gợi ý nội dung khó) | activity × chapter (gộp, không theo HS) |
| `cue_accuracy` | tỷ lệ đúng câu hỏi chèn | lớp × activity |
| `simulation_goal_rate` | tỷ lệ đạt mục tiêu mô phỏng (đã kiểm chứng) | lớp × template |

**Nguồn evidence cho mastery theo loại hoạt động (ADR-0048, ADR-0072):** mọi evidence đều do **server tự chấm**, không có evidence từ chấm tay hay nhận xét của người lớn. Quiz/bài tương tác/mini-game: từng câu trả lời (`question.answered`); video/audio: câu hỏi chèn (`video.cue_answered`, câu hỏi nghe hiểu), **không** dùng thời gian xem/nghe làm evidence; mô phỏng: mục tiêu đạt **đã kiểm chứng** (`simulation.goal_met`, ADR-0052). Re-evaluate sinh evidence điều chỉnh, không xóa evidence cũ.

**Không dùng thời gian chơi hay thời gian xem làm thước đo thành công học tập chính** (Master Prompt mới §23): các chỉ số thời gian chỉ để phát hiện khó khăn (xem lại nhiều, bỏ dở, làm quá nhanh) và quản lý thời gian lành mạnh.

## 5. Read models & aggregation

| Bảng | Cập nhật | Nội dung |
|---|---|---|
| `learning.progress_daily` | streaming (consumer) | `(student_id, subject_id, bucket_date)` → counters (khóa và cột chuẩn ở [DASHBOARD_DATA §3](../07-backend/DASHBOARD_DATA.md)) |
| `learning.objective_masteries` | streaming | trạng thái mastery hiện tại |
| `learning.mastery_snapshots_weekly` | job hằng tuần (Chủ nhật 23:00) | lịch sử mastery để vẽ xu hướng |
| `learning.rm_class_objective_stats` | job mỗi 15 phút (giờ học) | `(class_id, objective_id)` → phân bố level, accuracy, số HS có evidence |
| `learning.rm_class_subject_summary` | job mỗi 15 phút | tổng hợp lớp × môn |
| `learning.rm_school_grade_summary` | job hằng đêm | trường × khối × môn (chỉ số gộp) |
| `learning.rm_student_recommendations` | streaming + hằng đêm | danh sách gợi ý (§6) |

Bảng trên gồm hai loại: **read model trạng thái hiện tại** (`objective_masteries`, `rm_*`) và **chuỗi thời gian** (`progress_daily`, `mastery_snapshots_weekly`). Dashboard có trục thời gian đọc các **rollup chuỗi thời gian** sau, dựng theo cơ chế chung ở [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) (ADR-0062: upsert gia tăng từ event, job chốt ngày 02:30 ICT, partition, read replica, cache):

| Bảng rollup | Chiều | Độ hạt | Cột đo chính |
|---|---|---|---|
| `learning.progress_daily` | học sinh, `class_id` (snapshot lúc event), môn | ngày | số hoạt động, câu trả lời, câu đúng (tử/mẫu), câu `rapid_guess`, câu có gợi ý, giây học, hist thời gian |
| `learning.rollup_class_daily` / `_weekly` / `_monthly` | lớp, môn | ngày / tuần / tháng | học sinh hoạt động (distinct, chốt cuối ngày), hoạt động hoàn thành, độ chính xác (tử/mẫu), `rapid_guess` (tử/mẫu), giây học, bài giao nộp đúng hạn (tử/mẫu) |
| `learning.rollup_class_objective_weekly` | lớp, LO | tuần | số evidence, đúng (tử/mẫu), phân bố level cuối tuần |
| `learning.rollup_school_daily` / `_weekly` / `_monthly` | trường, khối, môn | ngày / tuần / tháng | như cấp lớp + số lớp có hoạt động |
| `learning.rollup_tenant_daily` / `_weekly` / `_monthly` | khối, môn | ngày / tuần / tháng | như cấp trường + số trường có hoạt động |

- Dashboard không đọc `learning_events`, `attempts`, `attempt_answers` (DASHBOARD_DATA §1, D-1).
- Read model gắn `school_id`, lọc theo RLS như bảng nghiệp vụ.
- Báo cáo cấp tổ chức/platform chỉ dùng dữ liệu **gộp**, ẩn ô có < 5 học sinh (k-anonymity tối thiểu) [Proposed].
- Mọi aggregate theo lớp dùng enrollment **tại thời điểm event** (`learning_events.class_id` snapshot) để chuyển lớp không làm méo số liệu lớp cũ.

## 6. Recommendation rules

Gợi ý tự động là **kênh chính** thay cho việc người lớn chỉ định ôn tập (ADR-0072). Rule-based, giải thích được (không black-box) [Proposed]. Mỗi gợi ý có `recommendation_id`; attempt bắt đầu từ gợi ý mang `recommendation_id` trong `context` để đo `recommendation_follow_rate` và để module game áp dụng miễn giảm XP cho ôn LO tiên quyết ([PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md)).

| # | Điều kiện | Gợi ý | Ưu tiên |
|---|---|---|---|
| R1 | Assignment chưa nộp, `due_at` < 48 giờ | "Hoàn thành bài nhà trường giao" | 1 |
| R2 | LO level `beginning`/`developing` với ≥ 3 evidence, và LO tiên quyết ≥ `proficient` | Practice `prefer_weak_objectives` cho LO đó | 2 |
| R3 | LO yếu nhưng LO tiên quyết cũng yếu (kể cả `M0` thấp từ bài chẩn đoán) | Ôn LO tiên quyết trước | 2 |
| R4 | LO `mastered`/`proficient` có decay đưa M_eff giảm ≥ 0.15 | "Ôn lại" (spaced review) | 3 |
| R5 | Sau exam có LO đúng < 60% | Lesson + practice LO đó (EXAM_ENGINE §10) | 2 |
| R6 | Chưa có evidence ở topic kế tiếp trong lịch LMS | Lesson mới / quest tương ứng | 4 |
| R7 | Đầu năm học, hoặc bắt đầu topic mới mà phần lớn LO của topic chưa có evidence và chưa làm bài chẩn đoán trong 120 ngày | "Làm bài khởi động" (`diagnostic`, §3.5) | 2 |
| R8 | ≥ 3 LO `proficient`/`mastered` thuộc ≥ 2 topic của cùng môn, không có R2/R3 đang chờ | "Luyện trộn" — practice xen kẽ nhiều chủ đề (interleaving, [QUESTION_BANK §6](QUESTION_BANK.md#6-chọn-câu-ngẫu-nhiên-theo-điều-kiện)) | 3 |
| R9 | Có câu sai lượt đầu đến hạn ôn theo lịch `retry_wrong` | "Ôn câu sai" — ưu tiên câu anh em ([ACTIVITY_ENGINE §6.1](ACTIVITY_ENGINE.md#61-mode--hoạt-động-được-gọi-từ-đâu-trong-game)) | 3 |

Giới hạn: tối đa 3 gợi ý hiển thị cùng lúc; không gợi ý lặp lại item đã bỏ qua 3 lần; gợi ý trong game hiển thị qua NPC/quest board (không thông báo đẩy dồn dập — xem CHILD_SAFETY). Mỗi gợi ý kèm lý do dễ hiểu cho học sinh ([LEARNING_EXPERIENCE §6.2](LEARNING_EXPERIENCE.md#62-recommendation-release-10-rule-based)).

## 7. Dashboard metrics theo đối tượng

| Đối tượng | Thấy | Không thấy |
|---|---|---|
| Học sinh | mastery của mình (biểu tượng thân thiện), tiến độ, gợi ý, lịch sử bài làm | mastery/điểm của bạn khác; so sánh xếp hạng học tập |
| School Admin / Academic Manager (**School Learning Dashboard**) | trường → lớp → học sinh: tổng hợp trường × khối × môn, độ phủ chương trình, mức tham gia; bảng LO × HS của lớp, phân bố level, accuracy, assignment completion, HS cần hỗ trợ (nhiều LO yếu, lâu không học, tỉ lệ "làm quá nhanh" cao), hiệu quả gợi ý; `academic_manager` chỉ trong môn được giao phạm vi | dữ liệu trường khác; Game Rating như năng lực; nội dung chat |
| Tenant `unit` (đơn vị) | dữ liệu gộp các trường trong tenant (ẩn ô < 5 HS) | dữ liệu cá nhân học sinh (trừ khi có quyền rõ ràng) |
| Vận hành tenant (TA/SA) | thêm: hoạt động đăng nhập game (lần cuối, số ngày chưa vào, lịch sử phiên — không IP/device_id), danh sách sắp bị dọn | tenant khác |
| Platform Admin | metric vận hành, gộp | dữ liệu cá nhân theo mặc định (truy cập phải có lý do + audit) |

Kết quả trên dashboard là **tham khảo** cho nhà trường, không thay đánh giá chính thức; báo cáo theo mức của thông tư: [ASSESSMENT_REGULATION_PROFILE](ASSESSMENT_REGULATION_PROFILE.md). Màn hình School Learning Dashboard: [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md) (§8). Quy tắc truy vấn (khoảng tối đa 2 năm, độ hạt tự chọn, phân trang, timeout, cache) và API dashboard bổ sung: [DASHBOARD_DATA §8, §14](../07-backend/DASHBOARD_DATA.md#8-quy-tắc-truy-vấn).

## 8. API

| Method | Path | Mô tả |
|---|---|---|
| GET | `/api/v1/learning/progress/me?subject_id=` (`getMyProgress`) | Tiến độ của tôi: tổng quan (`lessons_completed`, `items_answered`, `accuracy`, `active_minutes`) và mastery theo LO (level + biểu tượng, giá trị số chỉ cho lớp 6–9 [Proposed]) trong cùng một response; không có endpoint mastery riêng |
| GET | `/api/v1/learning/recommendations/me` | Gợi ý |
| GET | `/api/v1/lms/dashboards/school` | School Learning Dashboard: tổng quan trường → lớp (`school_admin`, `academic_manager`) |
| GET | `/api/v1/lms/classes/{class_id}/analytics/objectives?subject=` | Bảng LO × HS của lớp (`school_admin`; `academic_manager` theo môn) |
| GET | `/api/v1/lms/classes/{class_id}/analytics/summary` | Tổng quan lớp |
| GET | `/api/v1/lms/students/{student_id}/analytics` | Hồ sơ học tập (người có quyền trong trường, có audit) |
| POST | `/api/v1/learning/events` | **Chỉ** event phía client được whitelist (`lesson.viewed`, `material.opened`, heartbeat lesson); event kết quả học tập chỉ do server sinh |

## 9. Privacy & sử dụng dữ liệu

- Purpose limitation: dữ liệu học tập dùng cho (1) phản hồi và gợi ý cho học sinh, (2) nhà trường hỗ trợ học sinh, (3) quản lý chất lượng giáo dục của trường, (4) cải thiện nội dung và đánh giá hiệu quả học tập (gộp, ẩn danh, [LEARNING_EFFICACY_PLAN](LEARNING_EFFICACY_PLAN.md)). Không dùng cho quảng cáo, không bán/chia sẻ bên thứ ba.
- Không ghi dữ liệu hành vi game chi tiết (vị trí, đường đi) vào learning events.
- Retention [Proposed]: learning events chi tiết giữ trong thời gian học sinh học tại trường + 1 năm; sau đó job gộp/ẩn danh hóa (không xóa vật lý, ADR-0070) theo [DATA_PRIVACY](../10-security/DATA_PRIVACY.md). Rà soát thời hạn theo luật ngoài version này (ADR-0082).
- Mọi truy cập hồ sơ học tập cá nhân ghi audit (`audit.audit_logs`, action `learning.profile_viewed`).
- Mô hình mastery minh bạch: nhà trường và học sinh (lớp 6–9) xem được "vì sao" (danh sách evidence gần nhất, evidence bị loại vì `rapid_guess`) cho từng ô.

## 10. Yêu cầu & kiểm thử

Mã FR, nhãn và phase do [PRODUCT_REQUIREMENTS](../00-overview/PRODUCT_REQUIREMENTS.md) (§2.9 `PRG`, §2.10 `ANL`) cấp (ADR-0082); bảng dưới chỉ chỉ ra phần chi tiết trong tài liệu này.

| Yêu cầu (mã PRD) | Chi tiết trong tài liệu này |
|---|---|
| FR-PRG-001 | Progress metrics: tiến độ, số bài, accuracy, thời gian, số lần làm, kỹ năng yếu, tiến bộ theo thời gian (§4) |
| FR-PRG-002 | Mastery theo `(student, LO)` từ nhiều evidence (§3.1) |
| FR-PRG-003 | Mô hình mastery cấu hình được, `model_version` (§3.1, §3.4) |
| FR-PRG-004 | Gợi ý ôn tập rule-based (§6) |
| FR-PRG-005 | Hiển thị cho học sinh (§7; giao diện: [LEARNING_EXPERIENCE §7](LEARNING_EXPERIENCE.md#7-hiển-thị-tiến-bộ-cho-học-sinh)) |
| FR-PRG-006 | Loại `rapid_guess` khỏi mastery, báo cáo "làm quá nhanh", hệ số đoán trúng theo loại câu (§3.1, §4) |
| FR-PRG-007 | Bài chẩn đoán đặt `M0` ban đầu theo LO (§3.5) |
| FR-PRG-008 | Gợi ý tự động có giải thích là con đường học chính (§6) |
| FR-PRG-009 | Recompute mastery khi đổi `model_version` hoặc re-evaluate (§3.4) |
| FR-EXR-012 | Trọng số `w_hint` khi dùng gợi ý (§3.1) |
| FR-EXR-014 | Gợi ý "Luyện trộn" (§6, R8) |
| FR-ANL-001 | Nguồn learning events (§2) |
| FR-ANL-002 | `learning.learning_events` append-only làm LRS nội bộ (§2) |
| FR-ANL-003 | Analytics theo học sinh, lớp, trường, môn, chủ đề, LO (§5, §7) |
| FR-ANL-004 | Purpose limitation (§9) |
| FR-ANL-005 | Read model và aggregation trong worker (§5) |
| FR-ANL-015 | Báo cáo tham khảo TT27/2020, TT22/2021 ([ASSESSMENT_REGULATION_PROFILE](ASSESSMENT_REGULATION_PROFILE.md)) |
| FR-ANL-016 | Báo cáo gộp ẩn ô < 5 học sinh (§5, §7) |

Kiểm thử bắt buộc: bảng §3.3 (kể cả ví dụ bổ sung) là golden test của `MasteryModel`; event trùng không làm thay đổi mastery; evidence `rapid_guess` không đổi `M`, `n`; diagnostic đặt `M0` đúng công thức và không đổi level; Game Rating thay đổi không ảnh hưởng mastery; `school_admin` trường A không đọc analytics trường B; `academic_manager` môn Toán không đọc analytics môn Tiếng Việt.
