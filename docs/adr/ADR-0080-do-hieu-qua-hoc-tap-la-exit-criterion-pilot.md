# ADR-0080 — Đo hiệu quả học tập là exit criterion của pilot

> **Trạng thái:** Accepted · **Ngày:** 2026-11-15 · **OQ:** — · **Tài liệu:** [LEARNING_EFFICACY_PLAN](../02-learning/LEARNING_EFFICACY_PLAN.md) · [PRODUCT_VISION](../00-overview/PRODUCT_VISION.md) · [ROADMAP](../12-project/ROADMAP.md) · [EDU_FIT_REVIEW](../12-project/EDU_FIT_REVIEW.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

KPI "mastery gain" trong [PRODUCT_VISION §7](../00-overview/PRODUCT_VISION.md) được đo bằng chính mô hình mastery của hệ thống, nên không chứng minh được học sinh tiến bộ thật. Exit criteria hiện tại không có cổng nào về hiệu quả học tập. Không còn giáo viên trong hệ thống (ADR-0072), nên chất lượng gợi ý tự động là yếu tố quyết định và phải được đo độc lập ([EDU_FIT_REVIEW](../12-project/EDU_FIT_REVIEW.md) PED-01, K1).

## Quyết định

1. Pilot (ADR-0073) chỉ được coi là đạt khi có báo cáo nghiên cứu hiệu quả theo [LEARNING_EFFICACY_PLAN](../02-learning/LEARNING_EFFICACY_PLAN.md): bài kiểm tra độc lập trước/sau (tự chấm, không dùng câu hỏi luyện tập), có lớp so sánh, thời gian tối thiểu 8 tuần [Proposed].
2. Ngưỡng đạt (effect size, tỉ lệ học sinh tiến bộ) do PO + EdTech chốt trước khi pilot bắt đầu [TBD].
3. Báo cáo gồm đối chiếu mastery với kết quả kiểm tra độc lập (độ tin cậy dự báo); kết quả dùng để hiệu chỉnh tham số mastery (ADR-0017).
4. Dữ liệu nghiên cứu dùng định danh giả và tối thiểu hóa theo [DATA_PRIVACY](../10-security/DATA_PRIVACY.md); nhà trường tham gia bằng thỏa thuận pilot và có quyền rút lớp/học sinh khỏi phần phân tích. Phần pháp lý (consent, ADR-0077) hoãn theo ADR-0082.

## Hệ quả

(+) Có bằng chứng cho nhà trường và cơ sở điều chỉnh sản phẩm. (−) Cần phối hợp trường pilot, thiết kế đề kiểm tra độc lập, thời gian chờ kết quả.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR (Proposed) từ EDU_FIT_REVIEW | Technical Architect |
| 2026-11-17 | Accepted | Technical Architect |
