# Quickstart: F061 — `ApiClient`, đăng nhập game trong form, chọn tenant, takeover, buộc cập nhật, đăng xuất

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F013, F014, F060.
- Lệnh: `make up`, `make migrate`, `make seed`, `make up-app`; mở `godot/project.godot`, profile `-- --env=local`, chạy `res://scenes/bootstrap/Boot.tscn` ([LOCAL_DEVELOPMENT §6.3](../../docs/11-operations/LOCAL_DEVELOPMENT.md)).
- Tài khoản: `hs.an.3a`, `hs.binh.3a`, `hs.tudo`, `hs.nosis`, `hs.locked`, `ad.truong.a`, `qlcm.truong.a`, `sa.admin`; tài khoản thử thuộc 2 tenant từ fixture integration của F013 (seed local không có học sinh 2 tenant).
- Hai instance để thử takeover: `godot --path godot -- --env=local --device-id=<uuid-A>` và `--device-id=<uuid-B>`.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | Đặt `min_client_version` (client-config local) lớn hơn bản build; mở game | ForceUpdate, không có form | F061-AC-01 |
| 2 | Đặt lại bình thường; mở game | Form S01 đúng thành phần, không trình duyệt | F061-AC-02 |
| 3 | Đăng nhập `hs.an.3a` (bắt gói qua proxy local) | Body đúng `GameLoginRequest`; không `Authorization`/`X-Tenant-Id` | F061-AC-03 |
| 4 | Bắt gói sau bước 3; sau khi vào game, quét `user://`, thư mục app, grep refresh token mẫu | `gameLogin` `200` không có `game_session_id`; ngay sau đó `createGameSession` có `X-Device-Id`, `X-Tenant-Id`, `Idempotency-Key` → `201`; `user://` chỉ có `device_id`, `settings.cfg` (`last_username`) và ciphertext `secure/` (Windows); không có token dạng rõ ở đâu | F061-AC-04 |
| 5 | Sai mật khẩu 3 lần | Thông điệp chung + gợi ý đợi; ô mật khẩu trống | F061-AC-05 |
| 6 | Giả lập hàng chờ (hạ ngưỡng `login_rate` local) | "Đang vào lớp…", tự vào khi hết hàng chờ; Hủy dừng | F061-AC-06 |
| 7 | Đăng nhập `ad.truong.a` / `sa.admin` | Màn "không dùng để vào game", không token | F061-AC-07 |
| 8 | Đăng nhập `hs.nosis` | "chưa được kích hoạt" | F061-AC-08 |
| 9 | Đăng nhập `hs.locked`; đổi ngôn ngữ EN rồi thử lại | Thông điệp theo `code`, đúng ngôn ngữ | F061-AC-09 |
| 10 | Dùng mật khẩu mẫu duy nhất, grep log/crash/telemetry | Không xuất hiện | F061-AC-10 |
| 11 | Quét artifact export Windows/Android | Không secret, không `/protocol/openid-connect/token`, không `oidc_` | F061-AC-11 |
| 12 | Bật nhớ tên, Đăng xuất, mở lại; bật `shared_device`, đăng nhập, đóng cửa sổ, mở lại | Tên điền sẵn ở form; với `shared_device` không có ô nhớ tên, mở lại luôn hiện form, không còn bản ghi phiên | F061-AC-12, F061-AC-26, F061-AC-32 |
| 13 | Tài khoản 2 tenant đăng nhập → chọn trường | `TenantChoice` sau xác thực; vào game không nhập lại mật khẩu | F061-AC-13 |
| 14 | Để màn chọn trường quá `game_login_ticket_ttl` rồi chọn | "Phiên chọn trường đã hết", về form | F061-AC-14 |
| 15 | Kiểm `user://` sau bước 13; Đăng xuất rồi đăng nhập lại | Không có dữ liệu tenant dạng rõ (chỉ trong bản ghi `SecureStorage`); `TenantChoice` không chọn sẵn | F061-AC-15 |
| 16 | Trong game, bắt gói request bất kỳ (`getContentManifest`) | Đủ header chuẩn, `X-Tenant-Id` = tenant phiên | F061-AC-16 |
| 17 | Toxiproxy: 503 cho GET và POST không key | GET retry theo backoff; POST không retry | F061-AC-17 |
| 18 | Chờ access token hết hạn (đặt TTL ngắn ở realm local), 5 request song song | Một lần refresh (`mode: "renew"`, có `device_id`); refresh token mới ghi đè `SecureStorage`; request lặp lại thành công | F061-AC-18, F061-AC-33 |
| 19 | Gỡ membership tenant trên backend khi đang chơi | `TENANT_MISMATCH` → về form "Em hãy đăng nhập lại" | F061-AC-19 |
| 20 | Unit test parse fixture Problem | `code`, `choices`, `other_device` có kiểu | F061-AC-20 |
| 21 | `hs.an.3a` chơi trên A, đăng nhập B → `createGameSession` `409` → "Tiếp tục ở đây" (bắt gói) | B gọi lại `createGameSession` `takeover: true` với `Idempotency-Key` khác, không có mật khẩu trong request; B vào game không nhập lại mật khẩu. Lặp lại, chọn "Ở lại máy kia" → B gửi `gameLogout`, về form; A tiếp tục | F061-AC-21 |
| 22 | Quan sát máy A sau bước 21 | Hộp "đã được đăng xuất vì…" trong ≤ 1 chu kỳ heartbeat, về form | F061-AC-22 |
| 23 | Cài đặt → Tài khoản → Đăng xuất (xác nhận); mở lại game | `endGameSession` gửi trước, rồi `gameLogout`; bản ghi `SecureStorage` bị xóa; mở lại → form (TC-AUTH-088) | F061-AC-23 |
| 24 | Để game chạy, quan sát heartbeat | `heartbeatGameSession` theo chu kỳ `ws_heartbeat` | F061-AC-24 |
| 25 | Android: đưa app ra nền 10 phút, rồi buộc dừng app và mở lại; máy `shared_device` để idle quá `shared_device_idle_logout` | Android: tiếp tục/tự vào lại, không hỏi mật khẩu; `shared_device`: tự đăng xuất, về form | F061-AC-25 |
| 25b | Thiết bị cá nhân: đăng nhập, đóng cửa sổ (bắt gói) | Chỉ `endGameSession`, không `gameLogout`; bản ghi `SecureStorage` còn | F061-AC-26 |
| 26 | Gửi `client_version` cũ trực tiếp tới `gameLogin` (bỏ qua bước 1) | ForceUpdate với `store_url` | F061-AC-27 |
| 27 | Dừng module game rồi đăng nhập `hs.an.3a` | `gameLogin` `200`; `createGameSession` lỗi sau retry → thông điệp lỗi, `gameLogout` gửi, về form, không vào game | F061-AC-28 |
| 28 | Sau bước 25b, mở lại game (Windows) | "Đang vào lớp…", `refreshGameToken` `mode: "resume"` có `device_id` + `X-Tenant-Id` đã lưu → `200` có `bootstrap`; bản ghi được ghi đè; `createGameSession` → vào hub, không hiện form (TC-AUTH-082) | F061-AC-29 |
| 29 | Realm local: đặt client session idle của `eduworld-game-backend` 2 phút; thoát, chờ 3 phút, mở lại | `401 REFRESH_TOKEN_INVALID` → bản ghi bị xóa; form với "Phiên đăng nhập đã hết, em đăng nhập lại nhé", tên điền sẵn (TC-AUTH-083) | F061-AC-30 |
| 30 | Chặn mạng (toxiproxy) rồi mở lại game | "Đang kết nối…", tự thử lại; bản ghi còn; nút "Đăng nhập bằng tài khoản khác" về form | F061-AC-31 |
| 31 | Bản Web: đăng nhập, tải lại tab; quét localStorage | Form hiện lại; không có refresh token (TC-AUTH-084) | F061-AC-32 |
| 32 | Sao chép refresh token đang lưu (debug build), gọi refresh bằng tiến trình khác rồi để game refresh bằng token cũ | Game nhận `401 REFRESH_TOKEN_REUSED` → về form; telemetry `auth_refresh_reused` (TC-AUTH-085) | F061-AC-33 |
| 33 | Xóa `user://device_id` rồi mở lại game | `401 REFRESH_TOKEN_DEVICE_MISMATCH` → form (TC-AUTH-086) | F061-AC-30 |
| 34 | School Admin đặt lại mật khẩu `hs.an.3a` (portal) khi máy đã thoát; mở lại game | Resume `401 REFRESH_TOKEN_INVALID` → form, không vòng lặp thử lại (TC-AUTH-087) | F061-AC-34 |
| 35 | Máy cá nhân có phiên đã lưu, mở game; bấm "Không phải tôi"; mở lại và bấm "Tiếp tục" | Màn "Tiếp tục là <nickname>?" trước khi vào; "Không phải tôi" → form trống, kho bảo mật trống; "Tiếp tục" → vào S02/S03 | F061-AC-35 |
| 36 | Clock server 21:35 (backend compose): mở game có phiên đã lưu; đang chơi lúc 21:20 rồi tới 21:30 | `CurfewNotice` "mở lại lúc 06:30", token còn; cảnh báo 21:20, 21:30 về `CurfewNotice` | F061-AC-36 |

## 3. Lệnh test tự động

```bash
make godot-test        # GdUnit4 headless: godot --headless --path godot -s addons/gdUnit4/bin/GdUnitCmdTool.gd -a res://tests
make godot-export-check
make test-e2e          # integration headless với backend compose (tests/integration/auth)
```

## 4. Bằng chứng cần nộp

- Kết quả GdUnit4 (pass/fail), log quét artifact (bước 11), log quét lưu trữ thiết bị (bước 4, 12, 31), bắt gói resume (bước 28–34), `trace_id` đăng nhập thành công, video/ảnh takeover hai thiết bị Windows + Android.
