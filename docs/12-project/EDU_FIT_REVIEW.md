# Review độ phù hợp của bộ docs/specs cho game chuyên giáo dục

> Trạng thái: Draft · Phiên bản: 0.4 · Owner: Technical Architect · Cập nhật: 2026-11-17 (§9: ADR-0074 Accepted — K6 đã xử lý; quyết định Technical Architect 2026-11-17 — ADR-0073/0075/0078/0080 Accepted, ADR-0076/0079 Rejected)
> Liên quan: [DOCUMENTATION_AUDIT](DOCUMENTATION_AUDIT.md) · [RISKS](RISKS.md) · [OPEN_QUESTIONS](OPEN_QUESTIONS.md) · [ROADMAP](ROADMAP.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) · [adr/README](../adr/README.md)

## 1. Mục đích và phạm vi

Trả lời câu hỏi: **bộ tài liệu và đặc tả hiện tại đã đủ cho một game chuyên về giáo dục (lớp 1–9, Việt Nam) chưa, và các lựa chọn công nghệ có phù hợp không?**

Review chỉ đọc, chia 5 chiều: (1) sư phạm & thiết kế học tập, (2) cân bằng game–giáo dục & phạm vi, (3) an toàn trẻ em, quyền riêng tư, trợ năng, thực tế lớp học, (4) độ đầy đủ & nhất quán của `specs/`, (5) lựa chọn công nghệ. Mọi kết luận "không có" đã được grep trên `docs/` và `specs/`; các phát hiện Critical đã được kiểm tra lại thủ công. Nhận định dựa trên kiến thức bên ngoài (luật, tình trạng thư viện) gắn nhãn **[Ngoài — cần xác minh]**.

Tài liệu này là đánh giá, không thay quyết định. Mỗi khuyến nghị được chấp nhận phải đi qua ADR mới / sửa tài liệu sở hữu theo [docs/README §3.6](../README.md).

