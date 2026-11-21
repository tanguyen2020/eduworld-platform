# ADR-0080 — Đo hiệu quả học tập là exit criterion của pilot

> **Trạng thái:** Accepted · một phần superseded by ADR-0095 · **Ngày:** 2026-11-15 · **OQ:** — · **Tài liệu:** [LEARNING_EFFICACY_PLAN](../02-learning/LEARNING_EFFICACY_PLAN.md) · [PRODUCT_VISION](../00-overview/PRODUCT_VISION.md) · [ROADMAP](../12-project/ROADMAP.md) · [EDU_FIT_REVIEW](../12-project/EDU_FIT_REVIEW.md) · [Chỉ mục ADR](README.md)

> [!WARNING]
> **Một phần đã được thay bởi [ADR-0095](ADR-0095-bo-khai-niem-lop-hoc-sinh-theo-khoi.md):** Không còn hiệu lực: đơn vị nghiên cứu "lớp" — "lớp so sánh" (mục 1), "rút lớp/học sinh" (mục 4), "đơn vị mặc định là lớp" (mục 5). **Còn hiệu lực:** toàn bộ phần còn lại; đơn vị là **cụm (trường, khối)**, ghép cặp cụm cùng khối giữa các trường, rút theo cụm hoặc học sinh, `experiments.unit` ∈ {`student`, `school_grade`}.

## Bối cảnh

KPI "mastery gain" trong [PRODUCT_VISION §7](../00-overview/PRODUCT_VISION.md) được đo bằng chính mô hình mastery của hệ thống, nên không chứng minh được học sinh tiến bộ thật. Exit criteria hiện tại không có cổng nào về hiệu quả học tập. Không còn giáo viên trong hệ thống (ADR-0072), nên chất lượng gợi ý tự động là yếu tố quyết định và phải được đo độc lập ([EDU_FIT_REVIEW](../12-project/EDU_FIT_REVIEW.md) PED-01, K1).

## Quyết định

1. Pilot (ADR-0073) chỉ được coi là đạt khi có báo cáo nghiên cứu hiệu quả theo [LEARNING_EFFICACY_PLAN](../02-learning/LEARNING_EFFICACY_PLAN.md): bài kiểm tra độc lập trước/sau (tự chấm, không dùng câu hỏi luyện tập), có lớp so sánh, thời gian tối thiểu 8 tuần [Confirmed].
2. Ngưỡng đạt [Confirmed — owner 2026-11-20, theo đề xuất của LEARNING_EFFICACY_PLAN §4]: Hedges' g ≥ 0,2 (mô hình hồi quy đa mức, điểm trước làm biến kiểm soát) với khoảng tin cậy 95% nằm trên 0, tỉ lệ rời nghiên cứu < 20%, tối thiểu 8 tuần; tỉ lệ học sinh tiến bộ báo cáo kèm, không là ngưỡng. Không đạt thì pilot chỉ qua cổng khi có kế hoạch khắc phục được Product Owner chấp nhận.
3. Báo cáo gồm đối chiếu mastery với kết quả kiểm tra độc lập (độ tin cậy dự báo); kết quả dùng để hiệu chỉnh tham số mastery (ADR-0017).
4. Dữ liệu nghiên cứu dùng định danh giả và tối thiểu hóa theo [DATA_PRIVACY](../10-security/DATA_PRIVACY.md); nhà trường tham gia bằng thỏa thuận pilot và có quyền rút lớp/học sinh khỏi phần phân tích. . Hạ tầng thử nghiệm A/B cho mô hình mastery và luật gợi ý (LEARNING_EFFICACY_PLAN §6) [Confirmed — owner 2026-11-20, bổ sung theo F035]: bảng `learning.experiments` (kind `mastery_model`, `recommendation_rule`; `selection_rule` chưa xếp feature), phân nhánh bằng hash ổn định `(experiment_id, unit_id) mod 100` với đơn vị mặc định là lớp và lưu ở lần tiếp xúc đầu; arm không phải control chạy shadow (tính song song, không hiển thị) trước khi bật live; ở live chỉ trải nghiệm phía học sinh đổi theo arm, báo cáo nhà trường, mức tham khảo, XP và `learning_objective.mastered` giữ mô hình `active`; không thử nghiệm trên attempt `exam`; tenant phải có thỏa thuận phủ `experiment`; learning event mang `context.experiment_arms`, không response nào cho học sinh chứa arm; rào chắn dừng sớm khi arm giảm chỉ số chính từ `experiment_guardrail_drop_pp` trở lên.
6. Bài kiểm tra trước/sau là ExamSession của F042 có `efficacy_wave_id`: không phát `exam.*` ra outbox, không evidence mastery, không công bố kết quả; learning event mang `context.efficacy_wave_id` và mọi consumer/recompute bỏ qua (phòng thủ hai lớp).

## Hệ quả

(+) Có bằng chứng cho nhà trường và cơ sở điều chỉnh sản phẩm. (−) Cần phối hợp trường pilot, thiết kế đề kiểm tra độc lập, thời gian chờ kết quả.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR (Proposed) từ EDU_FIT_REVIEW | Technical Architect |
| 2026-11-17 | Accepted | Technical Architect |
| 2026-11-20 | Chốt ngưỡng đạt mục 2 theo đề xuất của LEARNING_EFFICACY_PLAN §4 | Owner |
| 2026-11-21 | Bổ sung mục 5 (hạ tầng A/B, câu hỏi mở 1 của F035 đã chốt 2026-11-20) và mục 6 (đợt kiểm tra không phát `exam.*`) | F035 T008 |
| 2026-11-21 | Một phần superseded by ADR-0095 (bỏ khái niệm lớp — học sinh theo khối) | Owner |
