# ADR-0080 — Đo hiệu quả học tập là exit criterion của pilot

> **Trạng thái:** Accepted · **Ngày:** 2026-11-15 · **OQ:** — · **Tài liệu:** [LEARNING_EFFICACY_PLAN](../02-learning/LEARNING_EFFICACY_PLAN.md) · [PRODUCT_VISION](../00-overview/PRODUCT_VISION.md) · [ROADMAP](../12-project/ROADMAP.md) · [EDU_FIT_REVIEW](../12-project/EDU_FIT_REVIEW.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

KPI "mastery gain" trong [PRODUCT_VISION §7](../00-overview/PRODUCT_VISION.md) được đo bằng chính mô hình mastery của hệ thống, nên không chứng minh được học sinh tiến bộ thật. Exit criteria hiện tại không có cổng nào về hiệu quả học tập. Không còn giáo viên trong hệ thống (ADR-0072), nên chất lượng gợi ý tự động là yếu tố quyết định và phải được đo độc lập ([EDU_FIT_REVIEW](../12-project/EDU_FIT_REVIEW.md) PED-01, K1).

## Quyết định

1. Pilot (ADR-0073) chỉ được coi là đạt khi có báo cáo nghiên cứu hiệu quả theo [LEARNING_EFFICACY_PLAN](../02-learning/LEARNING_EFFICACY_PLAN.md): bài kiểm tra độc lập trước/sau (tự chấm, không dùng câu hỏi luyện tập), có lớp so sánh, thời gian tối thiểu 8 tuần [Confirmed].
2. Ngưỡng đạt [Confirmed — owner 2026-11-20, theo đề xuất của LEARNING_EFFICACY_PLAN §4]: Hedges' g ≥ 0,2 (mô hình hồi quy đa mức, điểm trước làm biến kiểm soát) với khoảng tin cậy 95% nằm trên 0, tỉ lệ rời nghiên cứu < 20%, tối thiểu 8 tuần; tỉ lệ học sinh tiến bộ báo cáo kèm, không là ngưỡng. Không đạt thì pilot chỉ qua cổng khi có kế hoạch khắc phục được Product Owner chấp nhận.
3. Báo cáo gồm đối chiếu mastery với kết quả kiểm tra độc lập (độ tin cậy dự báo); kết quả dùng để hiệu chỉnh tham số mastery (ADR-0017).
4. Dữ liệu nghiên cứu dùng định danh giả và tối thiểu hóa theo [DATA_PRIVACY](../10-security/DATA_PRIVACY.md); nhà trường tham gia bằng thỏa thuận pilot và có quyền rút lớp/học sinh khỏi phần phân tích. Phần pháp lý (consent, ADR-0077) hoãn theo ADR-0082.

## Hệ quả

(+) Có bằng chứng cho nhà trường và cơ sở điều chỉnh sản phẩm. (−) Cần phối hợp trường pilot, thiết kế đề kiểm tra độc lập, thời gian chờ kết quả.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR (Proposed) từ EDU_FIT_REVIEW | Technical Architect |
| 2026-11-17 | Accepted | Technical Architect |
| 2026-11-20 | Chốt ngưỡng đạt mục 2 theo đề xuất của LEARNING_EFFICACY_PLAN §4 | Owner |
