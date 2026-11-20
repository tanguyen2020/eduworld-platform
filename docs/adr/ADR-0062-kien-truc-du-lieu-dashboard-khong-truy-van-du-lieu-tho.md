# ADR-0062 — Kiến trúc dữ liệu dashboard: không truy vấn dữ liệu thô, chỉ đọc bảng tổng hợp

> **Trạng thái:** Accepted · một phần superseded by ADR-0072 · **Ngày:** 2026-11-15 · **OQ:** — · **Tài liệu:** [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) · [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md) · [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md) · [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md) · [Chỉ mục ADR](README.md)

> [!WARNING]
> **Một phần đã được thay bởi [ADR-0072](ADR-0072-bo-vai-tro-giao-vien-hoc-sinh-tu-hoc-kiem-duyet-tap-trung.md).** Không còn hiệu lực: dashboard "giáo viên/lớp" trong phạm vi áp dụng — nay là School Learning Dashboard (trường → lớp → học sinh). **Còn hiệu lực:** toàn bộ kiến trúc dữ liệu dashboard.

## Bối cảnh

Stakeholder yêu cầu: phải có trang dashboard; dữ liệu rất lớn vì lấy theo từng ngày; cơ chế phải hợp lý để **không làm treo database** khi query. Áp dụng cho mọi dashboard: giáo viên/lớp, trường, tenant, platform, Live Ops, Game Analytics, Learning Analytics.

## Quyết định

1. **Dashboard không bao giờ truy vấn bảng nghiệp vụ hay bảng sự kiện thô.** Chỉ đọc **bảng tổng hợp (rollup)** dựng sẵn theo các chiều cố định: ngày, tuần, tháng × tenant × trường × lớp × khối × môn/LO (learning) hoặc nền tảng/ngôn ngữ/hạng thiết bị (game).
2. **Dựng rollup gia tăng (incremental)** bằng worker từ event NATS (ADR-0042) và job định kỳ:
   - Mỗi event cập nhật bộ đếm "hôm nay" (upsert theo khóa chiều) — số liệu gần thời gian thực.
   - Job cuối ngày (02:30 ICT [Confirmed]) chốt ngày hôm qua, tính các chỉ số không cộng dồn được (distinct, retention, percentile) và rollup tuần/tháng từ bảng ngày.
   - Dữ liệu đến muộn ≤ 48 giờ được tính lại cho đúng ngày (ADR-0058).
3. **Lưu trữ:** bảng rollup **partition theo tháng** (bảng ngày) hoặc theo năm (bảng tuần/tháng); index B-tree theo `(tenant_id, scope_id, date)` và BRIN theo `date` cho bảng lớn; dùng `pg_partman` hoặc job tương đương để tạo/gỡ partition. Rollup nằm trong database của service sở hữu (learning-service cho học tập, game-service cho game) — không join chéo database.
4. **Bảo vệ database khi truy vấn:**
   - Truy vấn dashboard chạy trên **read replica** với role DB riêng, `statement_timeout` 5 giây, giới hạn số kết nối cho pool dashboard [Confirmed].
   - **Tự chọn độ hạt theo khoảng thời gian:** ≤ 31 ngày → theo ngày; ≤ 1 năm → theo tuần; > 1 năm → theo tháng. Khoảng tối đa một lần truy vấn: 2 năm.
   - Bắt buộc lọc theo tenant (và trường/lớp theo quyền); cấm truy vấn không có điều kiện thời gian.
   - Kết quả ô/tile cache Valkey theo khóa (scope, metric, khoảng, độ hạt), TTL 5 phút cho hôm nay, 24 giờ cho dữ liệu đã chốt.
   - Danh sách chi tiết (ví dụ từng học sinh) phân trang cursor, tối đa 100 dòng/trang.
   - Xuất báo cáo lớn chạy **bất đồng bộ** (job + file trên object storage + link có hạn), không chạy trên request.
5. **Ngưỡng nâng cấp:** khi truy vấn dashboard p95 > 2 giây hoặc bảng rollup ngày > 500 triệu dòng → đánh giá kho phân tích riêng (ClickHouse hoặc tương đương) bằng ADR mới (đồng nhất ngưỡng với ADR-0058).
6. **Nguồn sở hữu định nghĩa chỉ số:** học tập → LEARNING_ANALYTICS; game → GAME_ANALYTICS; cơ chế dữ liệu dashboard chung → DASHBOARD_DATA.

## Hệ quả

(+) Dashboard nhanh và ổn định bất kể dữ liệu lớn; database nghiệp vụ không bị ảnh hưởng. (−) Số liệu "hôm nay" có độ trễ vài giây đến vài phút; thêm chiều mới phải dựng lại rollup (backfill); cần read replica.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR | Stakeholder (người duyệt) |
| 2026-11-15 | Một phần superseded by ADR-0072 (bỏ vai trò giáo viên) | Technical Architect |
