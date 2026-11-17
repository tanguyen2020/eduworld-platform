# ADR-0077 — Mô hình consent, vai trò xử lý dữ liệu và tuân thủ quy định game cho người dưới 18 tuổi

> **Trạng thái:** Proposed · hoãn — ngoài version này (ADR-0082) · **Ngày:** 2026-11-15 · **OQ:** OQ-12, OQ-17 · **Tài liệu:** [DATA_PRIVACY](../10-security/DATA_PRIVACY.md) · [CHILD_SAFETY](../10-security/CHILD_SAFETY.md) · [EDU_FIT_REVIEW](../12-project/EDU_FIT_REVIEW.md) · [Chỉ mục ADR](README.md)

> [!WARNING]
> **Hoãn theo [ADR-0082](ADR-0082-spec-driven-development-va-noi-dung-deferred.md):** pháp lý không có trong version này; thiết kế liên quan nằm ở [docs/99-deferred/LEGAL_COMPLIANCE.md](../99-deferred/LEGAL_COMPLIANCE.md). Không triển khai.

## Bối cảnh

[DATA_PRIVACY](../10-security/DATA_PRIVACY.md) dẫn chiếu Nghị định 13/2023/NĐ-CP, chưa nêu Luật Bảo vệ dữ liệu cá nhân 2025 [Ngoài — cần pháp chế xác nhận số hiệu và hiệu lực]. Một số dòng xử lý dùng cơ sở "lợi ích hợp pháp". Vai trò bên kiểm soát/bên xử lý giữa trường và EduWorld chưa được định. Có cột `consent_status` nhưng không có API và không có điểm thực thi. Quy định về trò chơi điện tử trực tuyến cho người dưới 18 tuổi (cấp phép, phân loại độ tuổi, giới hạn thời gian chơi) chưa được đối chiếu [Ngoài — cần pháp chế xác nhận, ví dụ Nghị định 147/2024/NĐ-CP] ([EDU_FIT_REVIEW](../12-project/EDU_FIT_REVIEW.md) SAF-01, SAF-02).

## Quyết định (đề xuất)

1. **Vai trò:** trường (tenant) là bên kiểm soát dữ liệu học sinh phục vụ học tập; EduWorld là bên xử lý theo hợp đồng. Với Game Analytics và cải tiến sản phẩm, EduWorld là bên kiểm soát và chỉ dùng dữ liệu đã định danh giả.
2. **Cơ sở xử lý:** thay "lợi ích hợp pháp" bằng căn cứ hợp đồng với trường hoặc sự đồng ý, theo kết luận của pháp chế.
3. **Consent là một chiều của policy resolution:** trạng thái `granted | denied | unknown | withdrawn` theo phiên bản văn bản; mặc định `unknown` thì tắt các tính năng xã hội ngoài chat gameplay tối thiểu (quick message) cho tới khi có consent. API ghi/thu hồi consent theo phiên bản, có audit.
4. **Quy định game cho người dưới 18 tuổi:** pháp chế kết luận EduWorld có thuộc loại game phải cấp phép hay không; áp dụng phân loại độ tuổi, giới hạn thời gian chơi và xác thực tài khoản theo kết luận. Tham số giới hạn đặt ở cấp platform, trường chỉ siết thêm.
5. Đánh giá tác động xử lý dữ liệu (DPIA) và runbook thông báo vi phạm hoàn thành **trước Phase 4** (trước khi bật chat gameplay).

## Hệ quả

(+) Cơ sở pháp lý rõ trước khi xử lý dữ liệu trẻ em ở quy mô lớn; thực thi consent từ đầu, tránh sửa nhiều module sau. (−) Phụ thuộc kết luận pháp chế; thêm luồng thu consent qua trường.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR (Proposed) từ EDU_FIT_REVIEW | Technical Architect |
| 2026-11-15 | Hoãn, ngoài version này | ADR-0082 |
