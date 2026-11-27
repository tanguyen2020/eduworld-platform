# Anti-cheat

> Trạng thái: Draft · Phiên bản: 0.2 · Owner: Multiplayer Engineer + Identity & Security Engineer · Cập nhật: 2026-11-27 (F132: harness §11, metric speed hack §10, mã từ chối đề §3) · 2026-11-15
> Liên quan: [ADR-0016, ADR-0018, ADR-0010, ADR-0011](../adr/README.md) · [EXAM_ENGINE](../02-learning/EXAM_ENGINE.md) · [EXERCISE_ENGINE](../02-learning/ACTIVITY_ENGINE.md) · [MATCHMAKING](../03-social-multiplayer/MATCHMAKING.md) · [RANKING](../01-game-design/COMPETITION_SYSTEM.md) · [GAME_NETWORKING](../08-game-client/GAME_NETWORKING.md) · [SECURITY_ARCHITECTURE §14](SECURITY_ARCHITECTURE.md)

**Mục đích & phạm vi.** Nguồn duy nhất về chống gian lận game, học tập và thi đấu: server-authoritative, kiểm tra thời gian, chống trùng/replay, rate limit đặc thù gameplay, phát hiện bất thường, **quy tắc và giá trị chống farm XP/Xu**, xử lý vi phạm. Rate limit chung của API → [SECURITY_ARCHITECTURE §14](SECURITY_ARCHITECTURE.md).

## 1. Mô hình mối đe dọa

Người gian lận điển hình là **học sinh** (6–15 tuổi) có thể: dùng tool sửa bộ nhớ/tốc độ game, chặn/sửa request bằng proxy, chạy script tự trả lời, nhờ người khác làm, dùng nhiều tài khoản (của bạn), farm XP bằng câu dễ, cố tình thua để giảm rating (sandbagging), thông đồng trong thi đấu. Mục tiêu: **bảo vệ tính công bằng của kết quả học tập và thi đấu**, xử lý theo hướng giáo dục, không tạo môi trường nghi ngờ trẻ.

Nguyên tắc nền: **client không đáng tin**. Mọi thứ có giá trị (điểm, mastery, XP, reward, rating, kết quả trận, bracket) do server quyết định (ADR-0016). Godot có thể bị sửa toàn bộ — không đặt bất kỳ bí mật hay quyết định nào trong client.

## 2. Server-authoritative

| Dữ liệu | Client gửi | Server làm |
|---|---|---|
| Câu trả lời | `response` thô + `attempt_item_id` | Chấm bằng answer key server, tính điểm theo scoring rule |
| Thời gian làm bài | (tham khảo) `client_elapsed_ms` | Dùng `started_at`/`deadline_at`/`answered_at` theo đồng hồ server |
| Kết quả trận | Input trả lời từng câu | State machine trong module `competition` (battle room) quyết định thắng/thua, điểm, thời gian |
| XP/Reward | Không gửi | Reward rule chạy trên event server |
| Vị trí hub | Trạng thái di chuyển ≤ 10 Hz | Kiểm tra tốc độ tối đa, biên bản đồ, vùng cấm; điều chỉnh (snap back) nếu vượt |
| Quest step | "Tôi đã đến NPC X" | Kiểm tra vị trí server-side gần trigger (bán kính) + điều kiện bước |

Answer key không bao giờ có trong render payload; `explanation` chỉ trả sau khi trả lời (practice) hoặc sau khi đóng đề (exam). Thứ tự đáp án xáo trộn theo `seed` của attempt — server ánh xạ lại khi chấm.

## 3. Kiểm tra thời gian

| Kiểm tra | Quy tắc [Confirmed] | Hành động |
|---|---|---|
| Deadline | `answered_at > deadline_at + grace(2s)` | Từ chối (`ATTEMPT_DEADLINE_PASSED` với attempt có thời hạn; đề thi trả `422 EXAM_TIME_OVER` theo `exam_offline_sync_grace` — [EXAM_ENGINE §6](../02-learning/EXAM_ENGINE.md)); exam tự động submit tại deadline bởi worker |
| Thời gian tối thiểu hợp lý | `min_plausible_ms` theo loại câu + độ dài stem: đọc 2 từ/giây cho lớp 1–3, 4 từ/giây lớp 4–9, tối thiểu 800 ms; matching/ordering nhân số mục | Không từ chối; gắn cờ `too_fast` vào answer; ≥ 30% câu `too_fast` và đúng trong một attempt → tín hiệu anomaly |
| Battle | Câu trả lời trước khi server gửi câu hỏi (`answered_at < question_sent_at + RTT_min`) | Bỏ qua; tín hiệu |
| Speed hack (hub) | Khoảng cách / thời gian > `max_speed × 1.2` | Snap back; ≥ 5 lần/phút → tín hiệu |
| Đồng hồ client | Không dùng cho quyết định | — |

