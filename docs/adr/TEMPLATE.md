# Mẫu ADR

Sao chép khối dưới đây thành file mới `ADR-NNNN-<slug-khong-dau>.md` (số tiếp theo trong [chỉ mục](README.md)), rồi thêm một dòng vào bảng chỉ mục và nhóm chủ đề tương ứng.

```markdown
# ADR-NNNN — <Tiêu đề quyết định>

> **Trạng thái:** Proposed | Accepted | Superseded by ADR-xxxx | Proposed · một phần superseded by ADR-xxxx · **Ngày:** YYYY-MM-DD · **OQ:** OQ-xx | — · **Tài liệu:** [TÊN](../<thư-mục>/<FILE>.md) · [Chỉ mục ADR](README.md)

<!-- Chỉ khi bị thay toàn phần/một phần: -->
> [!WARNING]
> **Đã được thay bởi ADR-xxxx.** Không còn hiệu lực: … **Còn hiệu lực:** …

## Bối cảnh

Vấn đề, ràng buộc, yêu cầu của stakeholder, OQ liên quan.

## Phương án

1. … (tùy chọn khi có nhiều lựa chọn đáng cân nhắc)

## Quyết định

Bản **hiện hành**, viết liền mạch, đủ cụ thể để triển khai. Nhãn [Confirmed]/[Proposed]/[TBD] cho từng tham số chưa chốt. Không gạch xóa, không chú thích "sửa cùng ngày" trong thân — mọi lần sửa ghi ở Lịch sử thay đổi.

## Hệ quả

(+) Tích cực. (−) Tiêu cực / việc phải làm theo.

## Phương án đã loại

(tùy chọn) Lựa chọn khác và lý do loại.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| YYYY-MM-DD | Tạo ADR | … |
```

## Quy tắc

- ADR **Accepted** không sửa ý nghĩa quyết định; thay đổi = ADR mới, ADR cũ chuyển "Superseded by" (toàn phần) hoặc "một phần superseded" kèm khối cảnh báo ghi rõ phần còn/không còn hiệu lực.
- Điều chỉnh nhỏ không đổi ý nghĩa (đổi tên thư mục, tên service, tham chiếu) được cập nhật trực tiếp trong thân và ghi vào Lịch sử thay đổi.
- Link tới tài liệu dùng đường dẫn tương đối từ `docs/adr/`.
