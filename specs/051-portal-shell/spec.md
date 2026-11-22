# Feature Specification: F051 — Portal shell: đăng nhập, chọn tenant sau đăng nhập, menu theo quyền, bộ lọc tenant

**Feature Branch**: `051-portal-shell` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: `src/cms/frontend/apps/portal` · **Mốc**: M-C1 · **Phụ thuộc**: F012, F050
**Input**: Khung Admin Portal dùng chung cho 5 workspace: trang đăng nhập do portal tự vẽ gọi `/api/v1/auth/portal/*` (ADR-0091, không chuyển hướng OIDC), thiết lập tài khoản lần đầu, quên/đặt lại mật khẩu, định tuyến sau đăng nhập theo `GET /me/tenants`, dựng workspace/menu sinh từ danh mục `specs/portal/menu-catalog.yaml` lọc theo `GET /me/permissions` và `disabled_menus`/`disabled_actions` (ADR-0090), đổi đơn vị làm việc bằng `X-Tenant-Id`, bộ lọc tenant của System Admin. FR nguồn: FR-AUTH-029, FR-AUTH-030, FR-CMS-009, FR-CMS-010, FR-CMS-011, FR-CMS-012, FR-CMS-018, FR-ORG-008.

## Clarifications

### Session 2026-11-16

- Q: Portal có gọi `POST /auth/session` (`createAuthSession`) sau đăng nhập không? → A: Không. Luồng sau đăng nhập theo [ADMIN_PORTAL §1.3b](../../docs/04-admin-portal/ADMIN_PORTAL.md) và [KEYCLOAK_SSO §3.3](../../docs/09-identity/KEYCLOAK_SSO.md): `listMyTenants` → (chọn tenant) → `getMyPermissions` → `getMe`. `createAuthSession` trả `PlayerBootstrap` (bắt buộc `game_profile`) không áp dụng cho nhân sự; ghi mâu thuẫn ở [contracts](contracts/README.md) §6.
- Q: Tenant của phiên cho `getMyPermissions` truyền bằng query `tenant_id` (ADMIN_PORTAL §2.1) hay header? → A: Theo contract: header `X-Tenant-Id` (`TenantIdHeaderOptional`, ADR-0065); vắng header = phạm vi platform.
- Q: ADR-0086 (giữ phiên game bằng refresh token) có đổi phiên portal không? → A: *(Thay bởi Session 2026-11-17 — ADR-0091: không còn `prompt=none`/`max_age`; tải lại trang dùng `refreshPortalToken` với cookie.)* Không đổi chính sách (idle 30 phút, tối đa 10 giờ — `portal_session_sso`; MFA theo role đã bỏ, ADR-0094); ADR-0086 mục 7 chốt: refresh token xoay vòng **chỉ trong bộ nhớ**, gia hạn im lặng, **tải lại trang không phải đăng nhập lại khi phiên còn hạn** (khởi tạo lại bằng phiên SSO Keycloak `prompt=none`). Vì realm SSO Session nâng lên 7 ngày/30 ngày cho client game, portal tự cưỡng chế idle/max: gửi `max_age` = 10 giờ, ghi `last_activity_at` (dấu thời gian, không phải secret) và quá 30 phút không hoạt động → RP-initiated logout hoặc `prompt=login` ([KEYCLOAK_SSO §5.2](../../docs/09-identity/KEYCLOAK_SSO.md)) [Confirmed — kiểm chứng trên Keycloak triển khai].
- Q: Alias Organization dùng cho scope `organization:{alias}` lấy ở đâu? → A: *(Bỏ theo ADR-0091 mục 4: portal không xin token theo scope `organization:{alias}`; delta `keycloak_org_alias` và task T004 không còn cần.)*
- Q: Banner "Đang tắt kiểm duyệt", chỉ báo trạng thái hệ thống, tìm kiếm toàn portal, chuông thông báo? → A: F051 chỉ dựng **vị trí** (slot) trên topbar; dữ liệu do feature khác (Ngoài phạm vi).

### Session 2026-11-17

- Q: Portal còn dùng `oidc-client-ts`/Authorization Code + PKCE và trang Keycloak? → A: Không ([ADR-0091](../../docs/adr/ADR-0091-dang-nhap-portal-bang-form-rieng-qua-backend.md)). Portal có trang `/login` tự vẽ (Ant Design, theme từ `design/admin-tokens`) gửi một request `portalLogin` gồm mật khẩu và (nếu nhập) mã 6 số; access token trả trong body và chỉ nằm trong bộ nhớ; refresh token nằm trong cookie `__Secure-ew_portal_rt` HttpOnly do backend đặt; `refreshPortalToken`/`portalLogout` gửi header `X-EW-CSRF` = giá trị cookie `__Host-ew_portal_csrf`. Ngoại lệ duy nhất dùng trang Keycloak: link đăng ký ứng dụng xác thực trong email.
- Q: Tải lại trang (F5) khôi phục phiên thế nào? → A: Gọi `refreshPortalToken` khi khởi động; còn phiên (idle 30 phút / tối đa 10 giờ — `portal_session_sso`) → token mới, giữ URL và tenant phiên (`sessionStorage`, không phải secret); hết phiên → `/login` giữ `returnUrl`.
- Q: Đổi đơn vị làm việc còn xin token mới? → A: Không; chỉ đổi `X-Tenant-Id` + tải lại `getMyPermissions` (backend kiểm membership — ADR-0065, ADR-0091 mục 4).
- Q: Menu/workspace khai báo ở đâu? → A: Sinh từ `specs/portal/menu-catalog.yaml` bằng `make portal-catalog` → `src/cms/frontend/apps/portal/src/generated/menu-catalog.ts` ([ADR-0090](../../docs/adr/ADR-0090-danh-muc-menu-action-portal-bat-tat-trong-db.md)); không khai báo menu tay trong TSX; workspace feature chỉ gắn component màn hình vào khóa menu. Menu ẩn khi thiếu permission hoặc nằm trong `disabled_menus`; nút action ẩn/khóa khi thiếu permission hoặc nằm trong `disabled_actions` (`<ActionGate actionKey>`); `403 FEATURE_DISABLED` → thông báo + tải lại quyền. Ẩn chỉ là tiện ích, backend chặn bằng `featuregate` (F012).
- Q: Phần đăng nhập portal có chờ kết quả spike Keycloak? → A: Có — F004 T039 (`amr`/`acr`, required action) trước T055–T059 [Confirmed].

### Session 2026-11-19 (triển khai)

Các điểm thiếu/mâu thuẫn gặp khi triển khai; lấy mặc định [Confirmed]; owner chốt 2026-11-20.