## 4. Duplicate submission, idempotency & replay

- `POST .../answers`, `...:submit`, `/exams/.../submit`, hoàn thành quest step, nhận reward, xác nhận kết quả trận: bắt buộc `Idempotency-Key` (ADR-0010). Lưu `platform.idempotency_records(key, user_id, route, request_hash, response, expires_at)`; cùng key + khác `request_hash` → 422 `IDEMPOTENCY_KEY_REUSED`.
- Ràng buộc DB: attempt submit một lần (`status` chuyển qua `UPDATE ... WHERE status='in_progress'`); `match_results.match_id` unique; XP ledger unique `(source_type, source_id, student_id, rule_id)`; `reward_grants` unique theo event gốc.
- Exam: một học sinh một attempt active cho mỗi `exam_session` (unique partial index); mở lại tab/thiết bị khác → resume attempt cũ, không tạo mới.
- WebSocket: mỗi message client có `id` (UUID); realtime server giữ cửa sổ `id` đã xử lý 5 phút per connection/room; message `seq` cũ hơn bị bỏ. Battle answer gắn `question_instance_id` duy nhất; câu trả lời thứ 2 cho cùng câu bị bỏ.
- Realtime ticket dùng 1 lần, TTL 60s (ADR-0008).
- Không có endpoint nào nhận "score" hay "xp" từ client — CI có Spectral rule cấm field `score`, `xp`, `is_correct`, `rating` trong request schema của học sinh.

## 5. Rate limit

Theo [SECURITY_ARCHITECTURE §14](SECURITY_ARCHITECTURE.md). Bổ sung riêng anti-cheat:

| Hành động | Giới hạn |
|---|---|
| Bắt đầu attempt practice cùng question set | 20/giờ |
| Matchmaking ticket | 30/giờ; hủy/bỏ ready check ≥ 3 lần liên tiếp → cooldown 2 phút, ≥ 5 → 10 phút |
| Rời trận giữa chừng | Tính thua; abandon lặp lại → hạn chế ranked (ngưỡng và thời gian do [MULTIPLAYER_GAMEPLAY](../03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md) sở hữu) |
| Retry câu sai | Không giới hạn học tập, nhưng không XP sau cap (§7) |

## 6. Anomaly detection [Confirmed]

Chạy trong worker (consumer event `question.answered`, `match.completed`, `activity.graded`), ghi `competition.integrity_signals` / `assessment.integrity_signals` (`student_id`, `signal_type`, `score`, `evidence`, `created_at`). Không tự động phạt; tổng hợp thành **integrity score** theo cửa sổ 7 ngày.

| Tín hiệu | Phát hiện | Trọng số |
|---|---|---|
| Accuracy bất thường | Tỉ lệ đúng ở câu độ khó cao vượt xa mastery hiện tại (vd. mastery < 0.3 nhưng đúng > 90% trên ≥ 10 câu khó, thời gian rất ngắn) | Cao |
| Pattern thời gian | Phân bố thời gian trả lời quá đều (std rất thấp), hoặc toàn bộ ≈ `min_plausible_ms` | Trung bình |
| Too-fast | Tỉ lệ `too_fast` cao | Trung bình |
| Đa tài khoản / account sharing | Cùng `device_id` (cài đặt ngẫu nhiên, không phải định danh phần cứng) đăng nhập nhiều tài khoản trong thời gian ngắn và các tài khoản đấu với nhau; một tài khoản đăng nhập song song từ 2 thiết bị trong thi đấu | Cao (cho competition) |
| Win trading | Cặp người chơi đấu với nhau lặp lại bất thường, kết quả luân phiên hoặc một bên luôn bỏ cuộc | Cao |
| Sandbagging | Rating giảm mạnh do thua nhanh/không trả lời, sau đó thắng liên tục | Trung bình |
| Bot hub | Đường di chuyển tuyến tính hoàn hảo, input tần số cố định nhiều giờ | Thấp |
| XP farming | Lặp cùng nội dung dễ nhiều lần (đã chặn ở reward, chỉ dùng làm tín hiệu) | Thấp |

