# Biên bản kiểm thử khôi phục

> Trạng thái: Draft · Phiên bản: 0.1 · Owner: DevOps Engineer · Cập nhật: 2026-11-27 (F130 T041)
> Liên quan: [BACKUP_RECOVERY §8](../BACKUP_RECOVERY.md#8-kiểm-thử-khôi-phục-định-kỳ) · [DEPLOYMENT §15](../DEPLOYMENT.md#15-checklist-trước-khi-lên-production-phase-6) · [F130](../../../specs/130-swarm-secrets-backup-dr/spec.md)

**Mục đích & phạm vi.** Nơi lưu bằng chứng của mọi lần kiểm thử khôi phục và diễn tập trên **staging/production** (một file `<YYYY-MM>.md` mỗi tháng). Tiêu chí đạt, tần suất và runbook thuộc [BACKUP_RECOVERY](../BACKUP_RECOVERY.md); thư mục này chỉ chứa biên bản. Kết quả chạy test tự động trên container dùng một lần (`make test-ops-integration`) **không** phải biên bản ở đây (đó là stand-in, không phải môi trường thật).

## 1. Cách tạo biên bản

- Restore test tháng / quý: `make restore-test ENV=<env> MODE=full` hoặc `MODE=module DB=eduworld_<module>` (chạy `scripts/backup/restore_test.sh` trong job một lần có credential khôi phục, cụm tạm cô lập). Script tự ghi một mục vào `/restore/reports/<YYYY-MM>.md` của service `backup-tools`; DevOps chép mục đó vào file cùng tên ở thư mục này trong MR kèm link pipeline.
- Diễn tập switchover (`scripts/ops/patroni-switchover.sh`), RB-04 (`scripts/ops/rb04-keycloak-restore.sh`), RB-05 (`scripts/ops/rb05-object-storage-restore.sh`), DR toàn site (`scripts/ops/rb03-cluster-restore.sh`): dán dòng audit JSON (`audit:true`) và số đo vào mục mới theo mẫu dưới.

## 2. Mẫu một mục (trường bắt buộc, F130 data-model §1)

```markdown
## <thời điểm bắt đầu UTC> — <loại kiểm thử>

| Trường | Giá trị |
|---|---|
| Loại kiểm thử | Restore full + PITR / Restore một database module / Switchover / RB-04 / RB-05 / DR |
| Môi trường | staging / production |
| Người thực hiện | <tài khoản> |
| Mục đích | <vì sao: lịch tháng, sau sự cố…> |
| Thời điểm đích T | <timestamp có múi giờ> |
| Bắt đầu | <UTC> |
| Kết thúc | <UTC> |
| Thời lượng | <giây> (ngưỡng theo BACKUP_RECOVERY §8) |
| Kết quả | **PASS** / **FAIL** |
| Pipeline | <link workflow/job> |

**Kết quả đối soát**
- <số attempt submit khôi phục/live, `worker reconcile --module=<module>`, checksum object…>

**Vấn đề**
- <không có / mô tả + việc tiếp theo>
```

## 3. Trạng thái

Chưa có biên bản: staging/production chưa được cấp (F130, chờ môi trường). Lần đầu: restore test full + module `eduworld_learning` ngay sau khi staging chạy backup đầy đủ tuần đầu tiên.
