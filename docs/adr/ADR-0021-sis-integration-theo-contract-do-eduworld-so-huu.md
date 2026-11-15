# ADR-0021 — SIS integration theo contract do EduWorld sở hữu

> **Trạng thái:** Accepted · **Ngày:** 2026-11-15 · **OQ:** OQ-01 · **Tài liệu:** [SIS_INTEGRATION](../09-identity/SIS_INTEGRATION.md), [SIS contract](../../specs/sis/eduworld-sis-contract.v1.yaml) · [Chỉ mục ADR](README.md)

## Bối cảnh

Mỗi trường có thể dùng SIS khác nhau. Stakeholder xác nhận: trường cung cấp API, nhưng **DTO do EduWorld đưa ra**, không để SIS áp DTO riêng vì sẽ làm vỡ cấu trúc domain.

## Quyết định

- EduWorld công bố **EduWorld SIS Contract v1** (`specs/sis/eduworld-sis-contract.v1.yaml`): endpoint pull mà trường triển khai, payload webhook push mà SIS gửi tới EduWorld, và DTO chuẩn (School, AcademicYear, Class, Student, Teacher, Enrollment, TeachingAssignment, Change). Enum và mã môn dùng giá trị của EduWorld.
- Trường hoặc nhà cung cấp SIS tự dựng **facade** chuyển dữ liệu SIS sang contract. EduWorld **không** viết adapter theo DTO của từng SIS, **không** cho cấu hình field-mapping/JSONPath theo SIS.
- `identity-service` có port `SISProvider` với đúng hai implementation: `contract_v1` (HTTP client sinh từ spec) và `csv_import` (cột CSV = tên trường DTO). Dữ liệu đi qua contract validator → staging → reconciliation → apply, chạy trong worker của identity-service.
- Chỉ EduWorld được sửa contract; v1 chỉ thêm trường tùy chọn; breaking change → v2 song song. Bản ghi vi phạm contract bị từ chối; trường lạ bị bỏ qua và không lưu.
- EduWorld cung cấp bộ conformance test và mock server để trường tự kiểm tra facade.
- Client (Godot, Admin Portal) không bao giờ gọi SIS. Credential SIS nằm trong secret manager.
- Cách nhận biết SIS nguồn của học sinh khi đăng nhập: ADR-0046.

## Hệ quả

(+) Domain EduWorld không bị cấu trúc dữ liệu SIS bên ngoài làm vỡ; thêm trường không cần viết code. (−) Trường phải tự xây facade; trường chưa làm được thì dùng CSV.

## Phương án đã loại

Adapter riêng cho từng SIS hoặc adapter cấu hình mapping: để DTO của SIS lọt vào EduWorld.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR (port & adapter cho từng SIS) | Phase 0 |
| 2026-11-15 | Accepted: contract do EduWorld sở hữu, bỏ adapter theo SIS | Stakeholder |
| 2026-11-15 | Thuộc identity-service | ADR-0036 |
