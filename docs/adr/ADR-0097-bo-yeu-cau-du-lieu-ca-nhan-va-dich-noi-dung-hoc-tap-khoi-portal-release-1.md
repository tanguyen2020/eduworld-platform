# ADR-0097 — Bỏ "Yêu cầu dữ liệu cá nhân" và "Dịch nội dung học tập" khỏi Admin Portal Release 1

> **Trạng thái:** Accepted (owner chốt 2026-11-22; rủi ro được chấp nhận) · **Ngày:** 2026-11-22 · **OQ:** — · **Tài liệu:** [ADMIN_PORTAL](../04-admin-portal/ADMIN_PORTAL.md) · [DATA_PRIVACY §3](../10-security/DATA_PRIVACY.md#3-yêu-cầu-về-dữ-liệu-học-sinh) · [API_DESIGN](../07-backend/API_DESIGN.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

Catalog menu Portal 2026.11.22.1 còn hai menu chưa có màn hình (placeholder) nhưng đã có API backend: **Quản trị hệ thống → Yêu cầu dữ liệu** (`menu.platform_admin.data_requests`, F016 US5: `listDataRequests`, `createDataRequest`, `getDataRequest`) và **Content Studio → Bản dịch** (`menu.content_studio.translations`, F025 US5: `exportLearningTranslations`, `importLearningTranslations`, `getLearningTranslationJob`; nút xuất/nhập trên trình soạn câu hỏi F053-AC-25). Lần bỏ ba menu trước (catalog 2026.11.22.1) giữ hai menu này chờ owner quyết. `check-portal-catalog.mjs` buộc mọi operation `/cms`, `/admin`… có `x-portal-action` trong catalog, nên bỏ menu thì phải bỏ luôn operation.

## Quyết định

1. Release 1 **không có** hai menu trên. Catalog 2026.11.23.1 bỏ `menu.platform_admin.data_requests` (action `…data_requests.view`, `…data_requests.create`) và `menu.content_studio.translations` (action `…translations.translate`).
2. Sáu operation public bị gỡ khỏi contract và route HTTP: `listDataRequests`, `createDataRequest`, `getDataRequest` (module `identity`), `exportLearningTranslations`, `importLearningTranslations`, `getLearningTranslationJob` (module `learning`). Code chỉ phục vụ các operation này bị xóa (command/query/repository, codec XLIFF/CSV và job `translation-jobs` của learning; tiếp nhận yêu cầu của identity). Giữ lại: saga `user_anonymize` và job `retention-anonymizer` (F016-AC-20/21/23), schema `TranslationJob`/`TranslationExportRequest`/`TranslationImportRequest` dùng chung với dịch nội dung game (F082), `translation_status` và chặn `TRANSLATION_INCOMPLETE` (F025).
3. Bảng `platform.deletion_requests` và `content.translation_jobs` giữ nguyên, không migration xóa (R12, ADR-0070); permission `platform.data_request.write` và `learning_content.translate` giữ trong catalog nhưng Release 1 không có operation nào dùng.
4. **Rủi ro được chấp nhận:** Release 1 không có kênh Portal để tiếp nhận và theo dõi yêu cầu của chủ thể dữ liệu (truy cập, xóa, hạn chế xử lý) theo Nghị định 13/2023/NĐ-CP. Yêu cầu đến qua nhà trường được vận hành xử lý thủ công theo [DATA_PRIVACY §3.1](../10-security/DATA_PRIVACY.md#31-release-1-không-có-kênh-portal-xử-lý-thủ-công). Người dịch nội dung học tập nhập bản dịch trực tiếp trong trình soạn (song ngữ theo ADR-0056), không có xuất/nhập file.
5. Đưa lại một trong hai tính năng cần ADR mới, thêm lại menu vào catalog, operation vào contract và AC vào feature spec.

## Hệ quả

(+) Portal không còn menu dẫn tới màn trống; contract không còn operation không có màn hình gọi.
(+) Giảm bề mặt tấn công ở route platform đọc dữ liệu học sinh liên tenant.
(−) Thời hạn xử lý yêu cầu theo luật phụ thuộc quy trình thủ công của vận hành; không có hàng đợi, trạng thái hay audit theo mã yêu cầu trên Portal — ghi [RISKS](../12-project/RISKS.md) RISK-57.
(−) Thay đổi phá vỡ contract (gỡ 6 operation): đăng ký trong `specs/breaking-allowlist.yaml` với ADR này.
(−) Môi trường đang chạy phải đồng bộ lại catalog menu: `cmd/migrate --module=identity`.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-22 | Tạo ADR theo quyết định owner bỏ hai menu khỏi Release 1 | Owner dự án |
