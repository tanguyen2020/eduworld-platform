# Biên bản diễn tập alert

> Trạng thái: Draft · Owner: DevOps Engineer · Cập nhật: 2026-11-27 (F131)
> Liên quan: [DEPLOYMENT §15](../DEPLOYMENT.md) · [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) · [Runbook theo alert](../runbooks/README.md)

**Mục đích & phạm vi.** Bằng chứng "alert chính đã kích hoạt thử và đến đúng người trực" (DEPLOYMENT §15, F131-AC-11). Mỗi đợt diễn tập ghi một file `<YYYY-MM>.md`; biên bản ca trực Safety Team (thiếu ca, case p1 quá hạn) cũng lưu ở đây. Không dán PII, token hay nội dung chat.

## 1. Cách chạy

```bash
make alert-drill ENV=staging                    # mọi alert critical của MONITORING §5
make alert-drill ENV=staging ALERT=ServiceDown  # một alert
```

`scripts/obs/alert-drill.sh` gửi một alert tổng hợp (nhãn `drill="true"`, `severity` của rule) vào Alertmanager staging qua API v2 (`amtool alert add`), chờ người trực xác nhận đã nhận trên kênh (hoặc webhook kiểm tra của kênh chat trả về), đo thời gian và ghi một dòng vào biên bản tháng. Alert có điều kiện thật dễ tạo trên staging (ví dụ `ServiceDown` của `worker-chat`) nên được diễn tập bằng điều kiện thật ít nhất mỗi quý.

## 2. Mẫu biên bản `<YYYY-MM>.md`

| Alert | Cách kích hoạt (synthetic / điều kiện thật) | Thời điểm kích hoạt (UTC) | Thời điểm nhận ở kênh trực | Độ trễ | Người nhận | Kết quả (đạt ≤ 2 phút?) | Vấn đề / việc tiếp theo |
|---|---|---|---|---|---|---|---|
| `ServiceDown` | điều kiện thật: `docker service scale eduworld_worker-chat=0` 3 phút | | | | | | |