- OQ F051-1 — Đường dẫn route workspace? → A: [Confirmed] Lấy `path` của danh mục `specs/portal/menu-catalog.yaml` (`/school-lms`, `/content-studio`, `/game-ops`, `/live-ops`, `/platform-admin` — nguồn duy nhất, ADR-0090, khớp Assumptions), thay route `/school`, `/content`, `/platform` F050 đặt tạm; thư mục `src/workspaces/<slug>` giữ nguyên.
- OQ F051-2 — `.env.example` (T001)? → A: [Confirmed] Theo quyết định F050 (cấu hình runtime `/config.json`, mẫu `apps/portal/public/config.example.json`, `VITE_*` chỉ là fallback khi dev): bỏ `oidcIssuer`/`oidcClientId` (ADR-0091), thêm `principalCacheTtlSeconds` (`principal_cache_ttl`) và `sessionIdleTimeoutSeconds` (idle của `portal_session_sso`); không có `VITE_PORTAL_BASE_PATH` (portal phục vụ ở gốc origin).
- OQ F051-3 — `getMe` khai báo `TenantIdHeader` bắt buộc nhưng nhân sự platform không có tenant? → A: [Confirmed] Portal gọi `getMe` không kèm header ở phạm vi platform, chỉ dùng `display_name` cho user menu và hiển thị nhãn chung khi lỗi; sửa contract thuộc F012.
- OQ F051-4 — Breakpoint 1280 px chưa có trong `design/admin-tokens`? → A: [Confirmed] Hằng `DESKTOP_MIN_WIDTH_PX = 1280` và vùng chạm 44 px trong shell, trỏ ADMIN_PORTAL §1.5; 768 px lấy từ token Ant Design `screenMD`. Thêm token breakpoint là việc của design system.
- OQ F051-5 — ProLayout? → A: [Confirmed] Dùng `Layout` + `Menu` + `Drawer` của Ant Design 5 (cùng design system, kiểm soát a11y và bundle tốt hơn); ProLayout không bắt buộc cho shell.
- OQ F051-6 — Khóa per-viewer `ew.portal.<userSub>.*` cần `sub` trước khi gọi `getMe`? → A: [Confirmed] Đọc `sub` từ payload của access token (không xác minh, chỉ làm khóa lưu trữ); tùy chọn hiển thị giữ khóa F050 `ew.portal.prefs` (cần trước đăng nhập).
- OQ F051-7 — Nhiều tab làm mới đồng thời? → A: [Confirmed] Web Locks API (`navigator.locks`, khóa `ew.portal.refresh`) tuần tự hóa `refreshPortalToken` giữa tab; tab nhận `401 REFRESH_TOKEN_INVALID`/`403 CSRF_TOKEN_INVALID` khi cookie CSRF đã đổi thì đọc lại cookie và thử lại một lần; `BroadcastChannel` chỉ báo đăng xuất (không mang token).
- OQ F051-8 — Nhãn 232 action (`portal.action.*`)? → A: [Confirmed] F051 chỉ dịch nhãn workspace và menu (shell hiển thị); nhãn action do F059 (màn Bật/tắt) và feature workspace bổ sung.
- OQ F051-9 — E2E Playwright `@F051` với backend local thật (T015, T016, T027, T039, T044, T055–T057, T060) và kiểm cấu hình realm (T052)? → A: [Confirmed] Chưa chạy được: binary backend chưa nối (OQ F010-2), spike F004 T039 chưa xong. Mọi tiêu chí được kiểm ở mức unit/integration bằng MOCK backend hợp đồng (`src/test/mock-backend.ts`, gắn nhãn MOCK — R17); E2E chạy khi F004/F012 có môi trường.
- OQ F051-10 — Đếm `portal_login_route{result}`? → A: [Confirmed] Chưa làm: F050 chưa có kênh telemetry.
- OQ F051-11 — Vị trí test và hook? → A: [Confirmed] Theo quy ước F050: test đặt cạnh mã (`src/**/*.test.ts(x)`), hook API ở `src/shared/api/hooks/`.
- OQ F051-12 — Placeholder "Tất cả tenant" (màu mặc định Ant Design `#bfbfbf`, 1,83:1) bị axe chặn? → A: [Confirmed] Thêm seed `antd.colorTextPlaceholder` = `ops.color.text.secondary` vào `design/admin-tokens` (tokens@0.3.0, MINOR), như F050 đã làm với `colorText*`.

### Session 2026-11-20 (owner chốt)

- OQ F051-1…F051-12 (mặc định [Confirmed] chờ duyệt) → owner chốt 2026-11-20, giữ nguyên các quyết định trên. Riêng:
- OQ F051-3 → **Chốt:** `getMe` đổi sang header tùy chọn ở contract và backend (task ở F012); portal giữ cách gọi không header ở phạm vi platform.
- OQ F051-4 → **Chốt:** token breakpoint/vùng chạm vào `design/admin-tokens` (task ở F005).
- OQ F051-9 → **Chốt:** E2E Playwright với backend thật và kiểm realm (T015, T016, T027, T039, T044, T055–T057, T060, T052) chạy khi có môi trường compose (binary đã nối 2026-11-20) — không phải câu hỏi mở.
- OQ F051-10 → **Chốt:** không có telemetry trình duyệt cho đăng nhập trong Release 1.0; số liệu đăng nhập portal đo phía server bằng `eduworld_portal_login_total{result}` của module identity, bỏ `portal_login_route` ở plan.

### Session 2026-11-21 (owner bỏ MFA — [Confirmed])

- Q: Owner bỏ MFA/TOTP cho đăng nhập Admin Portal với mọi tài khoản, kể cả role platform (2026-11-21, [ADR-0094](../../docs/adr/ADR-0094-bo-mfa-totp-cho-dang-nhap-admin-portal.md)) — phần nào của feature này đổi? → A: F051-AC-13 sửa: bỏ hộp đăng nhập lại (step-up) và xử lý `MFA_REQUIRED`; `401 REFRESH_TOKEN_INVALID` (kể cả phiên `platform_staff` quá 8 giờ) → về `/login`. F051-AC-02, AC-29, AC-30, AC-31 sửa: form chỉ username/email + mật khẩu, không ô mã, không cờ "lần trước đã nhập mã", không màn "Kiểm tra email để cài ứng dụng xác thực". Nhắc OTP/MFA trong plan, data-model, contracts, tasks (T012, T026, T039, T055, T059), quickstart là lịch sử; phần UI do đợt sửa portal của ADR-0094 làm. Thay thế chung: route platform (`/admin/*`, `/game-admin/*`) chỉ còn gateway nội bộ + `azp = eduworld-portal` + `user_type = staff` + role platform (guard `platform_role`, thiếu → `403 PERMISSION_DENIED` + `authz.denied`); `CrossTenantScope` giữ `platform.cross_tenant.read` + client portal + `reason` + audit; phiên `platform_staff` bị từ chối `refreshPortalToken` sau 8 giờ từ lúc nhập mật khẩu (`EW_OIDC_PLATFORM_MAX_AUTH_AGE`, `401 REFRESH_TOKEN_INVALID`).

