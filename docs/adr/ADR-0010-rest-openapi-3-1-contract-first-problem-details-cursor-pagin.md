# ADR-0010 — REST + OpenAPI 3.1 contract-first, Problem Details, cursor pagination, Idempotency-Key

> **Trạng thái:** Accepted · **Ngày:** 2026-11-15 · **OQ:** — · **Tài liệu:** [API_DESIGN](../07-backend/API_DESIGN.md), [specs](../../specs/README.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

Nhiều client (Godot, Admin Portal) và nhiều service cần một contract ổn định, kiểm tra được trong CI.

## Quyết định

- Viết `specs/openapi/*.yaml` trước; spec public gộp `eduworld-api.v1.yaml` là nguồn sự thật, spec theo service được sinh tự động; sinh server stub/types bằng `oapi-codegen`. CI chạy lint (Redocly + Spectral) và breaking-change check (oasdiff).
- Lỗi theo RFC 9457 `application/problem+json`, mở rộng `code` (mã lỗi ổn định), `trace_id`, `errors[]` cho validation. Catalog mã lỗi trong API_DESIGN và enum `ProblemCode` phải khớp tuyệt đối.
- Pagination cursor-based: `?limit=&cursor=`, response `{ "data": [...], "page": { "next_cursor": ..., "has_more": ... } }`.
- Thao tác quan trọng (submit bài, nộp thi, tạo party, gửi lời mời, trao thưởng, mua trong Shop, xác nhận kết quả trận) bắt buộc header `Idempotency-Key`; server lưu kết quả 24 giờ.
- Versioning qua path `/api/v1`; breaking change → `/api/v2` song song tối thiểu 1 học kỳ.
- Mọi trường chuỗi có `maxLength` (ADR-0041).

## Hệ quả

(+) Contract kiểm chứng được, sinh code giảm sai lệch. (−) Mọi thay đổi API phải sửa spec trước.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR | Phase 0 |
| 2026-11-15 | Spec gộp là nguồn sự thật, spec theo service sinh tự động; maxLength bắt buộc | ADR-0036, ADR-0041 |
| 2026-11-20 | Owner chốt (Accepted) | Owner |
