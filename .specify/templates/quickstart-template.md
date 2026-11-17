# Quickstart: F[NNN] — [Tên tính năng]

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

<!--
Cách chạy và kiểm feature end-to-end trên môi trường local (LOCAL_DEVELOPMENT). Dùng làm bằng chứng nghiệm thu.
-->

## 1. Chuẩn bị

- Phụ thuộc đã Done: F…
- Lệnh: `make up`, `make migrate`, `make seed`, `make run-core-api` hoặc `make dev` (3 binary; theo [LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md))
- Tài khoản demo: `…`

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | … | … | F[NNN]-AC-01 |

## 3. Lệnh test tự động

```bash
make test MOD=<module>
make test-integration MOD=<module>
```

## 4. Bằng chứng cần nộp

- Kết quả test (số pass/fail), trace_id của kịch bản chính, ảnh màn hình (nếu có UI).
