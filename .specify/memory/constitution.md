# EduWorld Constitution

> Phiên bản: 1.2.0 · Phê chuẩn: 2026-11-15 · Sửa lần cuối: 2026-11-17 (điều II theo ADR-0074: ranh giới module của modular monolith) · Owner: Technical Architect
> Nguồn chi tiết: [AGENTS.md](../../AGENTS.md) §2 (R1–R34) · [ADR hiện hành](../../docs/adr/README.md) · [ADR-0082](../../docs/adr/ADR-0082-spec-driven-development-va-noi-dung-deferred.md)

Constitution là bộ nguyên tắc **bất biến** cho mọi feature theo Spec-Driven Development (GitHub Spec Kit). Nội dung chi tiết của từng quy tắc do **AGENTS.md §2** sở hữu (R33): file này nhóm các quy tắc thành điều khoản và nêu **câu hỏi gate** dùng trong mục *Constitution Check* của mọi `plan.md`. Không sửa điều khoản bằng feature spec; thay đổi phải qua ADR và tăng phiên bản file này.

## Điều khoản

| Điều | Nguyên tắc | Quy tắc chi tiết | Câu hỏi gate trong `plan.md` |
|---|---|---|---|
| I | **Spec trước, code sau.** Mọi tính năng có feature spec `specs/NNN-*/`; contract (OpenAPI/AsyncAPI/JSON Schema/event) cập nhật trước khi code. Không làm nội dung deferred. | R34, ADR-0010, ADR-0082 | Feature có `spec.md` trạng thái Ready? Contract mới đã nằm trong task Giai đoạn đầu? Không chạm `docs/99-deferred`, `specs/_deferred`? |
| II | **Ranh giới module và Clean Architecture + CQRS.** Backend là modular monolith (`src/backend/modules/<module>/`, ba binary `core-api`/`realtime`/`worker`): module khác chỉ import `src/backend/modules/<module>/api` (facade sinh từ spec internal, có trong `x-allowed-callers`), không import `internal/` của module khác, không đọc/ghi database của module khác; gọi đồng bộ qua facade trong tiến trình (`src/backend/pkg/modcall`), phản ứng phụ qua event; tách rõ write context (command) và read context (query), không mediator. | R13, R26, R29, ADR-0074, ADR-0038, ADR-0040, ADR-0042, ADR-0087 | Thay đổi nằm trong đúng module sở hữu và database của module đó? Gọi module khác qua facade (operation có `x-allowed-callers` gồm module gọi) hoặc event, không gọi facade trong transaction? Tầng domain không phụ thuộc hạ tầng? Query không ghi DB/phát event? Command dùng `UnitOfWork` + outbox? Điểm vào mới (route, consumer, job) đăng ký trong `module.go` và chạy ở đúng binary/profile? |
| III | **Cô lập tenant và an toàn dữ liệu.** Mọi truy vấn có tenant + soft-delete; chỉ soft-delete; PostgreSQL là nguồn sự thật; migration expand/contract. | R8, R9, R12, R21, R27, ADR-0005, ADR-0028, ADR-0039 | Mọi bảng/truy vấn mới có `tenant_id` + `deleted_at`? Có test tenant isolation? Không có hard-delete? |
| IV | **Server-authoritative cho học tập và phần thưởng.** Không tin client; không gửi đáp án; nội dung publish bất biến, lượt chơi ghim version. | R5, R6, R10, R11, R31, ADR-0016, ADR-0043 | Điểm/kết quả/phần thưởng tính ở server? Render payload không chứa đáp án? Thao tác quan trọng có Idempotency-Key + audit? |
| V | **Xác thực và phân quyền ở backend.** Mọi endpoint có authn + authz nghiệp vụ; phân vùng quyền platform/tenant; game chỉ cho học sinh; đường đăng nhập nhẹ. | R3, R4, R7, R20, R23, R30, ADR-0066, ADR-0072, ADR-0081 | Permission cụ thể cho từng endpoint? Không cấp quyền platform cho role tenant? Không hardcode secret/URL? |
| VI | **An toàn trẻ em.** Không log PII; một bộ quy tắc chat cho mọi lớp, mọi tin qua pipeline; không lộ danh tính; không dark pattern. | R14, R15, R16, R19, R25, ADR-0020, ADR-0030, ADR-0035 | Có dữ liệu/chat của học sinh? Đã kiểm tra R14–R16, R19, R25? |
| VII | **Một nguồn sự thật, cấu hình không hardcode.** Quy tắc/tham số định nghĩa một nơi; đọc giá trị từ cấu hình; mọi chuỗi có giới hạn độ dài; mã FR chỉ từ PRD. | R2, R28, R33, ADR-0041 | Tham số dùng tên trong CONFIG_DEFAULTS? Chuỗi mới có `maxLength`/`MaxLen`? Không có mã FR tự tạo? |
| VIII | **UI tách logic, giao diện theo token.** Không business logic trong UI; hai design system tách biệt; không hardcode style. | R1, R32, ADR-0047 | UI chỉ gọi service/API? Style lấy từ token? |
| IX | **Test có bằng chứng.** Test viết trước phần code của từng user story và phải thất bại trước khi code; không tuyên bố xong khi chưa có test pass; mock phải gắn nhãn. | R17, R18, AGENTS §5 | Mỗi tiêu chí `F<NNN>-AC-*` có test? Có test tenant isolation + authorization? Không mock trong đường chạy thật? |

## Quản trị

- Vi phạm một điều khoản chỉ được chấp nhận khi ghi trong mục *Complexity Tracking* của `plan.md` kèm lý do và phương án đơn giản hơn đã bị loại; vi phạm điều III, IV, V, VI là **không được phép**.
- Sửa constitution: ADR mới → cập nhật AGENTS.md (nguồn chi tiết) → tăng phiên bản file này (MAJOR khi bỏ/đổi nghĩa điều khoản, MINOR khi thêm, PATCH khi sửa câu chữ).
