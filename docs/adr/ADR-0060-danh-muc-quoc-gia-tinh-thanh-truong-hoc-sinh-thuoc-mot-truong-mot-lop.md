# ADR-0060 — Danh mục quốc gia, tỉnh/thành, trường; học sinh thuộc đúng một trường và một lớp

> **Trạng thái:** Accepted · **Ngày:** 2026-11-15 · **OQ:** — · **Tài liệu:** [DOMAIN_MODEL](../06-architecture/DOMAIN_MODEL.md) · [SIS_INTEGRATION](../09-identity/SIS_INTEGRATION.md) · [MULTITENANCY](../06-architecture/MULTITENANCY.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

Stakeholder (người duyệt) yêu cầu: phải có **danh mục trường**; **học sinh phải thuộc một trường và một lớp cụ thể**; có **danh mục tỉnh/thành** (34 tỉnh/thành mới nhất của Việt Nam) gắn với **quốc gia** để sau này thêm tỉnh/thành của quốc gia khác.

## Quyết định

1. **Danh mục dùng chung toàn platform** (thuộc identity-service, không theo tenant; chỉ System Admin sửa; các service khác nhận qua projection `ref.*`):
   - **Country**: `country_code` (ISO 3166-1 alpha-2, ví dụ `VN`), tên song ngữ (ADR-0056), `status`. Release 1.0 chỉ bật `VN`.
   - **Province** (tỉnh/thành cấp 1): `province_id`, `country_code`, `code` nội bộ ổn định, `official_code` (mã đơn vị hành chính chính thức — **[TBD] nhập từ danh mục chính thức của cơ quan nhà nước, không tự đặt**), `name` song ngữ, `type` (`municipality` thành phố trực thuộc trung ương | `province` tỉnh), `effective_from`, `effective_to`, `status`. Có **lịch sử**: đơn vị cũ (trước sáp nhập) giữ dạng `inactive` + bảng `province_successions(old_province_id → new_province_id, effective_date)` để ánh xạ dữ liệu cũ.
   - Cấp xã/phường: chưa làm trong Release 1.0; mô hình để mở rộng `ward` sau [Confirmed].
   - **School** (danh mục trường): `school_id`, `tenant_id`, `country_code`, `province_id`, `moet_code` (mã trường do ngành giáo dục cấp, nếu có), tên song ngữ, cấp học (`primary` tiểu học, `lower_secondary` THCS, `multi_level` liên cấp), khối áp dụng (1–9), địa chỉ (chuỗi, có giới hạn độ dài), `status`, `sis_connection_id?`. Trường do SIS đồng bộ hoặc System Admin/Tenant Admin tạo; mỗi trường thuộc đúng một tenant.
2. **Seed tỉnh/thành Việt Nam (34 đơn vị)** theo sắp xếp đơn vị hành chính có hiệu lực từ 01/7/2025 [Confirmed về số lượng; mã chính thức TBD]. Dữ liệu seed đặt tại `specs/reference-data/` (JSON, có schema), kèm 63 đơn vị cũ ở trạng thái `inactive` và bảng kế thừa. Trước khi đưa vào production phải đối chiếu tên, loại và mã với văn bản chính thức.
3. **Học sinh thuộc đúng một trường và một lớp** [Confirmed]:
   - Tại mọi thời điểm, mỗi học sinh có **đúng một enrollment active** gắn với **một lớp** của **một trường** trong năm học hiện tại (unique partial index `enrollments(student_id) WHERE status = 'active' AND deleted_at IS NULL`; lớp thuộc trường của học sinh).
   - Học sinh SIS **không có lớp** trong dữ liệu SIS: không được vào game (`NO_ACTIVE_ENROLLMENT`), tạo `reconciliation_issue` để trường xử lý; không tự gán lớp giả.
   - Học sinh **tenant chung** (ADR-0026): thuộc School ảo "EduWorld chung" và **lớp ảo theo khối** ("Khối 3 – chung"…), học sinh chọn/khai báo khối khi tạo tài khoản theo quy trình của platform [Confirmed].
   - Chuyển lớp/chuyển trường: kết thúc enrollment cũ và mở enrollment mới trong cùng transaction (không có khoảng trống, không trùng).
4. **Hiển thị trong game:** hồ sơ học sinh hiển thị cho **chính học sinh** tên trường, lớp, tỉnh/thành; người chơi khác chỉ thấy nickname (R16) — riêng bạn cùng lớp/giáo viên theo policy trường.

## Hệ quả

(+) Dữ liệu tổ chức chuẩn hóa, báo cáo theo tỉnh/trường/lớp được; sẵn sàng cho quốc gia khác. (−) Phải duy trì danh mục hành chính khi có thay đổi; cần quy trình xử lý học sinh SIS thiếu lớp.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR | Stakeholder (người duyệt) |
