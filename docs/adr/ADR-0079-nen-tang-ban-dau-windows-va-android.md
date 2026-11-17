# ADR-0079 — Nền tảng của bản phát hành đầu: Windows + Android; Web bản nhẹ và iOS sau

> **Trạng thái:** Rejected · **Ngày:** 2026-11-15 · **OQ:** OQ-04 · **Nếu Accepted sẽ thay một phần:** ADR-0013 · **Tài liệu:** [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md) · [GODOT_ARCHITECTURE](../08-game-client/GODOT_ARCHITECTURE.md) · [EDU_FIT_REVIEW](../12-project/EDU_FIT_REVIEW.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

[ADR-0013](ADR-0013-nen-tang-client-release-1-0-gom-windows-android-ios-web.md) yêu cầu Release 1.0 trên Windows, Android, iOS và Web. Bản Web của game 3D Godot chạy renderer Compatibility, đơn luồng, dễ giật khi tải vùng, giải mã video bằng CPU; chỉ thử Chrome/Edge. iOS cần máy Mac cho CI và quy trình ký riêng. Đội client có 3 Godot dev (gồm 1 tech artist). Thiết bị phổ biến ở trường là PC phòng máy và Android tầm thấp ([EDU_FIT_REVIEW](../12-project/EDU_FIT_REVIEW.md) mục 4.5).

## Quyết định (đề xuất)

1. Bản phát hành đầu (pilot, ADR-0073): **Windows** (phòng máy, có bộ cài đặt sẵn nội dung) và **Android**.
2. **Web:** làm sau pilot dưới dạng bản "learning-lite" (hoạt động học tập, hub đơn giản, không cảnh 3D nặng); video phát qua lớp HTML5. Quyết định cuối sau spike trên máy phòng tin thật.
3. **iOS:** sau Web, khi có hạ tầng build ký số.
4. Ngân sách hiệu năng (ADR-0057) và benchmark CI giữ nguyên cho các nền tảng đang hỗ trợ.

## Hệ quả

(+) Tập trung vào thiết bị trường thực sự dùng; giảm tải QA và tối ưu. (−) Học sinh dùng iPhone/iPad hoặc chỉ có trình duyệt phải chờ; phải sửa ROADMAP, PERFORMANCE_BUDGET, DEPLOYMENT.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR (Proposed) từ EDU_FIT_REVIEW | Technical Architect |
| 2026-11-17 | Từ chối: giữ đủ 4 nền tảng theo ADR-0013 | Technical Architect |