Lưu ý thực tế trường học: nhiều học sinh dùng **chung máy phòng tin** và **chung IP NAT** → không dùng IP/thiết bị đơn lẻ làm bằng chứng; device-sharing chỉ có ý nghĩa khi kết hợp với tín hiệu competition.

## 7. Chống farm XP

**Quy tắc và giá trị** chống farm (giảm dần khi lặp, cap XP/ngày, XP câu sai/câu đã xem giải thích, hệ số nội dung thấp hơn khối) thuộc [PROGRESSION_SYSTEM §3.2](../01-game-design/PROGRESSION_SYSTEM.md); cap Xu thuộc [INVENTORY_ECONOMY](../01-game-design/INVENTORY_ECONOMY.md); cơ chế sổ cái: [ADR-0049](../adr/ADR-0049-so-cai-phan-thuong-va-tien-te-trong-game-reward-currency-led.md). Tài liệu này chỉ mô tả **phát hiện và xử lý** khi có người cố vượt các quy tắc đó:

- **Thực thi phía server:** mọi XP/Xu tính từ domain event; client không gửi số lượng. Grant idempotent theo nguồn (ADR-0049), nên gửi lặp request/replay event không cộng thêm.
- **Tín hiệu farm** (ghi `integrity_signals`, không tự phạt): một học sinh chạm cap XP/Xu nhiều ngày liên tiếp; lặp lại cùng content version với tỷ lệ đúng gần tuyệt đối và thời gian trả lời rất ngắn; luân phiên tài khoản trên cùng thiết bị cùng làm một nội dung; tỉ lệ bỏ trận/đầu hàng sớm cao bất thường ở chế độ có XP.
- **Competition:** XP/Competition Points chỉ khi trận hoàn thành hợp lệ (không bị hủy, đối thủ không AFK toàn trận); friend duel không tính rating ([COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md)).
- **Không có động cơ farm để bán:** phần thưởng không quy đổi sang tiền hay lợi thế học tập; không có giao dịch giữa người chơi trong Release 1.0 ([INVENTORY_ECONOMY](../01-game-design/INVENTORY_ECONOMY.md), trade tắt).
- **Xử lý:** theo §9 (thân thiện với trẻ); điều chỉnh số dư sai do lỗi/gian lận dùng bút toán bù có lý do + four-eyes (`/admin/economy/adjustments`).

## 8. Bảo vệ API thi đấu

- Battle chạy trong module `competition` (lệnh WS tới qua realtime → NATS request/reply), câu hỏi và chấm điểm lấy từ module `learning` qua facade gọi trong tiến trình `core-api` (room host của module competition chạy trong `core-api`, ADR-0074): câu hỏi chỉ gửi khi bắt đầu lượt, không gửi trước cả bộ đề; đáp án và kết quả công bố sau khi lượt đóng cho mọi người cùng lúc.
- Không có REST endpoint cho phép client tạo match result; `MatchResult` chỉ được ghi bởi room state machine qua use case `competition.RecordResult` (idempotent theo `match_id`).
- Matchmaking: client chỉ tạo/hủy ticket; không chọn đối thủ (trừ friend match có xác nhận 2 bên). Ranked friend match không tính rating [Confirmed] để tránh win trading.
- Tournament: kết quả match giải được xác nhận bởi hệ thống; điều chỉnh thủ công cần quyền tổ chức giải + audit + lý do.
- Reconnect: được vào lại room trong 30s với cùng user (kiểm tra principal), không phải bằng room token chia sẻ được.
- **Một thiết bị cho mỗi tài khoản (ADR-0031):** chỉ một game session active/học sinh, nên không thể chơi song song hai trận, hai máy cùng tài khoản (vd. một máy tra cứu, một máy thi đấu với cùng danh tính). Takeover giữa trận: thiết bị cũ bị đóng, thiết bị mới vào lại trong grace; mọi takeover ghi audit, tần suất cao bất thường (`GameTakeoverSpike`) là tín hiệu dùng chung tài khoản.
- **Chỉ học sinh vào game (ADR-0031):** nhân sự nhà trường/vận hành không thể tham gia trận, xếp hạng hay giải đấu — tránh tài khoản nhân sự làm lệch ranking.