### Session 2026-11-21 (owner chốt giao diện — [Confirmed])

- Giao diện `/login` và các màn trong `AuthLayout` → **Chốt:** hướng A "Sân trường": hai cột (bảng thương hiệu nền `colorPrimary` với câu dẫn, chip môn học theo mã màu nghiệp vụ, hình trang trí SVG `aria-hidden`; cột phải thẻ "Chào mừng thầy cô trở lại", ô cao 48 px, chữ ô nhập 16 px, bo 12 px qua theme `authTheme` sinh từ `design/admin-tokens`), một cột ở chiều rộng điện thoại; hộp gợi ý "Em là học sinh? Hãy mở game EduWorld để vào học nhé.", dòng bảo vệ dữ liệu (Nghị định 13/2023, đăng xuất trên máy dùng chung). Link chính sách bảo mật và link game **không** hiển thị vì cấu hình runtime chưa có khóa URL (R3) — chỉ giữ câu chữ. Cập nhật [ADMIN_PORTAL §1.3b](../../docs/04-admin-portal/ADMIN_PORTAL.md), [DESIGN_SYSTEM §4.1](../../docs/05-ux/DESIGN_SYSTEM.md).
- Banner cột trái của `/login` → **Chốt (owner 2026-11-21, bổ sung):** banner nói rõ cổng quản trị vận hành toàn bộ nền tảng: tiêu đề "Cổng quản trị EduWorld", dòng phụ "Soạn nội dung học tập, quản lý thế giới game và theo dõi học sinh — trong một nơi.", hình minh họa hình khối (thẻ câu hỏi, sách, ghim bản đồ có ngôi sao, biểu đồ tiến bộ; màu token, `aria-hidden`) và danh sách 5 năng lực có icon (ngân hàng câu hỏi & đề thi; bài học & hoạt động tương tác; thế giới game; giao bài theo khối & theo dõi tiến bộ; an toàn & kiểm duyệt) thay cho chip môn học; chữ "khối", không "lớp" (ADR-0095); điện thoại chỉ còn logo + tiêu đề. Thẻ đăng nhập bên phải không đổi (dòng phụ đổi thành "Đăng nhập để tiếp tục công việc của bạn.").
- Mã xác thực 6 số trên portal → **Chốt (owner 2026-11-21):** bỏ hẳn: không ô mã, không link "Tôi có mã xác thực", không cờ `otpHint` trên `/login`; bỏ hẳn hộp đăng nhập lại (step-up), xử lý `MFA_REQUIRED` và màn "Kiểm tra email để cài ứng dụng xác thực" (`configure_otp`); `portalLogin` chỉ gửi `username`, `password`, `device_id`, `locale`; thông điệp lỗi chung "Tên đăng nhập hoặc mật khẩu chưa đúng." Phía backend/realm theo [ADR-0094](../../docs/adr/ADR-0094-bo-mfa-totp-cho-dang-nhap-admin-portal.md) (thay ADR-0091 mục 2); ADR-0091 mục 1 (một form, một request, lỗi chung, không hỏi trước điều gì về tài khoản) vẫn giữ. F051-AC-01 (bố cục) sửa ở đây; AC-13, AC-30, AC-31 theo Session "owner bỏ MFA".

### Session 2026-11-22 (E2E Portal v2 — ADR-0096, Proposed)

- Q: Học sinh bấm đăng nhập portal lần hai thì thấy "Tên đăng nhập hoặc mật khẩu chưa đúng" thay vì thông báo dành cho học sinh — portal có phải đổi gì? → A: Không đổi portal. Nguyên nhân ở realm (Keycloak đếm lần từ chối của flow là đăng nhập sai, khóa tạm tài khoản, kể cả game); sửa theo [ADR-0096](../../docs/adr/ADR-0096-chan-hoc-sinh-o-portal-khong-tinh-la-dang-nhap-sai.md) ở realm và module identity (F004, F012-AC-29). Sau sửa, mỗi lần học sinh nhập đúng mật khẩu ở portal đều nhận `403 PORTAL_ACCESS_STAFF_ONLY` và portal hiện cảnh báo dành cho học sinh (F051-AC-04); mật khẩu sai vẫn `401 INVALID_CREDENTIALS`.

- Q: `ad.truong.a`, `ad.truong.b`, `qlcm.truong.a` thấy menu workspace D "Báo cáo học tập toàn platform" rồi gặp "Ngoài phạm vi quyền" — sửa ở đâu? → A: Danh mục menu (ADR-0090, AGENTS R20): `menu.platform_admin.reports` yêu cầu any-of `[platform.cross_tenant.read, report.export]`, mà `report.export` là quyền tenant nên `getMyPermissions` mở workspace D cho role tenant. Menu chỉ còn `[platform.cross_tenant.read]` (action `export` giữ `report.export`, backend vẫn chỉ cho System Admin); `src/generated/menu-catalog.ts` sinh lại. Chốt chặn: test `TestPlatformWorkspacesNeedPlatformPermissions` (module identity) — mọi menu của `game_ops`, `live_ops`, `platform_admin` chỉ dùng quyền platform-only (`authz.Permission.PlatformOnly`) và không role tenant hệ thống nào thấy workspace platform.

### Session 2026-11-22 (ADR-0094 — [Confirmed])

- Q: Văn bản còn mô tả MFA/`MFA_REQUIRED`/step-up như hành vi hiện hành? → A: [Confirmed] Viết lại theo [ADR-0094](../../docs/adr/ADR-0094-bo-mfa-totp-cho-dang-nhap-admin-portal.md), giữ nguyên mã AC: Clarifications Session 2026-11-16 về `portal_session_sso` (chú thích MFA theo role đã bỏ). Route platform chỉ còn gateway nội bộ + `azp = eduworld-portal` + `user_type = staff` + role platform; thiếu → `403 PERMISSION_DENIED` + `authz.denied`; không còn `403 MFA_REQUIRED` hay hộp đăng nhập lại (step-up). Câu trả lời Clarifications cũ nhắc MFA là lịch sử.

### Session 2026-11-22 (ADR-0098 — owner yêu cầu, [Confirmed])

