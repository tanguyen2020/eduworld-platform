# SchoolKitIndexInvalid

> Runbook của alert `SchoolKitIndexInvalid` (F134). Ngưỡng và điều kiện thuộc [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed) — runbook không định nghĩa ngưỡng. Rule: `deploy/observability/prometheus/rules/game.yaml` · Mức: **warning**.

## Triệu chứng

- Thông báo: _The signed school-kit index cannot be loaded or verified (installer page answers 503)_
- Điều kiện (MONITORING §5): `increase(eduworld_school_kit_index_fetch_total{result="invalid"}[15m]) > 0`

## Tác động

Trang "Tải bộ cài" của workspace C trả `503 UNAVAILABLE` cho School Admin/Tenant Admin: không link nào được trả khi chỉ mục chưa kiểm được chữ ký (F134-AC-14). Học sinh và bản Web không bị ảnh hưởng.

## Chẩn đoán

1. Log cảnh báo `school-kit index unavailable` của `core-api` (module game): lỗi tải (CDN `5xx`/timeout), `signature does not verify` hoặc `invalid school-kit index`.
2. Kiểm tra `<EW_CLIENT_RELEASES_BASE_URL>/school-kit/production.json` và `production.json.sig` trên CDN phát hành (cùng thư mục với manifest phát hành của F133).
3. So khóa công khai `EW_CLIENT_RELEASE_PUBKEY_FILE` của `core-api` với khóa phát hành dùng để ký (đổi khóa mà chưa đổi secret → chữ ký sai).

## Xử lý

1. Chạy lại job phát hành `deploy/installer/school-kit/publish-index.sh --channel=production …` (ký lại chỉ mục từ `kits.json` của bản đã promote) và tải lên CDN.
2. Nếu khóa phát hành đã xoay: cập nhật Docker secret của khóa công khai rồi deploy lại `core-api` (DEPLOYMENT §5).
3. Cache chỉ mục đã kiểm (Valkey `ew:game:school_kit_index:production`, 5 phút) tự hết hạn; không cần xóa tay.

Silence trong lúc xử lý phải có người tạo và lý do (Alertmanager `createdBy` + `comment`), thời hạn ngắn nhất cần thiết.

## Leo thang

Không gọi ngoài giờ.

## Liên quan

- [DEPLOYMENT §9](../DEPLOYMENT.md)
- [MONITORING §5](../MONITORING.md#5-alert-rules--confirmed)
- [runbooks/README](README.md)
