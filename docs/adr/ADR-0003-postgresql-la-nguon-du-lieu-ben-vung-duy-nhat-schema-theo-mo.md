# ADR-0003 — PostgreSQL là nguồn dữ liệu bền vững duy nhất, schema theo module

> **Trạng thái:** Proposed · một phần superseded by [ADR-0037](ADR-0037-moi-service-mot-database-postgresql-thay-mot-phan-adr-0003.md) và [ADR-0039](ADR-0039-orm-ent-cho-go-tuong-duong-entity-framework-migration-bang-a.md) · **Ngày:** 2026-11-15 · **OQ:** OQ-10 · **Tài liệu:** [DATABASE_DESIGN](../07-backend/DATABASE_DESIGN.md) · [Chỉ mục ADR](README.md)

> [!WARNING]
> **Một phần đã được thay.** Không còn hiệu lực: "một database, mỗi module một schema" (thay bởi ADR-0037: mỗi service một database) và "migration bằng goose" (thay bởi ADR-0039: Atlas). **Còn hiệu lực:** PostgreSQL là system of record; ID là UUIDv7; bảng tăng trưởng lớn được partition theo thời gian; không đọc/ghi dữ liệu của domain khác trực tiếp.

## Bối cảnh

Cần một nguồn dữ liệu bền vững duy nhất cho dữ liệu học tập, game, thi đấu, chat; Valkey chỉ là dữ liệu tạm (ADR-0004).

## Quyết định

- **PostgreSQL** (≥16, khuyến nghị 17) là system of record.
- ID là **UUIDv7** (sắp xếp theo thời gian, an toàn để lộ ra API).
- Bảng tăng trưởng lớn (learning events, tin nhắn chat, audit log, attempt answers, ledger) **partition theo thời gian**.
- Một domain không đọc/ghi trực tiếp dữ liệu của domain khác; nay được thực thi bằng database riêng cho từng service (ADR-0037).
- *(Gốc, không còn hiệu lực)* Mỗi module một schema (`identity`, `org`, `sis`, `curriculum`, `content`, `assessment`, `learning`, `game`, `multiplayer`, `competition`, `social`, `chat`, `gamification`, `lms`, `audit`, `platform`) trong một database; migration tập trung bằng goose.

## Hệ quả

(+) Một công nghệ lưu trữ, nhất quán giao dịch trong từng service. (−) Cần chiến lược partition và retention cho bảng lớn.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR | Phase 0 |
| 2026-11-15 | Phần "một DB, schema theo module" thay bởi ADR-0037 | Stakeholder chọn DB riêng mỗi service |
| 2026-11-15 | Migration goose thay bởi Atlas | ADR-0039 |