- Q: Owner muốn sidebar đổi tông màu, header và sidebar cố định khi cuộn, ngôn ngữ và sáng/tối là nút nhanh trên header, và dùng thêm Tailwind CSS — làm trong phạm vi nào? → A: [Confirmed] Chỉ shell (`PortalLayout`), không đổi dữ liệu hay hành vi màn: sidebar tông xanh thương hiệu đậm qua token `ops.color.sider.*` (`Menu theme="dark"`, tương phản ≥ 4,5:1; DESIGN_SYSTEM §4.1); trang không cuộn, chỉ vùng nội dung cuộn, drawer dưới 1280 px giữ nguyên (F051-AC-19); nút địa cầu + `VI`/`EN` và mặt trăng/mặt trời trên topbar, lưu cùng `ew.portal.prefs` (F051-AC-16/17); user menu còn tên, mật độ gọn, đăng xuất; Tailwind v3 làm lớp tiện ích bố cục theo [ADR-0098](../../docs/adr/ADR-0098-tailwind-css-lam-lop-tien-ich-bo-cuc-cho-admin-portal.md) (Preflight tắt, theme chỉ trỏ token, cấm arbitrary value). Test: `src/shell/layout.test.tsx`, `src/app/tailwind.test.ts`.

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Đăng nhập portal và vào đúng phạm vi (Priority: P1)

Nhân sự mở portal, đăng nhập bằng username hoặc email trên trang `/login` do portal tự vẽ, chỉ username/email + mật khẩu, không mã xác thực với bất kỳ tài khoản nào (ADR-0091, ADR-0094). Không chọn trường trước. Sau đăng nhập portal tự đưa vào phạm vi platform, vào thẳng tenant duy nhất, hiện màn "Chọn đơn vị làm việc", hoặc màn "chưa được cấp quyền". Học sinh bị chặn.

**Why this priority**: Không có phiên thì mọi workspace khác không chạy; là exit M-C1.