Các phát hiện §2–§7 được ghi tại thời điểm review, **trước** [ADR-0072](../adr/ADR-0072-bo-vai-tro-giao-vien-hoc-sinh-tu-hoc-kiem-duyet-tap-trung.md) (bỏ vai trò giáo viên); những khuyến nghị nhắc tới giáo viên được giữ nguyên làm bằng chứng. Cách xử lý hiện hành của từng phát hiện xem [§9](#9-trạng-thái-xử-lý).

## 2. Kết luận

**Chưa đáp ứng đủ.** Nền tảng ở tầng miền nghiệp vụ thuộc nhóm tốt so với game giáo dục thông thường, nhưng còn 6 vấn đề sống còn cần xử lý trước khi coi baseline là đủ cho sản phẩm giáo dục:

| # | Vấn đề sống còn | Chiều |
|---|---|---|
| K1 | Không có kế hoạch chứng minh hiệu quả học tập; KPI mastery tự đo bằng chính model của hệ thống | Sư phạm |
| K2 | Khối lượng nội dung chưa lượng hóa (OQ-08 mở), engine đòi rất nhiều nội dung, đội chỉ 1 EdTech | Sư phạm, phạm vi |
| K3 | Phạm vi Release 1.0 "đủ mọi thứ, không MVP" (~80 tuần, ~12 người), bằng chứng với trường đến quá muộn | Phạm vi |
| K4 | Lõi contract giáo dục chưa đủ để sinh code: không đặc tả `response` theo loại câu, enum loại câu hỏi và chính sách phản hồi lệch giữa các spec | Specs |
| K5 | Thiếu kịch bản lớp học thực tế: không có chế độ lớp do giáo viên điều khiển, lớp 1–2 phải gõ mật khẩu mỗi lần mở game, không tính băng thông cả phòng máy | Lớp học |
| K6 | Kiến trúc vận hành vượt năng lực đội: 6 microservice + hạ tầng stateful tự quản cho ~3 Go dev và 0,5 DevOps; floci làm object storage production giữ bài nộp của học sinh | Công nghệ |

Ngoài ra, cơ sở pháp lý về dữ liệu trẻ em và game trực tuyến cho trẻ vị thành niên chưa được cập nhật (§4.3).

## 3. Điểm mạnh nên giữ

- **Tách số liệu học tập khỏi số liệu game:** Learning score, Mastery, XP, Rating, Competition Points riêng biệt; XP/rating không vào mastery hay báo cáo giáo viên ([PROGRESSION_SYSTEM §2](../01-game-design/PROGRESSION_SYSTEM.md), [ADR-0018](../adr/ADR-0018-glicko-2-cho-game-rating-tach-biet-learning-mastery.md)).
- **Server-authoritative, answer key không rời server** ([ADR-0016](../adr/ADR-0016-server-authoritative-scoring-answer-key-khong-bao-gio-roi-se.md)); `question.schema.json` tách `render_payload`/`answer_key`/`scoring_rule` cho đủ 11 loại câu.
- **Version nội dung bất biến + ghim version** khi làm bài ([ADR-0043](../adr/ADR-0043-phat-hanh-noi-dung-draft-publish-con-tro-phien-ban-giu-phien.md)).
- **Mastery minh bạch:** EWMA có trọng số, cần ≥ 5 evidence trên ≥ 2 ngày, shadow recompute, giáo viên xem được "vì sao" ([LEARNING_ANALYTICS §3](../02-learning/LEARNING_ANALYTICS.md), [ADR-0017](../adr/ADR-0017-mastery-model-co-the-cau-hinh-mac-dinh-weighted-evidence.md)).
- **Quest gắn LO trong data model** (`objective_ids` bắt buộc), quy tắc không quá N câu liên tiếp ([QUEST_SYSTEM §2.1](../01-game-design/QUEST_SYSTEM.md), [GAME_DESIGN §6](../01-game-design/GAME_DESIGN.md)).
- **Chống gây nghiện, giảm áp lực thi đấu:** không streak, không loot box, cap XP/Xu, lớp 1–3 không có thứ hạng cá nhân, khối lớp là ràng buộc cứng khi ghép trận ([GAME_DESIGN §13](../01-game-design/GAME_DESIGN.md), [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md)).
- **An toàn trẻ em thực thi ở server:** bảng điểm thực thi EP-01…EP-12, nickname sinh tự động, không có người lớn trong game, tín hiệu grooming/tự hại ([CHILD_SAFETY](../10-security/CHILD_SAFETY.md), [ADR-0031](../adr/ADR-0031-game-chi-danh-cho-hoc-sinh-luu-lich-su-dang-nhap-mot-thiet-b.md)).
- **Multi-tenancy hai lớp** (ent filter + RLS), soft-delete + audit, mọi chuỗi có giới hạn độ dài: 2.463 trường, 0 vi phạm; 6 script kiểm tra spec chạy 0 lỗi.
- **Ngân sách hiệu năng theo hạng thiết bị tầm trung**, chặn bằng benchmark CI ([ADR-0057](../adr/ADR-0057-chay-tren-may-tam-trung-hang-thiet-bi-va-ngan-sach-hieu-nang.md)); dashboard chỉ đọc rollup ([ADR-0062](../adr/ADR-0062-kien-truc-du-lieu-dashboard-khong-truy-van-du-lieu-tho.md)).
- [RISKS](RISKS.md) trung thực, đã tự nhận diện phần lớn rủi ro dưới đây.

## 4. Phát hiện theo chiều

Mức độ: **C** Critical · **H** High · **M** Medium · **L** Low.

### 4.1. Sư phạm & thiết kế học tập

| ID | Mức | Phát hiện | Bằng chứng | Khuyến nghị |
|---|---|---|---|---|
| PED-01 | C | Không có kế hoạch đo hiệu quả học tập; KPI "mastery gain" đo bằng chính model mastery (vòng tròn); exit criteria Release 1.0 không có cổng hiệu quả | grep `efficacy\|pre-test\|post-test\|đối chứng` = 0; [PRODUCT_VISION §7](../00-overview/PRODUCT_VISION.md); [ROADMAP §3.7](ROADMAP.md) | Tài liệu `LEARNING_EFFICACY_PLAN`: test độc lập trước/sau, lớp so sánh ở trường pilot, đối chiếu mastery với điểm giáo viên, hạ tầng A/B cho `mastery_model_configs`; thêm exit criterion hiệu quả cho pilot |
| PED-02 | C | Năng lực sản xuất nội dung không khả thi: seed chỉ 16 LO mẫu; selection rule cần pool ≥ 3 × count; 1v1 không lặp câu 7 ngày; song ngữ; audio lớp 1–2. Ước tính thô ~57.000 câu cho 4 môn × 9 khối | [ROADMAP §1, §2](ROADMAP.md) (1 EdTech, khối lượng [TBD]); [CURRICULUM_MODEL §7](../02-learning/CURRICULUM_MODEL.md); [QUESTION_BANK §6](../02-learning/QUESTION_BANK.md) | Mô hình throughput (LO × item/LO × chi phí audio/dịch), chốt OQ-08 theo phạm vi pilot, ưu tiên câu tham số hóa, quest `learning` sinh từ template, cân nhắc AI sinh nháp + người duyệt (ADR), nâng RISK-06 |
| PED-03 | H | Học tập phần lớn là "cổng"/"lớp vỏ": mini-game được định nghĩa là lớp trình bày trên câu hỏi; bước `collect`/`go_to` không mang giá trị học tập; boss = trả lời câu hỏi để trừ năng lượng | [MINIGAME_ENGINE M-3](../08-game-client/MINIGAME_ENGINE.md); [QUEST_SYSTEM §3, §9](../01-game-design/QUEST_SYSTEM.md); grep `intrinsic\|nội tại` = 0 | Nguyên tắc "hành động chính = khái niệm học" trong GAME_DESIGN; catalog learning mechanic theo môn (trục số để di chuyển, ghép âm tiết mở cửa, cân phương trình sửa cầu); cho bước `collect`/`go_to` có validator học tập |
| PED-04 | H | Gợi ý và phản hồi theo lỗi sai có trong thiết kế nhưng không có trong contract | `hints[]` có trong `question.schema.json`, nhưng grep `hint` trong `learning.v1.yaml` = 0; `learning-event.schema.json` không có `hints_used`; không có feedback theo phương án nhiễu, không có `worked_example` | Thêm `option.feedback` (chỉ trả sau khi chấm), endpoint `…/items/{pos}/hints:next`, `hints_used` vào event và giảm trọng số mastery khi dùng gợi ý, content type `worked_example` |
| PED-05 | H | Evidence không lọc đoán bừa: mastery không lọc theo `response_ms`; `suspicious_speed` chỉ bắt đúng 100% và nhanh; không hiệu chỉnh xác suất đoán trúng; `retry_wrong` dùng lại cùng `question_id` làm mastery tăng ảo | [LEARNING_ANALYTICS §3.1](../02-learning/LEARNING_ANALYTICS.md); [ACTIVITY_ENGINE §6.1, §11](../02-learning/ACTIVITY_ENGINE.md); grep `guess\|đoán` = 0 | Cờ `rapid_guess` theo ngưỡng thời gian từng item, loại khỏi mastery và báo giáo viên; hệ số đoán trúng theo dạng câu; dùng câu tương tự khi retry |
| PED-06 | H | Bám GDPT 2018 còn nông: LO chỉ có `bloom_level`, không có mã yêu cầu cần đạt, năng lực, phẩm chất; không có thang đánh giá theo TT27/2020 (tiểu học) và TT22/2021 (THCS) | [CURRICULUM_MODEL §3.3](../02-learning/CURRICULUM_MODEL.md); grep `TT27\|TT22\|Hoàn thành tốt` = 0; [EXAM_ENGINE §10](../02-learning/EXAM_ENGINE.md) dùng Giỏi/Khá | Thêm `yccd_ref`, `competency_ids[]` vào LO; profile đánh giá theo cấp học cho sổ điểm, xếp loại, mẫu nhận xét; EdTech review seed theo văn bản chương trình |
| PED-07 | M | Lớp 1–2 (chưa đọc thạo) thiếu thiết kế: audio "lớp 1–2" vs TTS "lớp 1–3" không nhất quán, chưa chọn engine TTS tiếng Việt, thoại NPC không bắt buộc lồng tiếng, không có hoạt động đánh vần/tập viết | `question.schema.json`, [CONTENT_MODEL §8](../02-learning/CONTENT_MODEL.md), [LEARNING_EXPERIENCE §5](../02-learning/LEARNING_EXPERIENCE.md); grep `đánh vần\|tập viết` = 0 | Voice-over bắt buộc cho nhóm `g1_3` kèm ngân sách audio; activity ghép âm và tô chữ; phụ đề + icon khi không có tai nghe |
| PED-08 | M | Adaptive và cold start yếu: độ khó do tác giả tự khai, không có vòng hiệu chỉnh từ `p_value`; `M0` như nhau cho mọi LO; không có bài chẩn đoán đầu vào; không có interleaving | [QUESTION_BANK §9](../02-learning/QUESTION_BANK.md); grep `diagnostic\|chẩn đoán\|placement` = 0 (chỉ khớp trường kéo-thả và Docker) | Bài chẩn đoán ngắn đầu năm/chủ đề; hiệu chỉnh độ khó bán tự động; chế độ luyện trộn chủ đề; mốc dữ liệu cụ thể để chuyển sang BKT/IRT |
| PED-09 | M | XP xung đột với mục tiêu học: nội dung thấp hơn ≥ 2 khối chỉ được XP × 0,25 trong khi gợi ý ôn LO tiên quyết lại trỏ về khối dưới; thưởng tốc độ/chuỗi khuyến khích làm nhanh hơn suy nghĩ | [PROGRESSION_SYSTEM §3.2](../01-game-design/PROGRESSION_SYSTEM.md); [LEARNING_ANALYTICS §6](../02-learning/LEARNING_ANALYTICS.md) | Miễn giảm XP cho hoạt động ôn tập do hệ thống/giáo viên giao; thưởng khi lên mức mastery thay vì theo từng câu; giảm trọng số evidence từ `timed_challenge` |
| PED-10 | M | Gánh nặng chấm bài: nói/tự luận chấm tay, lớp 40+ học sinh; giáo viên không ghi chú được trên mastery; không gợi ý chia nhóm phân hóa | FR-CNT-015, [ADR-0053](../adr/ADR-0053-cham-diem-co-ho-tro-ai-ai-assisted-grading.md), [ADR-0059](../adr/ADR-0059-hop-nhat-api-attempt-va-chua-lam-ai-giong-noi.md) | Chấm theo lô, rubric nhanh; tự đề xuất nhóm theo LO yếu; nguồn evidence `teacher_observation` |
| PED-11 | M | Đề `is_official` thiếu kiểm tra độ tin cậy (KR-20/alpha) và cân bằng độ khó giữa các biến thể đề | [EXAM_ENGINE §3](../02-learning/EXAM_ENGINE.md) | Cân bằng biến thể theo `p_value`, báo cáo độ tin cậy cho bài chính thức |

### 4.2. Cân bằng game–giáo dục và phạm vi

| ID | Mức | Phát hiện | Bằng chứng | Khuyến nghị |
|---|---|---|---|---|
| GD-01 | C | Phạm vi Release 1.0 không khả thi; Teacher Loop chỉ xong khoảng tuần 42, UAT ở Phase 6, đánh giá mastery thật ở M7; RISK-05 còn nhắc "MVP" mâu thuẫn với ROADMAP §1 | [ROADMAP §1, §2, §3.7](ROADMAP.md); [RISKS](RISKS.md) RISK-05 | ADR mới xem lại quyết định "không MVP": bản pilot ưu tiên giáo dục (§6), đi kèm [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) |
| GD-02 | H | Co-op thực chất là quiz cá nhân chạy song song (`collective_count` cộng số câu đúng); enum policy lệch giữa PARTY_SYSTEM và QUEST/schema; `quest.schema.json` không có `role` | [QUEST_SYSTEM §6](../01-game-design/QUEST_SYSTEM.md); [PARTY_SYSTEM §7, §7.1](../03-social-multiplayer/PARTY_SYSTEM.md); grep `collaborat` trong 02-learning và schemas = 0 | Kiểu bước phụ thuộc lẫn nhau (mỗi người giữ một phần dữ kiện); thống nhất enum; thêm `role` vào schema |
| GD-03 | H | Bảng XP của lớp (lớp 4–9) vô tình xếp hạng năng lực công khai, vì XP chỉ tính câu đúng lần đầu; RISK-12 nói "trường tắt được" nhưng leaderboard chỉ platform cấu hình | [COMPETITION_SYSTEM §9.2, §13](../01-game-design/COMPETITION_SYSTEM.md); [PROGRESSION_SYSTEM §3.1](../01-game-design/PROGRESSION_SYSTEM.md); [GAME_DESIGN §13](../01-game-design/GAME_DESIGN.md) | Bảng lớp dựa trên nỗ lực/tiến bộ hoặc mục tiêu tập thể; giáo viên bật/tắt được cho lớp mình |
| GD-04 | M | Thi đấu cho trẻ: bonus tốc độ trong 1v1; câu trả lời trong trận vẫn vào mastery (trọng số 0,4); 1v1 mở cả lớp 1; hàng đợi chia theo trường × khối × mode × môn khó đạt KPI thời gian chờ | [COMPETITION_SYSTEM §2–§4](../01-game-design/COMPETITION_SYSTEM.md); [MATCHMAKING](../03-social-multiplayer/MATCHMAKING.md) H3 | Bỏ bonus tốc độ cho lớp 1–5; bỏ hoặc giảm evidence từ trận; gộp hàng đợi, bot luyện tập |
| GD-05 | M | Không có quy tắc khi lên lớp: mạch truyện `once`, level, vùng đã mở khi sang năm học mới | grep `lên lớp\|năm học mới` trong 01-game-design = 0 | Thêm quy tắc chuyển năm học vào PROGRESSION_SYSTEM |
| GD-06 | M | Không có kênh phụ huynh, trong khi MODERATION/CHILD_SAFETY/DATA_PRIVACY dựa vào phụ huynh; SIS không lấy thông tin phụ huynh | [OPEN_QUESTIONS](OPEN_QUESTIONS.md) OQ-13; [SIS_INTEGRATION §5](../09-identity/SIS_INTEGRATION.md) | Tối thiểu: quy trình "trường liên hệ phụ huynh" + báo cáo tuần chia sẻ được; chốt OQ-13 trước Phase 5 |
| GD-07 | M | KPI định nghĩa ở 2 nơi với giá trị khác nhau (75% vs 80%), vi phạm R33; RISK-02 còn ghi "chat chỉ mở ở Phase 5" | [PRODUCT_VISION §7](../00-overview/PRODUCT_VISION.md) vs [GAME_DESIGN §16](../01-game-design/GAME_DESIGN.md) | Giữ KPI ở một tài liệu sở hữu; sửa RISK-02/05 |
| GD-08 | L | Giáo viên không tạo được quest, chỉ giao quest có sẵn | [ADR-0027](../adr/ADR-0027-cms-tach-vung-tenant-chi-thao-tac-learning-management-cau-hi.md) | Quest template do giáo viên điền tham số (không đụng cấu hình game) |

### 4.3. An toàn trẻ em, quyền riêng tư, trợ năng, thực tế lớp học

| ID | Mức | Phát hiện | Bằng chứng | Khuyến nghị |
|---|---|---|---|---|
| SAF-01 | C | Cơ sở pháp lý chưa cập nhật và consent chưa được thực thi: chỉ dẫn NĐ 13/2023, không nêu Luật Bảo vệ dữ liệu cá nhân 2025; dùng "Lợi ích hợp pháp" làm cơ sở xử lý; chưa định vai controller/processor; `consent_status` có cột nhưng không có API, không có điểm thực thi | [DATA_PRIVACY §2, §7](../10-security/DATA_PRIVACY.md) (dòng 43); grep `91/2025` = 0 | ADR "Mô hình consent và vai trò dữ liệu" (trường là bên kiểm soát, EduWorld là bên xử lý, riêng analytics EduWorld là bên kiểm soát); `consent_status` là một chiều trong policy resolution, mặc định `unknown` = tắt social; rà soát pháp lý trước Phase 4. **[Ngoài — cần xác minh]** số hiệu và hiệu lực luật |
| SAF-02 | H | Chưa đối chiếu quy định game trực tuyến cho người dưới 18 tuổi (cấp phép, phân loại độ tuổi, giới hạn giờ chơi); không có khung giờ giới nghiêm hay trần thời lượng mặc định | grep `147/2024\|nhãn độ tuổi` = 0; [CHILD_SAFETY §4](../10-security/CHILD_SAFETY.md); OQ-17 mở | Xin ý kiến pháp chế xem EduWorld có thuộc loại game phải cấp phép không; giờ giới nghiêm mặc định ở cấp platform (trường chỉ siết thêm), trần thời lượng/ngày, nhãn độ tuổi. **[Ngoài — cần xác minh]** NĐ 147/2024 |
| SAF-03 | H | Mặc định an toàn còn lỏng cho trẻ 6–8 tuổi: lớp 1 có text tự do, DM, forward, GIF như lớp 9; kết bạn trong trường không giới hạn khối; bước PII tắt được qua công tắc kiểm duyệt (ADR-0030), mâu thuẫn NFR-SAFE-001 | [ADR-0020](../adr/ADR-0020-chat-thong-nhat-cho-moi-lop-moi-tin-nhan-phai-qua-kiem-tra-h.md), [ADR-0030](../adr/ADR-0030-chinh-sach-tu-nhay-cam-trong-chat-danh-muc-quan-ly-duoc-va-c.md), [CHILD_SAFETY §3.3](../10-security/CHILD_SAFETY.md) | Bước PII và grooming luôn chạy, không tắt được; `expires_at` bắt buộc + phê duyệt 2 người; kết bạn chênh ≥ 3 khối cần giáo viên duyệt; preset "chỉ quick message" để trường áp cho lớp 1–3; hoặc mở ADR xem lại ADR-0020 |
| SAF-04 | H | Tenant `common` coi mọi học sinh là "cùng trường": hàng nghìn người lạ, không có trường thu consent, không có giáo viên/giờ truy cập | [MULTITENANCY §8.3](../06-architecture/MULTITENANCY.md) | Với `common`: tắt friend/DM/World chat mặc định, platform tự thu consent, giờ giới nghiêm bắt buộc, moderator platform |
| SAF-05 | H | Kiểm duyệt chỉ có người trực trong giờ học; câu "ngoài giờ, chat lớp 1–5 có thể tự động tắt" là điều kiện theo khối, trái ADR-0020 và R15 | [CHILD_SAFETY](../10-security/CHILD_SAFETY.md) dòng 277; [MODERATION §6.3](../03-social-multiplayer/MODERATION.md) | SLA p1 24/7 cho grooming/tự hại (on-call hoặc tự động hạn chế người gửi); viết lại câu theo hướng giờ chat áp cho mọi học sinh |
| CLS-01 | H | Không có chế độ lớp do giáo viên điều khiển: "chế độ trong giờ học" chỉ có 1 dòng, không có FR/API; không có bắt đầu/dừng/khóa phiên, đẩy cả lớp vào hoạt động, xem ai bị kẹt, trình chiếu | [CHILD_SAFETY](../10-security/CHILD_SAFETY.md) dòng 131; `class_group` mới [Confirmed] ở [PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md) | ADR + FR "Class Session" (module learning): `start/pause/lock/end`, đẩy qua realtime, màn giám sát lớp và trình chiếu trên portal, Timed Quiz do giáo viên dẫn |
| CLS-02 | H | Đăng nhập không thực tế với lớp 1–2: username + mật khẩu ≥ 6 ký tự mỗi lần mở game; QR/mật khẩu hình ảnh nằm ngoài Release 1.0; lockout sau 5 lần sai cho phép bạn khóa tài khoản nhau | [ADR-0063](../adr/ADR-0063-dang-nhap-release-1-0-username-password-va-quy-tac-tenant.md), [ADR-0064](../adr/ADR-0064-khong-tu-dang-nhap-khi-mo-lai-game.md), [KEYCLOAK_SSO §10](../09-identity/KEYCLOAK_SSO.md) | ADR mới đưa "QR lớp + PIN/mật khẩu hình" cho lớp 1–3 vào phạm vi pilot, hoặc giáo viên mở phiên lớp rồi học sinh chọn tên + PIN; lockout theo cặp (username, thiết bị) |
| CLS-03 | M | Ngân sách mạng chỉ tính từng học sinh, không tính tổng cho cả phòng máy (40 × gói 40 MB ≈ 1,6 GB đầu tiết); không có cache LAN, bộ cài offline, xử lý máy có phần mềm đóng băng | [PERFORMANCE_BUDGET §8, §10](../08-game-client/PERFORMANCE_BUDGET.md) | "Classroom budget"; gói nội dung cài sẵn; cache LAN tùy chọn theo hash; chế độ dữ liệu thấp; tải trước ngoài giờ |
| A11Y-01 | M | Trợ năng cho trẻ chưa đọc và học sinh có nhu cầu đặc biệt: TTS tiếng Việt chưa chọn (thường thiếu trên Android rẻ/Web); audio "nếu có"; OQ-15 bỏ ngỏ dân tộc thiểu số/khuyết tật | [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md), [WORLD_DESIGN](../01-game-design/WORLD_DESIGN.md); grep `dyslex\|nhu cầu đặc biệt\|khiếm thính` = 0 | Audio tiếng Việt (ghi sẵn hoặc TTS phía server, cache như media) là điều kiện publish cho nội dung lớp 1–2; font thân thiện người khó đọc; OQ mới cho nhu cầu đặc biệt |
| PRV-01 | M | Thời hạn thông báo vi phạm để TBD; thời điểm DPIA lệch (Phase 5 vs Phase 6); ghi âm giọng trẻ hết hạn chỉ bị ẩn, file vẫn giữ | [SECURITY_ARCHITECTURE](../10-security/SECURITY_ARCHITECTURE.md), [DATA_PRIVACY §2, §8](../10-security/DATA_PRIVACY.md), [ADR-0070](../adr/ADR-0070-release-1-khong-voice-chat-khong-xoa-vat-ly-khong-purge-analytics.md) | Runbook thông báo vi phạm; DPIA trước Phase 4; ngoại lệ xóa vật lý có kiểm soát cho file ghi âm (giống ADR-0033) |

### 4.4. Specs

Số liệu: 364 operation public (identity 79, learning 80, game 87, competition 31, chat 76), 108 message AsyncAPI, 33 JSON Schema, 4/~152 event có payload schema; chỉ 14/295 FR có operation trỏ mã FR ([TRACEABILITY_MATRIX §2](TRACEABILITY_MATRIX.md)).

| ID | Mức | Phát hiện | Bằng chứng | Khuyến nghị |
|---|---|---|---|---|
| SPC-01 | C | Không đặc tả câu trả lời (`response`) theo loại câu: OpenAPI để object mở và trỏ tới `question.schema.json`, nhưng schema này không có `response` | grep `"response"` trong `specs/schemas/question.schema.json` = 0; AsyncAPI `AnswerResponse` là `type: object` | Thêm `$defs.<type>.response` vào `question.schema.json`, `$ref` từ OpenAPI và AsyncAPI |
| SPC-02 | C | Tên loại câu hỏi lệch 3 chiều | `question.schema.json:154` `fill_blank`; `learning.v1.yaml:3575` và AsyncAPI `:639` `fill_in_blank`; `learning-event.schema.json:268` `drag_and_drop` | Một enum duy nhất trong `specs/schemas/common`, mọi nơi `$ref` (bổ sung cho D10 của CODE_PLAN) |
| SPC-03 | H | 6 biến thể enum chính sách phản hồi/hiển thị kết quả (`feedback_policy`, `feedback.timing`, `default_feedback_policy`, `result_visibility`, `result_release.mode`, minigame) — quyết định việc lộ đáp án trước khi nộp (R6) | Xem file schema/OpenAPI tương ứng | Một enum chuẩn và bảng ánh xạ ngữ cảnh |
| SPC-04 | H | Chặn đáp án trong OpenAPI chỉ ở lớp nông: `render_payload` `additionalProperties: true`, `not` chỉ loại `answer_key`, `correct_option_ids` | `learning.v1.yaml` dòng 3543, 3635, 4015 | `$ref` tới phần render đã đóng chặt của `question.schema.json`; test tự động TC-LRN-007 trên mọi loại |
| SPC-05 | H | Assignment lệch [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md): thiếu `course_id`, `late_policy`, `follow_latest`; `content_type` không có `activity` nên không giao được video/tự luận/nói/mô phỏng | `AssignmentCreate` trong `learning.v1.yaml` | Sửa schema theo SCHOOL_LMS §2, §4 |
| SPC-06 | H | Learning event thiếu `activity.graded`/`exam.graded`/sự kiện bù trừ khi regrade; `actor.type` không có giáo viên; `context.mode` thiếu `minigame`, `challenge` | `learning-event.schema.json`; [EVENT_DESIGN §4.3](../07-backend/EVENT_DESIGN.md) | Bổ sung event và enum; payload schema cho `activity.submitted`, `content.published`, `assignment.*` |
| SPC-07 | H | CMS/LMS/Exam thiếu phần lớn endpoint quản trị: CRUD curriculum, question-set, blueprint, lesson; course, path, schedule, gradebook; tạo/quản lý exam session; 46 endpoint có trong API_DESIGN nhưng không có trong OpenAPI | So sánh [API_DESIGN](../07-backend/API_DESIGN.md) §9.13–9.14 với spec | Bổ sung trong Bước 0 của [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) |
| SPC-08 | H | Không có endpoint cho hàng đợi kiểm duyệt, sanction, chat policy, đọc audit log, yêu cầu dữ liệu cá nhân | FR-MOD-003/006/007; grep `/chat/moderation`, `/admin/audit-logs`, `/admin/data-requests` trong spec = 0 | Bổ sung trước Phase 4 (chat gameplay bật) |
| SPC-09 | M | Payload soạn thảo CMS là object mở; rubric lệch giữa OpenAPI và `rubric.schema.json`; thiếu schema activity `quiz`, `interactive_exercise`, `minigame` | `ActivityDefinitionInput.config`, `QuestionAuthoringInput` | `$ref` tới JSON Schema; thống nhất rubric |
| SPC-10 | M | "xAPI-like" nhưng không tương thích xAPI (không có verb/object IRI, duration theo ms); không có QTI/LTI/OneRoster | [EVENT_DESIGN §5.3](../07-backend/EVENT_DESIGN.md) | JSON Schema cho export xAPI Statement + ánh xạ `question_type` → `interactionType`; ghi rõ không hỗ trợ OneRoster/LTI trong 1.0 |
| SPC-11 | L | 0 example ở cấp operation; `specs/postman/` được tham chiếu nhưng chưa có; 21 GET danh sách không có phân trang | — | Bổ sung dần theo luồng |

### 4.5. Công nghệ

| Lựa chọn | Đánh giá | Lý do | Đề xuất |
|---|---|---|---|
| 6 microservice Go + DB riêng + saga ([ADR-0036](../adr/ADR-0036-microservices-theo-he-thong-lon-thay-adr-0001.md)) | **Rủi ro cao** | Bối cảnh ADR chỉ có một câu "Stakeholder quyết định…", không có phương án đã loại, không phân tích năng lực đội; mâu thuẫn Master Prompt ([DOCUMENTATION_AUDIT](DOCUMENTATION_AUDIT.md) C1); đường nóng vẫn gọi đồng bộ (attempt, chấm câu hỏi thi đấu, `principals:resolve`) nên lợi ích cô lập lỗi chỉ trên danh nghĩa; RISK-27…30 | ADR mới: modular monolith có ranh giới cưỡng chế (depguard, mỗi module một schema + role DB riêng, giữ outbox → NATS), 2–3 binary (`core-api`, `realtime`, `worker`); tiêu chí tách service bằng số đo |
| CQRS + mediator kiểu MediatR ([ADR-0038](../adr/ADR-0038-cqrs-clean-architecture-trong-moi-service.md)) | Cân nhắc | Mediator `any` là tư duy .NET, khó đọc trace và mất kiểu trong Go | Giữ tách command/query ở mức package; middleware/decorator có kiểu thay mediator |
| ent + Atlas ([ADR-0039](../adr/ADR-0039-orm-ent-cho-go-tuong-duong-entity-framework-migration-bang-a.md)) | Phù hợp, có điều kiện | Interceptor tenant + soft-delete khả thi | Kiểm tra license các tính năng Atlas cần dùng trên CI **[Ngoài — cần xác minh]**; dự phòng sqlc + goose |
| chi, oapi-codegen, pgx, coder/websocket, failsafe-go | Phù hợp | Thư viện chuẩn, idiomatic | Giữ |
| Keycloak + password grant qua backend ([ADR-0068](../adr/ADR-0068-dang-nhap-game-bang-form-trong-game-qua-backend.md)) | Cân nhắc | Rủi ro bảo mật của ROPC chấp nhận được với client của chính hệ thống; vấn đề thật là tải băm mật khẩu đầu tiết, lockout, trẻ nhỏ gõ mật khẩu, khó thêm SSO Google/Microsoft về sau | Phiên dài ràng buộc thiết bị cá nhân; QR/PIN ở phòng máy; benchmark Keycloak ngay Phase 1; sửa AGENTS R23 (`azp = eduworld-game` → `eduworld-game-backend`) |
| PostgreSQL + `tenant_id` + RLS ([ADR-0005](../adr/ADR-0005-mo-hinh-multi-tenancy-shared-database-tenant-id-rls.md)) | **Phù hợp** | Hai lớp bảo vệ, tương thích PgBouncer transaction mode | Chốt DCS cho Patroni nếu tự vận hành; ưu tiên PostgreSQL managed tại VN |
| NATS JetStream + Valkey Pub/Sub ([ADR-0035](../adr/ADR-0035-duong-truyen-chat-nats-jetstream-valkey-pub-sub-luu-postgres.md), [ADR-0042](../adr/ADR-0042-domain-event-qua-transactional-outbox-nats-jetstream-thay-tr.md)) | Cân nhắc | NATS tốt, nhưng đang có 2 hệ pub/sub (lệnh qua NATS, fan-out qua Valkey) | Fan-out cũng dùng NATS core; Valkey chỉ còn cache + rate limit; gộp một cụm Valkey ở pilot |
| APISIX ([ADR-0009](../adr/ADR-0009-apache-apisix-lam-api-gateway-backend-van-tu-authorize.md)) | Cân nhắc | Mạnh nhưng thêm etcd + instance nội bộ cho 0,5 DevOps | Chấp nhận; phương án nhẹ hơn là Traefik/Caddy + rate limit trong Go |
| Docker Swarm production ([ADR-0024](../adr/ADR-0024-chay-tren-docker-compose-cho-local-docker-swarm-cho-staging.md)) | Rủi ro cao (vận hành) | ~36 VM, HA tự quản cho PostgreSQL/Valkey/NATS/Keycloak/floci, không autoscale; vấn đề là thành phần stateful tự quản, không phải Swarm | Swarm cho stateless; PostgreSQL + object storage dùng dịch vụ managed tại VN; DevOps toàn thời gian là điều kiện bắt buộc |
| floci làm object storage production ([ADR-0071](../adr/ADR-0071-object-storage-production-dung-floci.md)) | **Rủi ro cao — không nên giữ** | Chính ADR ghi nhận là điểm lỗi đơn; RPO media 24 h trong khi bucket `ew-student-submissions` (ghi âm giọng trẻ, bài làm) nằm trên đó; floci tự định vị là công cụ dev/test/CI **[Ngoài — cần xác minh]** | S3 managed tại VN, hoặc Garage/SeaweedFS 3 node nếu tự host; tối thiểu tách bucket bài nộp sang storage có replication. Đổi chỉ cần thay `EW_S3_ENDPOINT` (ADR-0071 mục 4) |
| Godot 4 3D, GDScript ([ADR-0013](../adr/ADR-0013-nen-tang-client-release-1-0-gom-windows-android-ios-web.md), [ADR-0014](../adr/ADR-0014-godot-4-x-voi-gdscript-co-static-typing.md), [ADR-0057](../adr/ADR-0057-chay-tren-may-tam-trung-hang-thiet-bi-va-ngan-sach-hieu-nang.md)) | Phù hợp cho native, **rủi ro cao cho Web** | Web chạy Compatibility + single-thread, giật khi tải vùng, video giải mã bằng CPU; 4 nền tảng × 8 vùng với 3 Godot dev | Ưu tiên Windows (phòng máy) + Android; Web là bản "learning-lite" hoặc chuyển 1.x; video Web qua HTML5 overlay; spike Web trên máy phòng tin thật ở Phase 0/1 |
| Realtime: gateway WS + hub 40 người ([ADR-0011](../adr/ADR-0011-realtime-transport-websocket-qua-go-realtime-server.md), [ADR-0012](../adr/ADR-0012-the-gioi-game-hybrid-hub-instance-private-instance-match-roo.md)) | Phù hợp, có điều kiện | WebSocket đúng cho Web; hub do module competition sở hữu là ranh giới theo tổ chức, không theo runtime | Gộp gateway + room host thành một tiến trình realtime |
| MathJax 4 SVG + MathLive ([ADR-0055](../adr/ADR-0055-cong-thuc-toan-vat-ly-hoa-hoc-mathjax-4-hien-thi-mathlive-nh.md)) | **Phù hợp** | Thực dụng, game nhẹ; thiếu import từ Word/MathType, trong khi giáo viên VN soạn đề chủ yếu bằng Word | Pipeline import .docx (OMML → LaTeX); test rasterizer SVG của Godot với output MathJax |
| React + Ant Design Pro ([ADR-0022](../adr/ADR-0022-cms-lms-web-la-react-typescript-spa.md), [ADR-0047](../adr/ADR-0047-hai-he-thong-giao-dien-game-godot-3d-va-admin-ops-cms-lms.md)) | Phù hợp | Đúng cho CMS dày dữ liệu; đội web 1 người không đủ | 2–3 web dev |
| OTel + Prometheus + Grafana + Loki + Tempo ([ADR-0023](../adr/ADR-0023-observability-bang-opentelemetry-prometheus-grafana-loki-tem.md)) | Cân nhắc | Chuẩn nhưng 6–7 thành phần | Phase 1: Prometheus + Grafana + Loki; thêm Tempo khi cần |
| Analytics trong PostgreSQL + rollup ([ADR-0058](../adr/ADR-0058-game-analytics-tong-hop-trong-postgresql-qua-su-kien-nats.md), [ADR-0062](../adr/ADR-0062-kien-truc-du-lieu-dashboard-khong-truy-van-du-lieu-tho.md)) | **Phù hợp** | Có ngưỡng chuyển ClickHouse rõ ràng | Giữ |
| SIS Contract + CSV ([ADR-0021](../adr/ADR-0021-sis-integration-theo-contract-do-eduworld-so-huu.md)) | Cân nhắc | Hợp lý về kiến trúc, nhưng hệ thống quản lý trường phổ biến tại VN hầu như không mở API cho bên thứ ba **[Ngoài — cần xác minh]**, nên CSV/Excel sẽ là kênh chính | Coi import Excel theo mẫu xuất của hệ thống trường đang dùng là đường chính; đầu tư UX đối soát |
| Chuẩn liên thông (LTI 1.3, OneRoster, QTI, xAPI, SCORM) | Cân nhắc | Không có trong specs; ở K-9 công lập VN hầu như chưa dùng, nên không chặn 1.0 | Export xAPI đúng chuẩn (SPC-10); LTI 1.3 tool provider ở 1.x |
| Offline / băng thông thấp | **Rủi ro cao** | Không làm bài offline, không cache tại trường | Xem CLS-03 |

## 5. Quyết định cần đưa lên stakeholder (ADR mới đề xuất)

| # | Đề xuất ADR | Thay/ảnh hưởng | Liên quan |
|---|---|---|---|
| 1 | Release theo pilot ưu tiên giáo dục thay cho "1.0 đủ mọi thứ" | ROADMAP §1, PRODUCT_VISION §8 | K3, GD-01 |
| 2 | Modular monolith có ranh giới cưỡng chế | Supersede ADR-0036, một phần ADR-0037, ADR-0040 | K6 |
| 3 | Object storage production không dùng floci | Supersede ADR-0071 | K6 |
| 4 | Đăng nhập lớp 1–3 bằng QR lớp + PIN/mật khẩu hình; Class Session do giáo viên điều khiển | Một phần ADR-0063, ADR-0064 | K5, CLS-01, CLS-02 |
| 5 | Mô hình consent và vai trò dữ liệu; tuân thủ quy định game cho người dưới 18 tuổi | DATA_PRIVACY, CHILD_SAFETY | SAF-01, SAF-02 |
| 6 | Mặc định an toàn cho trẻ nhỏ (PII luôn bật, preset quick-message, tenant `common`) | Có thể xem lại ADR-0020, ADR-0030 | SAF-03, SAF-04 |
| 7 | Phạm vi nền tảng Release đầu: Windows + Android; Web lite/iOS sau | Một phần ADR-0013 | Công nghệ |
| 8 | Kế hoạch đo hiệu quả học tập là exit criterion của pilot | ROADMAP, PRODUCT_VISION §7 | K1 |

## 6. Phạm vi pilot gợi ý nếu ưu tiên giáo dục [Confirmed]

1. **Pilot:** Learning Engine 6–8 loại câu hỏi; LMS giao bài + dashboard + heatmap; Class Session; Toán và Tiếng Việt lớp 1–5; quest `main/learning/daily/weekly` + boss solo; 4–5 mini-game, trong đó có 2 cơ chế lấy kỹ năng làm cách chơi; XP, level, shop cosmetic; Timed Quiz do giáo viên dẫn; một kiểu co-op có phụ thuộc lẫn nhau; chỉ quick message + emote (không chat tự do); Windows + Android; nghiên cứu hiệu quả có test trước/sau.
2. **Sau khi có bằng chứng pilot:** 1v1 + bot, Team Battle, Glicko-2, leaderboard cấp trường, chat party/match có kiểm duyệt, thêm môn/khối, Web.
3. **Về sau:** leaderboard Global, các định dạng giải còn lại, đủ 8 channel + DM, club, season/event, iOS, chứng minh 10k CCU.

## 7. Thứ tự xử lý

| Khi nào | Việc |
|---|---|
| Trước khi code (Bước 0 của [CODE_PLAN](CODE_PLAN_CMS_LMS_FIRST.md)) | SPC-01…07, SPC-09; D1–D17; quyết định ADR #2, #3 (ảnh hưởng toàn bộ cấu trúc repo và hạ tầng) |
| Trước pilot | K1 (efficacy plan), K2 (mô hình nội dung, chốt OQ-08), ADR #1, #4, #7; PED-04, PED-05, PED-06, PED-07; CLS-03; A11Y-01 |
| Trước Phase 4 (bật chat gameplay) | SAF-01…05, SPC-08, PRV-01, ADR #5, #6 |
| Theo dữ liệu thật | PED-08 (BKT/IRT), PED-11, SPC-10, chuẩn liên thông |

## 8. Acceptance criteria của review

- Mỗi phát hiện C/H có quyết định: chấp nhận (tạo ADR/sửa tài liệu sở hữu, ghi CHANGELOG), hoặc từ chối có lý do ghi vào bảng này.
- Các mục **[Ngoài — cần xác minh]** được pháp chế (luật) hoặc spike kỹ thuật (floci, Atlas, Godot Web) xác nhận trước khi dùng làm căn cứ ADR.

## 9. Trạng thái xử lý

> Cập nhật: 2026-11-15, đợt sửa tài liệu theo ADR-0072; đợt 2 theo ADR-0081 (`user_type`) và ADR-0082 (pháp lý, viết/nói Deferred). Cột "Xử lý" ghi nơi quyết định/thay đổi được sở hữu; "Sửa tài liệu [Confirmed]" nghĩa là đã đưa vào tài liệu sở hữu với nhãn [Confirmed], chờ người duyệt. **Đợt 3 (2026-11-17, Technical Architect):** ADR-0073, ADR-0075, ADR-0078, ADR-0080 Accepted; ADR-0076, ADR-0079 Rejected (giữ 4 nền tảng theo ADR-0013); ADR-0077 hoãn (ADR-0082). CI chuyển sang GitHub Actions (ADR-0088). **Đợt 4 (2026-11-17, Technical Architect):** ADR-0074 Accepted (modular monolith thay ADR-0036), tài liệu vận hành, dữ liệu, định danh, bảo mật và dự án đã viết lại theo ADR-0074.

**Xử lý tổng quát**

| Loại | Ý nghĩa |
|---|---|
| ADR-0072 | Giải quyết hoặc làm thay đổi bản chất phát hiện bởi quyết định bỏ vai trò giáo viên, học sinh tự học, kiểm duyệt tập trung |
| Sửa tài liệu [Confirmed] | Bổ sung vào tài liệu sở hữu theo khuyến nghị, gắn [Confirmed] |
| ADR Proposed | Quyết định lớn đã viết thành ADR đề xuất, chờ stakeholder |
| ADR Accepted (2026-11-17) | Đã quyết định và áp vào tài liệu sở hữu |
| Một phần | Đã xử lý một phần, phần còn lại ghi ở cột Ghi chú |
| Không còn áp dụng | Phát hiện dựa trên vai trò giáo viên, không còn ý nghĩa sau ADR-0072 |
| Còn mở | Chưa có quyết định hoặc chưa có tài liệu xử lý |

**Vấn đề sống còn (§2)**

| ID | Xử lý | Ghi chú |
|---|---|---|
| K1 | ADR Accepted (2026-11-17) — [ADR-0080](../adr/ADR-0080-do-hieu-qua-hoc-tap-la-exit-criterion-pilot.md) | Cổng M-P2 ở ROADMAP §3.5, KPI ở PRODUCT_VISION §7; hạ tầng nghiên cứu là F035 (chưa viết spec); ngưỡng đạt [TBD] do PO + EdTech chốt trước pilot |
| K2 | **Đã xử lý** — OQ-08 Answered (owner 2026-11-20) | Pilot Toán + Tiếng Việt lớp 1–5, khối lượng theo mô hình throughput [CONTENT_PRODUCTION_PLAN §3.4](../02-learning/CONTENT_PRODUCTION_PLAN.md#34-ví-dụ-pilot-toán--tiếng-việt-lớp-15); thiếu throughput thì cắt khối |
| K3 | **Một phần** — ADR Accepted (2026-11-17) [ADR-0073](../adr/ADR-0073-phat-hanh-theo-pilot-uu-tien-giao-duc.md) | ROADMAP §1–§3 và PRODUCT_VISION §8 viết lại theo Pilot → Mở rộng → Hoàn thiện, bỏ "không MVP"; còn lại: pilot vẫn làm đủ 4 nền tảng (ADR-0013, ADR-0079 Rejected) nên tải client chưa giảm (RISK-54); CODE_PLAN §5 cần ánh xạ feature theo lát cắt pilot |
| K4 | Sửa tài liệu [Confirmed] — Bước 0 của [CODE_PLAN](CODE_PLAN_CMS_LMS_FIRST.md) (S1, S2) | Enum chung `specs/schemas/common`, response schema theo loại câu; TC-LRN-016…017 |
| K5 | ADR-0072 + CLS-02 + CLS-03 | "Chế độ lớp do giáo viên điều khiển" không còn áp dụng; không làm đăng nhập QR/PIN (ADR-0076 Rejected), đăng nhập lớp 1–3 xử lý theo CLS-02; băng thông phòng máy ở CLS-03 |
| K6 | **Đã xử lý** — ADR Accepted (2026-11-17): [ADR-0074](../adr/ADR-0074-modular-monolith-co-ranh-gioi-cuong-che.md) và [ADR-0075](../adr/ADR-0075-object-storage-production-khong-dung-floci.md) | Modular monolith thay 6 microservice (ADR-0036 Superseded): 3 binary `core-api`/`realtime`/`worker` + profile, một cụm PostgreSQL 5 database, một pipeline Go ma trận module; sizing giảm từ 34 xuống 30 node Release 1.0 và 9 xuống 8 VM pilot ([DEPLOYMENT](../11-operations/DEPLOYMENT.md) §3.1, §4), giả định đội 3 Go + 0,5 DevOps ([ROADMAP §1](ROADMAP.md)); RISK-27 đóng, RISK-55/56 mới. Object storage staging/production không dùng floci (thay ADR-0071; DEPLOYMENT §9b, BACKUP_RECOVERY §6, chọn backend sau spike) |

**Phát hiện theo chiều (§4)**

| ID | Xử lý | Ghi chú |
|---|---|---|
| PED-01 | ADR Accepted — ADR-0080 | Cùng K1; RISK-47, RISK-53 |
| PED-02 | **Đã xử lý** — OQ-08 Answered (owner 2026-11-20) | CONTENT_PRODUCTION_PLAN §2–§4; RISK-06 vẫn theo dõi (năng lực đội) |
| PED-03 | **Đã có tài liệu** [Confirmed] — [LEARNING_MECHANICS](../01-game-design/LEARNING_MECHANICS.md) | Cơ chế học tập nội tại (hành động chính = khái niệm học); pilot yêu cầu ≥ 2 cơ chế "kỹ năng là cách chơi" (ADR-0073, ROADMAP §3.4); chờ Game Director + EdTech duyệt |
| PED-04 | Sửa tài liệu [Confirmed] — ACTIVITY_ENGINE, QUESTION_BANK, LEARNING_ANALYTICS | `hints:next`, `hints_used`, `w_hint`, `options[].feedback` chỉ trong kết quả chấm, `worked_example`; TC-LRN-011…012 |
| PED-05 | Sửa tài liệu [Confirmed] — ACTIVITY_ENGINE, LEARNING_ANALYTICS | `rapid_guess`, hệ số đoán trúng, câu "anh em" cho `retry_wrong`; báo cho nhà trường thay vì giáo viên; TC-LRN-013 |
| PED-06 | Sửa tài liệu [Confirmed] — CURRICULUM_MODEL | `yccd_ref`, `competency_ids[]`, báo cáo tham khảo TT27/TT22; TC-LRN-018 |
| PED-07 | **Đã có tài liệu** [Confirmed] — CONTENT_MODEL, LEARNING_EXPERIENCE, [LEARNING_MECHANICS](../01-game-design/LEARNING_MECHANICS.md) | Lồng tiếng `g1_3` là điều kiện publish (miễn trừ cho câu đánh vần/chính tả); TC-CNT-010; cơ chế `LM-VIE-01` ghép âm tiết cho lớp 1–2 |
| PED-08 | Sửa tài liệu [Confirmed] — ACTIVITY_ENGINE, LEARNING_ANALYTICS, QUESTION_BANK | `AttemptMode = diagnostic`, hiệu chỉnh độ khó bán tự động, interleaving; TC-LRN-014; BKT/IRT theo dữ liệu thật |
| PED-09 | Sửa tài liệu [Confirmed] — PROGRESSION_SYSTEM, LEARNING_ANALYTICS | Miễn giảm XP cho ôn LO tiên quyết do hệ thống/nhà trường giao; giảm trọng số `timed_challenge` |
| PED-10 | Không còn áp dụng (ADR-0072); viết/nói **Deferred (ADR-0082)** | Không còn chấm tay, rubric, hàng đợi chấm hay nguồn evidence từ giáo viên; thiết kế chuyển `docs/99-deferred/`, schema chuyển `specs/_deferred/` |
| PED-11 | Sửa tài liệu [Confirmed] — EXAM_ENGINE | Cân bằng biến thể đề, báo cáo độ tin cậy |
| GD-01 | ADR Accepted — ADR-0073 | ROADMAP theo giai đoạn: pilot sau M-C3 School Loop, cổng hiệu quả M-P2; RISK-05 đã sửa |
| GD-02 | Sửa tài liệu [Confirmed] — QUEST_SYSTEM, PARTY_SYSTEM | Bước `interdependent` có `role` |
| GD-03 | Sửa tài liệu [Confirmed] — COMPETITION_SYSTEM | Bảng lớp theo nỗ lực/tiến bộ hoặc mục tiêu tập thể; RISK-12 đã sửa (leaderboard chỉ platform cấu hình); không có bật/tắt theo giáo viên |
| GD-04 | Sửa tài liệu [Confirmed] — COMPETITION_SYSTEM | Bỏ bonus tốc độ 1v1 cho lớp 1–5 |
| GD-05 | Sửa tài liệu [Confirmed] — PROGRESSION_SYSTEM | Quy tắc chuyển năm học |
| GD-06 | ADR-0072 + OQ-13 Answered (owner 2026-11-20) | Liên hệ phụ huynh qua nhà trường khi có vụ việc (ADR-0072 mục 4); không có parent portal trong Release 1.0 |
| GD-07 | Sửa tài liệu [Confirmed] | KPI chỉ định nghĩa ở PRODUCT_VISION §7; RISK-02 đã sửa (chat gameplay từ Phase 4) |
| GD-08 | Không còn áp dụng (ADR-0072) | Không có giáo viên tạo quest; quest template từ nhà trường không thuộc phạm vi |
| SAF-01 | **Deferred (ADR-0082)** — ADR-0077 hoãn | Cơ sở pháp lý, vai trò dữ liệu, consent chuyển [LEGAL_COMPLIANCE](../99-deferred/LEGAL_COMPLIANCE.md); bỏ `consent_status` khỏi DATABASE_DESIGN; OQ-12 hoãn; rủi ro chấp nhận (RISK-03, RISK-50). Nguyên tắc kỹ thuật giữ ở DATA_PRIVACY |
| SAF-02 | Phần pháp lý **Deferred (ADR-0082)**; phần kỹ thuật **Đã xử lý** — [ADR-0078](../adr/ADR-0078-mac-dinh-an-toan-cho-tre-nho.md) Accepted | OQ-24 hoãn; đối chiếu NĐ 147/2024 chuyển LEGAL_COMPLIANCE §10; giờ giới nghiêm platform 21:30–06:30 ở CHILD_SAFETY §4.1; trần thời lượng/ngày vẫn do trường tự bật |
| SAF-03 | **Đã xử lý** — ADR-0078 Accepted | PII/grooming luôn chạy, công tắc có thời hạn ≤ 24 giờ + hai người duyệt (MODERATION §12), preset "chỉ quick message + emote" cấp trường (CHILD_SAFETY §3.5); không có quy tắc theo khối (R15) |
| SAF-04 | **Đã xử lý** — ADR-0078 Accepted | Tenant `common`: tắt kết bạn, DM, World chat, giờ giới nghiêm bắt buộc, Safety Team kiểm duyệt (CHILD_SAFETY §3.6, MULTITENANCY §8.3, SOCIAL_SYSTEM §7) |
| SAF-05 | Sửa tài liệu [Confirmed] — CHILD_SAFETY, MODERATION | Bỏ câu điều kiện theo khối; on-call 24/7 cho case p1; RISK-48 |
| CLS-01 | Không còn áp dụng (ADR-0072) | Không có Class Session do giáo viên điều khiển; "chế độ trong giờ học" thay bằng khung giờ truy cập cấp trường do `school_admin` đặt |
| CLS-02 | **Không làm QR** (ADR-0076 Rejected 2026-11-17); một phần đã xử lý — [ADR-0086](../adr/ADR-0086-giu-phien-game-bang-refresh-token.md) | Không có thẻ QR/PIN/mật khẩu hình; thiết bị cá nhân vào lại bằng refresh token qua màn "Tiếp tục là <nickname>?"; Windows mặc định là máy dùng chung nên phòng máy vẫn nhập username/mật khẩu ngắn do trường in phát; rủi ro còn lại ở RISK-10 |
| CLS-03 | Sửa tài liệu [Confirmed] — PERFORMANCE_BUDGET | Ngân sách băng thông phòng máy, cache LAN, gói cài sẵn, tải trước ngoài giờ |
| A11Y-01 | **Đã xử lý** — OQ-23 Answered (owner 2026-11-20): lồng tiếng giọng người bắt buộc cho `g1_3`, không TTS phía server, TTS thiết bị chỉ dự phòng ([CONTENT_MODEL §8.1](../02-learning/CONTENT_MODEL.md#81-audio-đọc-đề-và-hướng-dẫn-cho-nhóm-g1_3-confirmed)) | Phần lồng tiếng `g1_3` xử lý cùng PED-07; không phụ thuộc phần deferred (lồng tiếng là audio do người soạn cung cấp, không ghi âm học sinh) |
| PRV-01 | **Deferred (ADR-0082)** | DPIA và thời hạn thông báo vi phạm theo luật chuyển LEGAL_COMPLIANCE §6–§7; bỏ khỏi exit criteria (ROADMAP, DEPLOYMENT, CHILD_SAFETY CS-07); ghi âm giọng nói thuộc SPEAKING_ASSESSMENT (deferred) |
| SPC-01…04 | Sửa tài liệu [Confirmed] — Bước 0 CODE_PLAN (S1–S4) | TC-LRN-007, TC-LRN-016…017 |
| SPC-05 | Sửa tài liệu [Confirmed] — Bước 0 CODE_PLAN (S5) | Giao `activity` cho video/âm thanh/mô phỏng; tự luận/nói Deferred (ADR-0082): schema `written-response`, `speaking`, `project`, `rubric` chuyển `specs/_deferred/`, `$defs outOfRelease*` bỏ khỏi `activity-base`, `check-enums` chặn giá trị deferred |
| SPC-06 | Sửa tài liệu [Confirmed] — Bước 0 CODE_PLAN (S2, S5) | `actor.type` = `student, system, staff` (không có giáo viên); envelope domain event bỏ `admin` (ADR-0081) |
| `user_type` (ADR-0081) | Đã sửa | `user_type` ∈ `student, staff` ở OpenAPI, internal spec, envelope, DATABASE_DESIGN (CHECK + bất biến `primary_tenant_id` ↔ role scope), BACKEND_DESIGN (`portal_staff`), GLOSSARY, 09-identity, ADMIN_PORTAL; PRD FR-AUTH-022 cần sửa riêng |
| SPC-07 | Sửa tài liệu [Confirmed] — Bước 0 CODE_PLAN (S6) | |
| SPC-08 | Còn mở | Bắt buộc trước Phase 4; hàng đợi case của `safety_moderator` |
| SPC-09 | Còn mở (rubric Deferred, ADR-0082) | Schema activity `quiz`, `interactive_exercise`, `minigame` vẫn cần |
| SPC-10, SPC-11 | Còn mở | Theo dữ liệu thật / bổ sung dần |
| Công nghệ — AGENTS R23 | Đã sửa | `azp = eduworld-game-backend` |
| Công nghệ — microservice, floci, Godot Web | floci: **Đã xử lý** (ADR-0075 Accepted); Godot Web: giữ Web từ pilot ([ADR-0079](../adr/ADR-0079-nen-tang-ban-dau-windows-va-android.md) Rejected, ADR-0013); microservice: **Đã xử lý** (ADR-0074 Accepted) | RISK-52, RISK-54, RISK-55 |
| Công nghệ — mediator CQRS (§4.5) | Đã xử lý — [ADR-0087](../adr/ADR-0087-cqrs-tach-read-write-khong-mediator.md) | Bỏ `src/backend/pkg/mediator`; CQRS tách write/read context, cross-cutting qua middleware `src/backend/pkg/httpx` + gọi tường minh; đã sửa AGENTS R26, constitution II, BACKEND_DESIGN, F003 |
| Công nghệ — các mục còn lại (§4.5) | **Đã chốt** (owner 2026-11-20) | Giữ quyết định hiện hành: APISIX (ADR-0009), observability (ADR-0023), NATS JetStream + Valkey Pub/Sub (ADR-0035, ADR-0042), ent + Atlas (ADR-0039) — đều Accepted; PostgreSQL/Valkey/NATS tự vận hành trên VM cloud tại VN, chỉ object storage dùng managed (OQ-10, OQ-21); import .docx theo [CONTENT_PRODUCTION_PLAN §8](../02-learning/CONTENT_PRODUCTION_PLAN.md#8-import-wordmathtype-confirmed). Kiểm tra license tính năng Atlas trên CI và nhập SIS bằng Excel là việc triển khai, không còn là quyết định mở |

**Đề xuất ADR ở §5**

| # | ADR | Ghi chú |
|---|---|---|
| 1 | ADR-0073 — **Accepted** 2026-11-17 | Nền tảng theo ADR-0013, không theo ADR-0079 |
| 2 | ADR-0074 — **Accepted** 2026-11-17 | Modular monolith, 3 binary + profile worker, database theo module trên một cụm; trả lời OQ-10 |
| 3 | ADR-0075 — **Accepted** 2026-11-17 | Thay toàn phần ADR-0071 |
| 4 | ADR-0076 — **Rejected** 2026-11-17 | Không làm thẻ QR + PIN/mật khẩu hình |
| 5 | ADR-0077 (Proposed) | Hoãn (ADR-0082) |
| 6 | ADR-0078 — **Accepted** 2026-11-17 | Bổ sung: Windows mặc định máy dùng chung, màn "Tiếp tục là…" (cùng ADR-0086) |
| 7 | ADR-0079 — **Rejected** 2026-11-17 | Giữ 4 nền tảng (ADR-0013) |
| 8 | ADR-0080 — **Accepted** 2026-11-17 | F035 viết riêng |

Phạm vi pilot gợi ý ở §6 còn nhắc Class Session và Timed Quiz do giáo viên dẫn; hai mục này không còn áp dụng sau ADR-0072.