## 9. Xử lý vi phạm (thân thiện với trẻ)

| Mức | Điều kiện | Hành động | Ai quyết định |
|---|---|---|---|
| 0 — Quan sát | Tín hiệu đơn lẻ | Không hành động; giữ dữ liệu 30 ngày | Tự động |
| 1 — Nhắc nhở | Integrity score vượt ngưỡng thấp | Thông điệp nhẹ nhàng trong game: "Hãy tự làm bài để biết mình đã giỏi đến đâu nhé!" | Tự động |
| 2 — Vô hiệu kết quả thi đấu | Bằng chứng mạnh trong competition | Trận bị đánh dấu `voided`, rating hoàn tác; không ảnh hưởng learning record | Tự động; ghi nhận vào báo cáo tổng hợp cho nhà trường |
| 3 — Xem xét | Gian lận ở assignment/exam do nhà trường giao | Báo nhà trường (`academic_manager`/`school_admin`) kèm bằng chứng (thời gian, pattern) trong School Learning Dashboard; **nhà trường** quyết định với kết quả học tập (ví dụ cho làm lại); hệ thống không tự trừ điểm | `academic_manager` / `school_admin` |
| 4 — Hạn chế | Lặp lại sau nhắc nhở | Tạm loại khỏi ranked/tournament 7 ngày | School Admin |

Không công khai tên học sinh vi phạm; không trừ điểm học tập tự động bởi thuật toán. Học sinh có thể giải trình qua nhà trường; mọi thao tác có audit.

## 10. Audit & quan sát

- Audit: void match, rollback rating, điều chỉnh kết quả tournament, xử lý vi phạm.
- Metrics: `eduworld_integrity_signals_total{type}`, `eduworld_idempotency_replays_total{route}`, `eduworld_answers_too_fast_ratio`, `eduworld_matches_voided_total{mode}`, `eduworld_rt_speed_violations_total` (module competition, F132; danh mục và nhãn: [MONITORING §3](../11-operations/MONITORING.md#3-metric-bắt-buộc-master-prompt-18)). `eduworld_answers_too_fast_ratio` và `eduworld_matches_voided_total` chưa phát (tín hiệu tương ứng ghi ở `integrity_signals`; void trận chưa có use case).
- Dashboard cho Multiplayer Engineer/Safety; báo cáo tuần cho tổ chức giải.

## 11. Kiểm thử

| ID | Kịch bản | Kỳ vọng |
|---|---|---|
| AC-CMP-201 | Gửi submit 2 lần cùng Idempotency-Key | Một kết quả, response giống nhau |
| AC-CMP-202 | Gửi submit 2 lần khác key | Lần 2: 409 `ATTEMPT_ALREADY_SUBMITTED` |
| AC-CMP-203 | Client gửi field `score` trong body | Bị bỏ qua/422 theo schema; điểm do server tính |
| AC-CMP-204 | Trả lời sau deadline exam | Từ chối; attempt tự submit tại deadline |
| AC-CMP-205 | Gửi answer battle cho câu chưa phát | Bỏ qua |
| AC-CMP-206 | Kết quả trận ghi lại 2 lần (worker retry) | Rating cập nhật đúng một lần |
| AC-CMP-207 | Disconnect rồi reconnect trong 30s | Tiếp tục trận; quá 30s → thua lượt còn lại |
| AC-CMP-208 | Sửa client tăng tốc chạy | Server snap back, tín hiệu ghi nhận |
| AC-CMP-209 | Làm lại cùng bài 5 lần | XP: 100%, 50%, 25%, 0, 0 |
| AC-CMP-210 | Replay WS message cũ | Bỏ qua theo `id`/`seq` |

Harness tự động (F132): `src/backend/test/anticheat` gửi request của "client bị sửa" qua API public và giao thức realtime (không import module), chạy 10 kịch bản trên bằng `make anticheat ENV=staging` với học sinh QA của tenant QA (workflow `anticheat.yml`, trước mỗi release); báo cáo đạt chỉ khi 10/10 đạt, kịch bản thiếu fixture bị đánh dấu bỏ qua. Spec: rule Spectral `no-client-authoritative-fields` (`specs/spectral-functions/noClientAuthoritativeFields.js`, chạy trong `make specs-lint`).