**Independent Test**: Với backend F012 thật, đăng nhập lần lượt `ad.truong.a`, `qlcm.truong.a`, `sa.admin`, `hs.an.3a` và kiểm tra màn đích.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F051-AC-01 | KHI người dùng mở portal chưa có phiên (và `refreshPortalToken` lúc khởi động trả `401`), Portal PHẢI hiện trang `/login` do portal tự vẽ (Ant Design, theme từ `design/admin-tokens`) gồm ô "Tên đăng nhập hoặc email", "Mật khẩu" (nút hiện/ẩn), "Quên mật khẩu?" và chọn ngôn ngữ (bố cục hướng A "Sân trường", không có ô mã xác thực); PHẢI KHÔNG chuyển hướng sang Keycloak, không có bước chọn tenant/trường trước đăng nhập và không có URL theo tenant. | FR-AUTH-029, FR-AUTH-030 · ADR-0091 mục 1 · [ADMIN_PORTAL §1.3b](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-AUTH-071, TC-AUTH-093 |
| F051-AC-02 | Portal PHẢI giữ access token chỉ trong bộ nhớ tiến trình (không `localStorage`, `sessionStorage`, IndexedDB hay cookie do portal ghi); refresh token PHẢI chỉ nằm trong cookie HttpOnly do backend đặt và mã JavaScript của portal PHẢI không đọc/ghi được nó; Portal PHẢI không lưu mật khẩu, `setup_ticket` hay token đặt lại mật khẩu vào bất kỳ kho trình duyệt nào. | ADR-0091 mục 4 · AC-AUTH-036 · [KEYCLOAK_SSO §5](../../docs/09-identity/KEYCLOAK_SSO.md) | TC-AUTH-098 |
| F051-AC-03 | KHI đăng nhập xong, Portal PHẢI gọi `listMyTenants` không gửi `X-Tenant-Id` và định tuyến: `has_platform_role = true` → phạm vi platform; đúng 1 tenant → vào thẳng; > 1 tenant → màn "Chọn đơn vị làm việc" (chọn sẵn tenant đã nhớ); rỗng và không có role platform → màn "Tài khoản chưa được cấp quyền quản trị" có nút Đăng xuất. | FR-AUTH-030, AC-AUTH-012 | TC-AUTH-072 |
| F051-AC-04 | NẾU `portalLogin` hoặc API trả `403 PORTAL_ACCESS_STAFF_ONLY` (tài khoản học sinh bị chặn ở Keycloak sau bước mật khẩu, hoặc token học sinh lọt qua Keycloak), THÌ Portal PHẢI hiện thông điệp theo [ADMIN_PORTAL §1.3b](../../docs/04-admin-portal/ADMIN_PORTAL.md), không render workspace nào và đăng xuất. | FR-AUTH-029, AC-AUTH-025 | TC-AUTH-070, TC-AUTH-095 |
| F051-AC-05 | KHI người dùng chọn tenant (màn chọn hoặc lựa chọn đã nhớ), Portal PHẢI dùng tiếp access token hiện có (không xin token mới, không scope `organization:{alias}`), gửi `X-Tenant-Id` = tenant đã chọn ở mọi request tenant-scoped và tải `getMyPermissions` của tenant đó. | FR-AUTH-030 · ADR-0091 mục 4 · ADR-0065 | TC-CMS-LOGIN-02, TC-AUTH-101 |
| F051-AC-06 | TRONG KHI phiên ở một tenant, Portal PHẢI gửi `X-Tenant-Id` = tenant phiên ở mọi request tenant-scoped và không bao giờ hiển thị dữ liệu của hai tenant trong cùng một màn. | FR-ORG-008, FR-CMS-010 · ADR-0065 | TC-TEN-011 |
| F051-AC-07 | NẾU access token không làm mới được (`refreshPortalToken` trả `401 REFRESH_TOKEN_INVALID` hoặc `403 CSRF_TOKEN_INVALID`, hoặc hết `portal_session_sso`), THÌ Portal PHẢI hiện hộp "Phiên làm việc đã hết, vui lòng đăng nhập lại", xóa access token và cache, và sau đăng nhập lại ở `/login` đưa về đúng URL trước đó. | [ADMIN_PORTAL §1.3b](../../docs/04-admin-portal/ADMIN_PORTAL.md) | — |
| F051-AC-08 | KHI người dùng chọn Đăng xuất, Portal PHẢI gọi `portalLogout` kèm `X-EW-CSRF` (backend thu hồi refresh token, phiên Keycloak và xóa cookie), rồi xóa access token, toàn bộ cache truy vấn và tenant phiên khỏi bộ nhớ và `sessionStorage`, và về `/login`; lỗi mạng khi đăng xuất vẫn PHẢI xóa trạng thái phía trình duyệt. | ADR-0091 mục 1, 4 · [KEYCLOAK_SSO §7](../../docs/09-identity/KEYCLOAK_SSO.md) | TC-AUTH-098 |
| F051-AC-28 | KHI người dùng tải lại trang (F5) hoặc mở lại tab trong lúc phiên portal còn hạn (chưa quá idle 30 phút và tối đa 10 giờ theo `portal_session_sso`), Portal PHẢI khôi phục phiên im lặng bằng `refreshPortalToken` (cookie refresh + header `X-EW-CSRF` đọc từ cookie CSRF), giữ URL hiện tại và tenant phiên (`sessionStorage`), không hiện trang đăng nhập; access token vẫn chỉ trong bộ nhớ (AC-02). | ADR-0091 mục 4 · ADR-0086 mục 7 · AC-AUTH-029 | TC-AUTH-089 |
| F051-AC-29 | NẾU không có tương tác quá 30 phút hoặc phiên quá 10 giờ (kể cả khi tab bị đóng rồi mở lại), THÌ Portal PHẢI kết thúc phiên (`IdleGuard` ngừng làm mới và gọi `portalLogout`; khi khởi động lại `refreshPortalToken` trả `401` vì client session Keycloak hết hạn) và đưa người dùng về `/login` (username + mật khẩu; phiên `platform_staff` cũng kết thúc khi `refreshPortalToken` trả `401` sau 8 giờ từ lúc nhập mật khẩu — ADR-0094), sau đăng nhập về đúng URL trước đó. | ADR-0091 mục 4 · ADR-0086 mục 7 · AC-AUTH-029 · `portal_session_sso` | TC-AUTH-089 |
| F051-AC-30 | KHI người dùng bấm "Đăng nhập", Portal PHẢI gửi đúng một request `portalLogin` gồm `username`, `password`, `device_id` (UUID ngẫu nhiên per-trình duyệt, không phải secret) và `locale` — không có ô hay trường mã xác thực (sửa theo ADR-0094); NẾU nhận `401 INVALID_CREDENTIALS` THÌ Portal PHẢI hiện đúng một thông điệp chung "Tên đăng nhập hoặc mật khẩu chưa đúng" và xóa ô mật khẩu; `429 RATE_LIMITED` PHẢI hiện thời gian chờ theo `Retry-After`. | FR-AUTH-029 · AC-AUTH-033 · ADR-0091 mục 1 · ADR-0094 | TC-AUTH-093, TC-AUTH-094 |
| F051-AC-31 | NẾU `portalLogin` trả `403 ACCOUNT_SETUP_REQUIRED`, THÌ Portal PHẢI chuyển sang màn "Thiết lập tài khoản" (giữ `setup_ticket` chỉ trong bộ nhớ): bước `update_password` → form mật khẩu mới + nhập lại (gợi ý policy) gọi `setupPortalPassword`, hiển thị lỗi `PASSWORD_POLICY_VIOLATION` theo field (bước `configure_otp` và màn "Kiểm tra email để cài ứng dụng xác thực" bỏ theo ADR-0094); `401 SETUP_TICKET_INVALID` → về `/login` với thông báo hết hạn. | FR-AUTH-029 · AC-AUTH-034 · ADR-0091 mục 3 · ADR-0094 | TC-AUTH-096 |
| F051-AC-32 | KHI người dùng gửi form `/forgot-password`, Portal PHẢI gọi `forgotPortalPassword` và luôn hiện cùng một thông điệp xác nhận dù tài khoản có tồn tại hay không; KHI mở `/reset-password?token=…`, Portal PHẢI đọc token rồi xóa khỏi URL ngay (`history.replaceState`), đặt `Referrer-Policy: no-referrer`, không gửi token vào telemetry, gọi `resetPortalPassword` với mật khẩu mới và chuyển về `/login` khi thành công; `401 PASSWORD_RESET_TOKEN_INVALID` → thông báo link không còn hiệu lực + liên kết yêu cầu lại. | FR-AUTH-029 · AC-AUTH-035 · ADR-0091 mục 3 | TC-AUTH-097 |
| F051-AC-33 | Portal PHẢI làm mới access token trước khi hết hạn và khi một request nhận `401 TOKEN_EXPIRED` bằng đúng một lời gọi `refreshPortalToken` dùng chung cho mọi request đang chờ (single-flight), gửi `X-EW-CSRF` = giá trị cookie `__Host-ew_portal_csrf`, rồi thử lại request gốc tối đa một lần; mọi tab mở song song PHẢI không làm mất phiên của nhau (tab nhận `401 REFRESH_TOKEN_INVALID` do tab khác đã xoay vòng PHẢI đọc lại cookie CSRF và thử lại một lần). | ADR-0091 mục 4 · AC-AUTH-036 | TC-AUTH-098 |

---

### User Story 2 — Shell theo quyền: workspace, menu, lỗi quyền (Priority: P1)

Sau khi có phạm vi, portal dựng workspace switcher và sidebar từ danh mục menu/action sinh từ `specs/portal/menu-catalog.yaml`, lọc theo permission và trạng thái bật/tắt backend trả về; mỗi workspace là chunk lazy. Mở URL không có quyền thấy trang 403 của portal. Ngoài VPN thì thấy thông báo cần VPN (không còn đăng nhập lại có MFA — ADR-0094).

**Why this priority**: Là lớp hiển thị bắt buộc của ADR-0027 (4 lớp chặn) và là khung cho F052–F057, F079, F122.

**Independent Test**: `qlcm.truong.a` đăng nhập → network log chỉ có chunk `school-lms`, `content-studio`; mở `/game-ops` → trang 403.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F051-AC-09 | KHI phạm vi phiên đã xác định, Portal PHẢI gọi `getMyPermissions` và dựng workspace switcher, sidebar từ danh mục sinh từ catalog lọc theo `workspaces[]`, `permissions[]` và `disabled_menus[]` trả về; mục menu thiếu permission hoặc đang tắt không hiển thị. | FR-CMS-012 · FR-CMS-018 | TC-CMS-AUTHZ-06, TC-CMS-MENU-06 |
| F051-AC-10 | Portal PHẢI đóng gói mỗi workspace (`school_lms`, `content_studio`, `game_ops`, `live_ops`, `platform_admin`) thành chunk lazy riêng và chỉ tải chunk khi `workspaces[]` chứa workspace đó; bundle khởi đầu không chứa mã workspace nào. | FR-CMS-009, FR-CMS-011 · ADR-0027 | TC-CMS-WS-01 |
| F051-AC-11 | NẾU người dùng mở trực tiếp URL của workspace hoặc màn không có trong permission, THÌ Portal PHẢI hiện trang 403 của portal và không gọi API của màn đó. | FR-CMS-011, AC-CMS-001 | TC-CMS-AUTHZ-05 |
| F051-AC-12 | Portal PHẢI làm mới `getMyPermissions` theo chu kỳ không dài hơn `principal_cache_ttl`, khi cửa sổ được focus lại và ngay sau khi nhận `403 PERMISSION_DENIED`, để menu phản ánh thay đổi role/UserRole. | FR-CMS-012 · ADR-0066 | TC-CMS-RBAC-03 |
| F051-AC-13 | Portal PHẢI KHÔNG có hộp đăng nhập lại (step-up) và PHẢI KHÔNG xử lý riêng mã `MFA_REQUIRED` (mã đã bỏ — ADR-0094); KHI `refreshPortalToken` trả `401 REFRESH_TOKEN_INVALID` (kể cả phiên `platform_staff` quá 8 giờ), Portal PHẢI về `/login` và giữ URL để quay lại sau đăng nhập. | ADR-0091 mục 4 · ADR-0094 · [KEYCLOAK_SSO §3.3](../../docs/09-identity/KEYCLOAK_SSO.md) | TC-CMS-WS-02, TC-AUTH-101 |
| F051-AC-14 | NẾU request của workspace `game_ops`, `live_ops` hoặc `platform_admin` tới gateway nội bộ lỗi kết nối mạng, THÌ Portal PHẢI hiện thông báo "Cần kết nối mạng quản trị (VPN)" thay cho lỗi chung. | [ADMIN_PORTAL §1.2](../../docs/04-admin-portal/ADMIN_PORTAL.md) | — |
| F051-AC-15 | KHI API trả Problem Details, Portal PHẢI hiển thị thông điệp i18n theo `code` kèm `trace_id`, và hiển thị thông điệp chung cho `code` lạ. | [ADMIN_PORTAL §10.1](../../docs/04-admin-portal/ADMIN_PORTAL.md) | — |
| F051-AC-16 | Portal PHẢI hiển thị mọi chuỗi UI qua i18n `vi` (mặc định) / `en` với khóa `portal.<workspace>.<screen>.<element>`, đổi ngôn ngữ bằng nút nhanh trên topbar (owner 2026-11-22, trước đây ở user menu) không tải lại trang, định dạng số/ngày/giờ theo locale. | NFR-I18N-003 · ADR-0056 | TC-I18N-006, TC-I18N-007 |
| F051-AC-17 | Portal PHẢI chỉ dùng theme Ant Design và CSS variables sinh từ `design/admin-tokens` (sáng/tối chọn bằng nút nhanh trên topbar, compact chọn ở user menu — owner 2026-11-22), không hardcode màu/font/khoảng cách và không import component/Theme của game. | FR-CMS-009, NFR-MAINT-007 · ADR-0047 | TC-UI-003, TC-UI-010 |
| F051-AC-18 | Portal PHẢI đạt WCAG 2.1 AA cho shell (điều hướng bàn phím, nhãn, landmark, focus rõ) ở 3 breakpoint. | NFR-A11Y-003 | TC-UI-006 |
| F051-AC-19 | Ở NƠI độ rộng màn hình < 768 px, Portal PHẢI chuyển sidebar thành drawer, đưa bộ chọn/bộ lọc tenant vào menu ☰, giữ banner hệ thống và vùng chạm ≥ 44 px; < 1280 px sidebar thành drawer. | [ADMIN_PORTAL §1.5](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-UI-004 |
| F051-AC-34 | Portal PHẢI sinh `workspaceRegistry`/`menuRegistry` và kiểu khóa action từ `specs/portal/menu-catalog.yaml` bằng `make portal-catalog` (file `src/cms/frontend/apps/portal/src/generated/menu-catalog.ts`, không sửa tay); mã TSX PHẢI KHÔNG khai báo mục menu, path hay permission của menu bằng tay (workspace feature chỉ gắn component màn hình vào khóa menu có sẵn); CI PHẢI fail khi file sinh lệch catalog hoặc khi một khóa menu không có component tương ứng. | FR-CMS-018 · FR-CMS-012 · ADR-0090 mục 1 | TC-CMS-MENU-05 |
| F051-AC-35 | Portal PHẢI chỉ hiển thị một menu khi workspace của nó có trong `workspaces[]`, người dùng có ít nhất một permission trong `required_permissions` của menu và khóa menu không nằm trong `disabled_menus[]`; workspace không còn menu hiển thị PHẢI bị ẩn; NẾU người dùng mở trực tiếp URL của menu đang tắt, THÌ Portal PHẢI hiện trang "Tính năng đang tắt" (khác trang 403) và không gọi API của màn đó. | FR-CMS-018 · ADR-0090 mục 3 | TC-CMS-MENU-02, TC-CMS-MENU-06 |
| F051-AC-36 | Portal PHẢI cung cấp component `<ActionGate actionKey>` (khóa kiểu sinh từ catalog — khóa không tồn tại gây lỗi biên dịch) ẩn nút/thao tác khi người dùng thiếu mọi permission trong `required_permissions` của action hoặc khi khóa nằm trong `disabled_actions[]`, hoặc ở chế độ `mode="disable"` thì khóa nút kèm tooltip "Tính năng đang tắt"; mọi workspace feature PHẢI dùng `ActionGate` cho thao tác thuộc catalog. | FR-CMS-018 · ADR-0090 mục 3 | TC-CMS-MENU-06 |
| F051-AC-37 | NẾU API trả `403 FEATURE_DISABLED`, THÌ Portal PHẢI hiện thông báo "Tính năng đang tắt" (kèm `feature_key`, `trace_id`), không đăng xuất, và tải lại `getMyPermissions` ngay để ẩn menu/action tương ứng. | FR-CMS-018 · ADR-0090 mục 3 | TC-CMS-MENU-02 |

---

### User Story 3 — Đổi đơn vị làm việc trên topbar (Priority: P2)

Nhân sự thuộc nhiều tenant (`qlcm.truong.a`) đổi đơn vị ở topbar mà không nhập lại mật khẩu; dữ liệu tenant cũ biến mất hoàn toàn.

**Why this priority**: Cần cho nhân sự nhiều tenant; ít người dùng hơn US1–US2.

**Independent Test**: `qlcm.truong.a` vào `th-demo-a`, đổi sang `pgd-demo`, kiểm tra header mới, menu mới, request với header cũ bị từ chối.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F051-AC-20 | KHI nhân sự nhiều tenant chọn đơn vị khác ở topbar, Portal PHẢI cảnh báo nếu có bản nháp chưa lưu, rồi (không xin token mới — AC-05) đổi `X-Tenant-Id`, xóa toàn bộ cache truy vấn của tenant cũ, tải lại `getMyPermissions` và đưa về trang tổng quan của workspace. | FR-AUTH-030, AC-AUTH-012 | TC-TEN-008 |
| F051-AC-21 | NẾU API trả `403 TENANT_MISMATCH` hoặc `403 TENANT_ACCESS_DISABLED`, THÌ Portal PHẢI hủy truy vấn đang chạy, thông báo "Đơn vị làm việc đã thay đổi quyền truy cập" và về màn chọn đơn vị (hoặc màn chưa cấp quyền nếu `listMyTenants` rỗng). | AC-AUTH-024 · ADR-0065 | TC-CMS-LOGIN-03 |

---

### User Story 4 — Bộ lọc tenant của System Admin (Priority: P2)

System Admin thấy dropdown tìm kiếm tenant trên topbar (mặc định "Tất cả tenant", chọn nhiều), cột Tenant ở danh sách; thao tác ghi luôn chọn một tenant đích.

**Why this priority**: Cần cho System Admin ở workspace B, C, D; người dùng tenant không phụ thuộc.

**Independent Test**: `sa.admin` lọc `th-demo-a` + `pgd-demo`, mở một danh sách hỗ trợ `tenant_ids`, kiểm tra query và cột Tenant; `ad.truong.a` không thấy bộ lọc.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F051-AC-22 | TRONG KHI người dùng có permission `platform.cross_tenant.read`, Portal PHẢI hiển thị bộ lọc tenant dạng dropdown tìm kiếm (nguồn `listTenants`, tìm theo mã/tên), mặc định "Tất cả tenant", chọn nhiều, lưu theo phiên trình duyệt. | FR-CMS-013, FR-ORG-008 | TC-TEN-018 |
| F051-AC-23 | KHI System Admin mở danh sách có tham số `tenant_ids`, Portal PHẢI không gửi `X-Tenant-Id`, gửi `tenant_ids[]` theo bộ lọc (vắng = mọi tenant) và hiển thị cột Tenant (tên + loại `school`/`unit`/`common`). | AC-CMS-003, AC-ORG-004 | TC-CMS-TEN-02 |
| F051-AC-24 | Ở NƠI màn hình dùng endpoint chỉ nhận `X-Tenant-Id` (không có `tenant_ids`), Portal PHẢI yêu cầu System Admin chọn đúng một tenant trước khi tải dữ liệu và gửi `X-Tenant-Id` = tenant đó. | [ADMIN_PORTAL §2.5](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-TEN-004 |
| F051-AC-25 | KHI System Admin thực hiện thao tác ghi, Portal PHẢI bắt buộc chọn một tenant đích (hoặc phạm vi Platform nếu màn cho phép) bằng trường "Tenant" của form và gửi `X-Tenant-Id` = tenant đích; không có lựa chọn "tất cả tenant" cho thao tác ghi. | FR-CMS-013, AC-CMS-003 | TC-CMS-TEN-03 |
| F051-AC-26 | TRONG KHI người dùng không có `platform.cross_tenant.read`, Portal PHẢI ẩn bộ lọc tenant và cột Tenant và không bao giờ gửi `tenant_ids`. | AC-CMS-002 | TC-CMS-TEN-01 |
| F051-AC-27 | Portal PHẢI giới hạn độ dài mọi ô nhập của shell (tìm tenant, tìm đơn vị) theo `maxLength` của tham số tương ứng trong OpenAPI. | ADR-0041 | TC-TEN-024 |


### Edge Cases

- NẾU trình duyệt chặn cookie của portal, THÌ đăng nhập vẫn thành công trong tab hiện tại nhưng tải lại trang phải đăng nhập lại; Portal hiện gợi ý bật cookie cho portal.
- NẾU `/reset-password` được mở khi đang có phiên, THÌ Portal đăng xuất phiên hiện tại trước khi đặt lại mật khẩu (backend đằng nào cũng thu hồi phiên).
- NẾU `disabled_menus` chứa menu đang mở, THÌ màn hiện "Tính năng đang tắt" thay vì 403 và không gọi API của màn đó (AC-35).
- NẾU `listMyTenants` trả lỗi mạng hoặc 5xx, THÌ Portal PHẢI hiện trang lỗi có nút Thử lại, không vào workspace (không đoán tenant).
- NẾU `refreshPortalToken` khi tải lại trang trả `401`/`403 CSRF_TOKEN_INVALID`, THÌ Portal PHẢI chuyển tới `/login` với `returnUrl` thay vì hiện lỗi (AC-28, AC-29).
- NẾU `localStorage` không dùng được (chế độ riêng tư, bị chặn), THÌ Portal PHẢI coi `last_activity_at` là không có, sinh `device_id` mới cho mỗi lần tải và chỉ dựa vào client session idle/max của Keycloak (không chặn đăng nhập).
- NẾU tenant đã nhớ không còn trong `listMyTenants`, THÌ Portal PHẢI bỏ lựa chọn nhớ và hiện màn chọn đơn vị (AC-03).
- NẾU `getMyPermissions` trả `workspaces[]` rỗng, THÌ Portal PHẢI hiện màn chưa cấp quyền (AC-03).
- NẾU người dùng mở nhiều tab và đổi tenant ở một tab, THÌ tab còn lại PHẢI xử lý `TENANT_MISMATCH` theo AC-21 (không trộn dữ liệu).
- NẾU đổi ngôn ngữ giao diện, THÌ ngôn ngữ nội dung đang soạn KHÔNG đổi ([ADMIN_PORTAL §1.5](../../docs/04-admin-portal/ADMIN_PORTAL.md) biên tập song ngữ).
- Shell không có thao tác ghi dữ liệu nghiệp vụ: không có yêu cầu Idempotency-Key ở F051; audit `cross_tenant.accessed` do backend ghi khi nhận `tenant_ids` (AC-23).

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-AUTH-029 | Trang `/login` tự vẽ (username/email + mật khẩu một request, lỗi chung; không mã xác thực — ADR-0094), thiết lập tài khoản lần đầu, quên/đặt lại mật khẩu, phiên bằng access token trong bộ nhớ + cookie refresh + CSRF, chặn học sinh `PORTAL_ACCESS_STAFF_ONLY` | [KEYCLOAK_SSO §3.3](../../docs/09-identity/KEYCLOAK_SSO.md) · [ADMIN_PORTAL §1.3b](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-AUTH-030 | Định tuyến sau đăng nhập theo `listMyTenants`, màn chọn đơn vị, đổi đơn vị trên topbar bằng `X-Tenant-Id` (không token mới) | [ADMIN_PORTAL §1.3b](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-CMS-009 | Một ứng dụng `src/cms/frontend/apps/portal`, 5 workspace lazy, Admin/Ops UI theo token | [ADMIN_PORTAL §1.1–§1.2](../../docs/04-admin-portal/ADMIN_PORTAL.md), [DESIGN_SYSTEM §4](../../docs/05-ux/DESIGN_SYSTEM.md) |
| FR-CMS-010 | Người dùng tenant làm việc trong đúng một tenant (header `X-Tenant-Id`) | [MULTITENANCY](../../docs/06-architecture/MULTITENANCY.md) |
| FR-CMS-011 | Lớp hiển thị: không menu/chunk workspace platform cho người dùng tenant, trang 403 | ADR-0027 mục 3, 7 |
| FR-CMS-012 | Menu từ `getMyPermissions`, làm mới khi quyền đổi | [ADMIN_PORTAL §2.1](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-CMS-018 | Menu/workspace sinh từ catalog, ẩn menu tắt (`disabled_menus`), `ActionGate` cho action tắt (`disabled_actions`), xử lý `403 FEATURE_DISABLED` | [ADMIN_PORTAL](../../docs/04-admin-portal/ADMIN_PORTAL.md) · ADR-0090 |
| FR-ORG-008 | Một tenant tại một thời điểm; bộ lọc tenant của System Admin; ghi chọn tenant đích | [ADMIN_PORTAL §2.5](../../docs/04-admin-portal/ADMIN_PORTAL.md) |

### Key Entities

- **Phiên portal**: người dùng đã xác thực, phạm vi (platform hoặc một tenant), access token trong bộ nhớ, refresh token trong cookie HttpOnly (backend quản lý), giá trị CSRF đọc từ cookie.
- **Danh mục menu/action**: cây workspace → menu → action sinh từ `specs/portal/menu-catalog.yaml` (khóa, path, nhãn i18n, `required_permissions`).
- **Trạng thái bật/tắt hiệu lực**: `disabled_menus[]`, `disabled_actions[]` của phạm vi phiên (từ `getMyPermissions`).
- **Đơn vị làm việc (tenant phiên)**: tenant người dùng đang thao tác; quyết định header `X-Tenant-Id`.
- **Tập quyền hiệu lực**: role, permission, workspace của người dùng trong phạm vi phiên (từ backend).
- **Bộ lọc tenant**: danh sách tenant System Admin đang xem (rỗng = tất cả), chỉ cho đọc.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `portal_session_sso` | Hành vi hết phiên, idle 30 phút / tối đa 10 giờ (`IdleGuard` + client session Keycloak), khôi phục phiên khi tải lại trang bằng `refreshPortalToken`, đăng nhập lại khi `auth_time` quá hạn ở route platform |
| `portal_login_rate` | Hiển thị thời gian chờ khi `429 RATE_LIMITED` ở `/login`, quên mật khẩu |
| `feature_toggle_cache_ttl` | Độ trễ tối đa từ khi System Admin tắt menu/action đến khi backend chặn (menu ẩn ở lần làm mới `getMyPermissions` kế tiếp) |
| `principal_cache_ttl` | Chu kỳ tối đa làm mới `getMyPermissions` (AC-12) |
| `access_token_ttl` | Lịch làm mới token im lặng (`refreshPortalToken` trước khi hết hạn) |
| `tenant_header_required` | Route nào gửi/không gửi `X-Tenant-Id` |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Ẩn menu/chunk không phải lớp bảo mật; mọi chặn quyền ở backend | AGENTS R1, R20 · ADR-0027 |
| Không hardcode URL, realm, client id (đọc từ env) | AGENTS R3, R4 |
| Theme chỉ từ token; tách hẳn design system game | AGENTS R32 · ADR-0047 · NFR-MAINT-007 |
| WCAG 2.1 AA | NFR-A11Y-003 |
| Không log token, PII trong console/telemetry | AGENTS R14 |
| Không log/gửi telemetry mật khẩu, `setup_ticket`, token đặt lại mật khẩu; xóa token khỏi URL | AGENTS R14 · ADR-0091 mục 5 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: 100% trong 4 kịch bản đăng nhập (1 tenant, 2 tenant, chỉ platform, không còn quyền) và kịch bản học sinh đưa người dùng tới đúng màn đích.
- **SC-002**: Người dùng tenant không tải được bất kỳ chunk nào của `game_ops`, `live_ops`, `platform_admin` (0 request trong network log).
- **SC-003**: Đổi đơn vị làm việc hoàn tất không cần nhập lại mật khẩu và 0 request nào sau đó mang header tenant cũ.
- **SC-004**: Thu hồi một UserRole làm menu thay đổi trong ≤ `principal_cache_ttl` + một chu kỳ làm mới.
- **SC-005**: 0 lỗi axe mức serious/critical ở shell; 0 vi phạm lint hardcode style.
- **SC-006**: 0 khai báo menu tay trong TSX (CI so file sinh với catalog); 100% menu trong `disabled_menus` không hiển thị sau một lần làm mới `getMyPermissions`.
- **SC-007**: Tải lại trang khi phiên còn hạn không hiện `/login` trong 100% lượt thử E2E; 0 token trong kho trình duyệt.

## Ngoài phạm vi

- Khung web, `apiClient`, theme sinh từ token, Storybook, component `ResponsiveTable` → F050.
- Màn hình nghiệp vụ của từng workspace → F052–F057, F079, F080, F122.
- Banner "Đang tắt kiểm duyệt" và dữ liệu công tắc kiểm duyệt → F122 (F051 chỉ cung cấp slot banner).
- Chỉ báo trạng thái hệ thống (health tổng hợp) → F131; chuông thông báo, tìm kiếm toàn portal (`Ctrl+K`): chưa có contract, chưa có feature sở hữu (ghi trong báo cáo).
- Realm Keycloak, client `eduworld-portal` confidential, direct grant flow, trang action-token `eduworld-ops` (`UPDATE_PASSWORD` từ email) → F004; backend `/auth/portal/*`, `/me/*`, `featuregate`, `principals:resolve` → F012.
- Dữ liệu bật/tắt và API System Admin → F015; màn "Bật/tắt menu & action" → F059; nội dung `specs/portal/menu-catalog.yaml` và script `check-portal-catalog` → contract chung (ADR-0090); gắn component màn hình vào khóa menu → từng workspace feature (F052–F059, F079, F108, F122…).

## Assumptions

- F050 cung cấp `apiClient` (tự gắn `Authorization`, `X-Tenant-Id`, `Idempotency-Key`, `If-Match`, ánh xạ Problem Details), kiểu sinh bởi `openapi-typescript`, theme `ops-ui`; đường dẫn theo plan.md, nếu F050 chốt khác thì theo F050.
- Kích thước breakpoint và thông điệp lỗi đăng nhập theo [ADMIN_PORTAL §1.3b, §1.5](../../docs/04-admin-portal/ADMIN_PORTAL.md) [Confirmed].
- Tên route workspace (`/school-lms`, `/content-studio`, `/game-ops`, `/live-ops`, `/platform-admin`) do feature này chốt [Confirmed]; path menu lấy từ catalog.
- API `/api/v1/*` được phục vụ dưới cùng origin với portal để cookie refresh/CSRF là first-party [Confirmed] (F004-AC-28).
- Claim và hành vi Keycloak theo spike F004 T039 [Confirmed].
