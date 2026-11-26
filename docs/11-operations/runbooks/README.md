# Runbook theo alert

> Trạng thái: Draft · Owner: DevOps Engineer · Cập nhật: 2026-11-27 (F131)
> Liên quan: [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) · [BACKUP_RECOVERY](../BACKUP_RECOVERY.md) · [DEPLOYMENT](../DEPLOYMENT.md) · [Biên bản diễn tập](../alert-drills/README.md)

**Mục đích & phạm vi.** Mỗi alert của [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) có đúng một runbook `<AlertName>.md` trong thư mục này; annotation `runbook_url` của rule trỏ tới file đó (F131-AC-13). Ngưỡng/điều kiện chỉ định nghĩa ở MONITORING §5 (AGENTS R33) — runbook chỉ nhắc lại để người trực đọc nhanh. Runbook khôi phục dữ liệu RB-01…RB-05 do [BACKUP_RECOVERY §9](../BACKUP_RECOVERY.md) sở hữu; runbook alert chỉ link tới đó.

## 1. Khung bắt buộc

Mục theo đúng thứ tự (job `obs-runbooks` — `node scripts/obs/check-alerts.mjs --only=runbooks` — chặn khi thiếu hoặc sai thứ tự, hoặc có runbook không khớp alert nào):

| Mục | Nội dung |
|---|---|
| `## Triệu chứng` | Thông báo người trực nhận (`summary`) và điều kiện của MONITORING §5 |
| `## Tác động` | Ai/tính năng nào bị ảnh hưởng (học sinh, trường, vận hành) |
| `## Chẩn đoán` | Các bước theo thứ tự: dashboard, truy vấn, trace/log theo `trace_id`, lệnh Swarm (qua VPN) |
| `## Xử lý` | Hành động khắc phục, rollback, lệnh `make` tương ứng |
| `## Leo thang` | Khi nào và gọi ai (secondary, Technical Lead binary/module, Safety Team lead, Security lead) |
| `## Liên quan` | Link MONITORING, BACKUP_RECOVERY RB-xx, DEPLOYMENT, tài liệu domain |

Không đưa PII, token, nội dung chat hay đáp án vào runbook hay vào bằng chứng dán từ log (AGENTS R14).

## 2. Định tuyến và silence

- `critical` → receiver `oncall-critical` (chat nhóm vận hành + SMS + email), `warning` → `ops-warning` (chat), nhãn `team: safety` → `safety-oncall`, `Watchdog` → heartbeat ngoài (`deploy/observability/alertmanager/alertmanager.yml`). Gom nhóm theo `alertname`, `service`, `module`.
- `ServiceDown` của một Swarm service inhibit các alert khác cùng `service`/`profile`; alert critical inhibit bản warning cùng tên và cùng target.
- Silence: `amtool silence add --author=<tên> --comment="<lý do + ticket>" --duration=<ngắn nhất cần thiết> alertname=<…>` (Alertmanager API v2 bắt buộc `createdBy` và `comment`); xóa silence khi xong.

## 3. Thêm hoặc đổi alert

1. Thêm/sửa dòng ở MONITORING §5 (ngưỡng, mức) trước.
2. Rule trong `deploy/observability/prometheus/rules/<nhóm>.yaml` (nhãn `severity`, `service`/`module` khi theo binary/module; annotation `summary`, `runbook_url`, `source`).
3. Ca kiểm thử bắn + không bắn trong `deploy/observability/prometheus/tests/<nhóm>.test.yaml`.
4. Runbook `<AlertName>.md` theo khung §1.
5. `make obs-check` (promtool check/test, amtool, `scripts/obs/*`).

## 4. Diễn tập

`make alert-drill ENV=staging` kích hoạt giả lập từng alert critical và đo thời gian tới kênh trực (≤ 2 phút, F131-AC-11); biên bản theo mẫu ở [alert-drills/README.md](../alert-drills/README.md).
