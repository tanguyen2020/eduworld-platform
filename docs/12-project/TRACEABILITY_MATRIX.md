# Ma trận truy vết yêu cầu (RTM)

> Trạng thái: Draft · Owner: Product Manager + QA Lead · **File sinh tự động** bởi `scripts/docs/build-rtm.mjs`: không sửa tay; sửa nguồn rồi chạy `node scripts/docs/build-rtm.mjs` (CI chạy `--check`).
> Nguồn: [PRODUCT_REQUIREMENTS](../00-overview/PRODUCT_REQUIREMENTS.md) (FR/US/AC) · [TEST_STRATEGY](TEST_STRATEGY.md) và bảng `TC-*` trong tài liệu domain · `specs/openapi/eduworld-api.v1.yaml` · [AGENTS.md §6](../../AGENTS.md#6-domain--tài-liệu--module) (module sở hữu)

## 1. Cách đọc

- **Tường minh**: tài liệu/operation/TC có nhắc đúng mã FR (hoặc FR nhắc đúng path/operationId). **Suy luận**: ghép theo domain của mã (tài liệu sở hữu, mẫu path, tiền tố TC) — cần rà khi review, không coi là bằng chứng.
- FR có phase `—` (mở rộng sau, ngoài Release 1.0) vẫn liệt kê nhưng không tính vào báo cáo mồ côi.
- AC/US gắn với FR theo domain (PRD chưa ghi FR cho từng AC). Muốn liên kết tường minh: ghi mã FR/AC trong cột kịch bản của bảng TC, hoặc trong `description` của operation OpenAPI.

## 2. Tổng hợp theo domain

| Domain | Module sở hữu | FR | FR có tài liệu tường minh | FR có operation tường minh | Operation suy luận | FR có TC tường minh | TC suy luận | US | AC | FR / AC mở rộng ở tài liệu sở hữu |
|---|---|---|---|---|---|---|---|---|---|---|
| `AUTH` Identity & Authentication | module identity | 36 | 36 | 10 | 16 | 1 | AUTH (85) | 3 | 36 | 0 / 56 |
| `ORG` Organization & Multi-tenancy | module identity | 18 | 18 | 0 | 47 | 0 | ORG (9), TEN (24) | 0 | 7 | 0 / 21 |
| `SIS` SIS Integration | module identity | 9 | 9 | 0 | 16 | 0 | SIS (7) | 2 | 0 | 0 / 0 |
| `CUR` Curriculum | module learning | 12 | 12 | 1 | 7 | 0 | LRN (18) | 0 | 0 | 0 / 0 |
| `CNT` Learning Content | module learning | 21 | 21 | 0 | 14 | 0 | CNT (10) | 0 | 8 | 0 / 0 |
| `QB` Question Bank | module learning | 14 | 14 | 0 | 17 | 0 | LRN (18) | 0 | 0 | 0 / 0 |
| `EXR` Activity & Attempt Engine | module learning | 16 | 16 | 2 | 35 | 0 | ACT (5), LRN (18), VID (5), SIM (5), MATH (3) | 3 | 8 | 0 / 25 |
| `EXM` Exam & Practice | module learning | 10 | 10 | 1 | 13 | 0 | EXM (6) | 1 | 2 | 0 / 8 |
| `PRG` Learning Progress & Mastery | module learning | 9 | 9 | 0 | 9 | 0 | LRN (18) | 1 | 2 | 0 / 0 |
| `ANL` Learning Events & Analytics | module learning · module game | 23 | 23 | 1 | 24 | 0 | ANL (11), DASH (10) | 0 | 0 | 0 / 21 |
| `GAME` Game World & Character | module game | 21 | 21 | 2 | 20 | 0 | WLD (6), GAM (14) | 2 | 0 | 0 / 9 |
| `QST` Quest | module game | 9 | 9 | 0 | 40 | 0 | GAM (14) | 2 | 0 | 0 / 15 |
| `MG` Mini-game Engine | module learning · game client | 4 | 4 | 0 | 1 | 0 | ACT (5) | 1 | 0 | 0 / 10 |
| `MP` Multiplayer & Presence | module competition · realtime | 6 | 6 | 0 | 2 | 0 | PTY (14), CMP (10) | 0 | 0 | 0 / 11 |
| `PTY` Party | module competition | 14 | 14 | 1 | 18 | 0 | PTY (14) | 4 | 13 | 0 / 0 |
| `MM` Matchmaking | module competition | 6 | 6 | 0 | 4 | 0 | MM (4) | 1 | 0 | 0 / 8 |
| `CMP` Competition / Battle | module competition | 6 | 6 | 0 | 7 | 0 | CMP (10) | 0 | 6 | 0 / 13 |
| `RNK` Ranking | module competition | 4 | 4 | 0 | 1 | 0 | CMP (10) | 0 | 0 | 0 / 6 |
| `LDB` Leaderboard | module competition | 5 | 5 | 0 | 4 | 0 | CMP (10) | 1 | 0 | 0 / 7 |
| `TRN` Tournament | module competition | 3 | 3 | 0 | 12 | 0 | CMP (10) | 1 | 0 | 0 / 8 |
| `SOC` Social | module chat | 4 | 4 | 0 | 37 | 0 | SOC (9) | 1 | 0 | 0 / 11 |
| `CHAT` Chat | module chat | 17 | 17 | 0 | 18 | 0 | CHAT (48) | 2 | 15 | 0 / 0 |
| `MOD` Moderation & Child Safety | module chat | 18 | 18 | 0 | 41 | 0 | CHAT (48) | 4 | 6 | 0 / 25 |
| `NTF` Notification | module chat | 3 | 3 | 2 | 3 | 0 | SOC (9) | 0 | 0 | 0 / 0 |
| `GAM` XP, level, tiến trình | module game | 11 | 11 | 0 | 5 | 0 | GAM (14) | 1 | 4 | 0 / 10 |
| `RWD` Phần thưởng | module game | 3 | 3 | 0 | 12 | 0 | GAM (14), ECO (8) | 0 | 0 | 0 / 0 |
| `CHL` Challenge & Boss | module game | 3 | 3 | 0 | 17 | 0 | CHL (8) | 0 | 0 | 0 / 8 |
| `ECO` Kinh tế, vật phẩm, sưu tầm | module game | 5 | 5 | 0 | 13 | 0 | ECO (8) | 0 | 0 | 0 / 8 |
| `EVT` Event & Season (live-ops) | module game | 8 | 8 | 1 | 19 | 0 | EVT (12) | 0 | 0 | 0 / 12 |
| `CMS` CMS | module learning · module game (game-admin) | 18 | 17 | 1 | 135 | 0 | CMS (4) | 2 | 5 | 0 / 0 |
| `LMS` LMS | module learning | 10 | 10 | 1 | 34 | 0 | LMS (3), DASH (10) | 11 | 0 | 0 / 17 |
| `AUD` Audit | module identity | 2 | 2 | 0 | 28 | 0 | AUD (5) | 0 | 0 | 0 / 4 |
| `OPS` Vận hành | src/backend/pkg/ (platform kit) · mọi module | 5 | 5 | 0 | 20 | 0 | SVC (17), PERF (26) | 0 | 0 | 0 / 0 |
| **Tổng** | | **353** | 352 | 23 | 606 op trong spec | 1 | 422 TC | 43 | 112 | 0 / 477 |

## 3. Ma trận FR

### 3.1. `AUTH` Identity & Authentication

Service: module identity · Tài liệu sở hữu: [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md)

AC: AC-AUTH-001, AC-AUTH-002, AC-AUTH-003, AC-AUTH-004, AC-AUTH-005, AC-AUTH-006, AC-AUTH-007, AC-AUTH-008, AC-AUTH-009, AC-AUTH-010, AC-AUTH-011, AC-AUTH-012, AC-AUTH-013, AC-AUTH-014, AC-AUTH-015, AC-AUTH-016, AC-AUTH-017, AC-AUTH-018, AC-AUTH-019, AC-AUTH-020, AC-AUTH-021, AC-AUTH-022, AC-AUTH-023, AC-AUTH-024, AC-AUTH-025, AC-AUTH-026, AC-AUTH-027, AC-AUTH-028, AC-AUTH-029, AC-AUTH-030, AC-AUTH-031, AC-AUTH-032, AC-AUTH-033, AC-AUTH-034, AC-AUTH-035, AC-AUTH-036 · US: US-AUTH-001, US-AUTH-002, US-AUTH-003

Mở rộng ở tài liệu sở hữu: 0 FR, 56 AC, 0 US — [ACCOUNT_PROVISIONING](../09-identity/ACCOUNT_PROVISIONING.md) · [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md)

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-AUTH-001 | Học sinh dùng tài khoản do nhà trường cấp (SIS) hoặc do platform tạo trong tenant chung; ngoài ra học sinh… | P1 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) · [ADR-0104-hoc-sinh-tu-dang-ky-trong-game-vao-tenant-chung](../adr/ADR-0104-hoc-sinh-tu-dang-ky-trong-game-vao-tenant-chung.md) | _suy luận:_ 16 op | _suy luận:_ TC-AUTH |
| FR-AUTH-002 | Godot đăng nhập bằng form username/password trong game, gửi qua HTTPS tới POST /api/v1/auth/game/login; mod… | P1 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | `gameLogin` | _suy luận:_ TC-AUTH |
| FR-AUTH-003 | EduWorld không lưu mật khẩu học sinh; xác thực ở Keycloak/IdP. Mật khẩu đi qua game và module identity (ADR… | P1 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 16 op | _suy luận:_ TC-AUTH |
| FR-AUTH-004 | Backend xác minh token (chữ ký JWKS, iss, aud=eduworld-api, exp) độc lập với APISIX (ADR-0008, ADR-0009) | P1 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 16 op | _suy luận:_ TC-AUTH |
| FR-AUTH-005 | Định danh ngoài bằng (issuer, subject); không dùng email/tên hiển thị làm khóa | P1 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 16 op | _suy luận:_ TC-AUTH |
| FR-AUTH-006 | Just-in-Time provisioning khi đăng nhập lần đầu qua IdP trường, chỉ khi khớp bản ghi SIS — ngoài Release 1.… | — | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 16 op | _suy luận:_ TC-AUTH |
| FR-AUTH-007 | POST /api/v1/auth/game/login (game) trả bootstrap: tenant_id của phiên, profile, school, grade, feature fla… | P1 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | `gameLogin` · `getMyPermissions` · `listMyTenants` · `portalLogin` | _suy luận:_ TC-AUTH |
| FR-AUTH-008 | Giữ phiên game bằng refresh token (ADR-0086, thay một phần ADR-0064): sau đăng nhập, game lưu refresh token… | P1 | [STUDENT_GAME_UX_CORE](../05-ux/STUDENT_GAME_UX_CORE.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | `refreshGameToken` | _suy luận:_ TC-AUTH |
| FR-AUTH-009 | Đăng xuất chủ động (nút "Đăng xuất" trong game, ADR-0086 mục 6): kết thúc game session (:end) → POST /api/v… | P1 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | `gameLogout` | _suy luận:_ TC-AUTH |
| FR-AUTH-010 | Tài khoản bị khóa/vô hiệu hóa: từ chối request REST, cắt kết nối realtime qua event user.access_revoked | P1 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 16 op | _suy luận:_ TC-AUTH |
| FR-AUTH-011 | Realtime ticket dùng 1 lần, TTL 60 giây để mở WebSocket /realtime/v1 (ADR-0008) | P1 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 16 op | _suy luận:_ TC-AUTH |
| FR-AUTH-012 | Cấu hình realm/client/issuer qua environment, không hardcode | P1 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 16 op | _suy luận:_ TC-AUTH |
| FR-AUTH-013 | Keycloak clients: eduworld-game-backend (confidential, chỉ Direct Access Grants, secret chỉ ở module identi… | P1 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 16 op | _suy luận:_ TC-AUTH |
| FR-AUTH-014 | Hỗ trợ nhiều issuer (multi-realm sau này) nhờ khóa (issuer, subject) | P1 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 16 op | _suy luận:_ TC-AUTH |
| FR-AUTH-015 | Device Authorization Grant cho máy dùng chung trong lớp — ngoài Release 1.0 (ADR-0063) | — | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) · [ADR-0095-bo-khai-niem-lop-hoc-sinh-theo-khoi](../adr/ADR-0095-bo-khai-niem-lop-hoc-sinh-theo-khoi.md) | _suy luận:_ 16 op | _suy luận:_ TC-AUTH |
| FR-AUTH-016 | Authorization theo Permission (trong code) → Role (DB, identity.roles) → UserRole (DB, identity.user_roles:… | P1 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 16 op | _suy luận:_ TC-AUTH |
| FR-AUTH-017 | Đăng nhập username + password, không chọn trường (ADR-0063, ADR-0068): form đăng nhập vẽ trong game gồm log… | P1 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | `selectGameLoginTenant` | _suy luận:_ TC-AUTH |
| FR-AUTH-018 | Backend xác định và kiểm tra chéo tenant khi tạo phiên và ở mọi request theo membership trong DB. Mọi reque… | P1 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | `createAuthSession` · `gameLogin` · `getClientConfig` · `listMyTenants` · `selectGameLoginTenant` | _suy luận:_ TC-AUTH |
| FR-AUTH-019 | Mỗi phiên có đúng một tenant; người thuộc nhiều tenant đổi tenant bằng header X-Tenant-Id mới được kiểm tra… | P1 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 16 op | _suy luận:_ TC-AUTH |
| FR-AUTH-020 | Trang đăng nhập Keycloak hiển thị tên/logo và IdP của tenant được gợi ý trước đăng nhập — ngoài Release 1.0… | — | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 16 op | _suy luận:_ TC-AUTH |
| FR-AUTH-021 | Tenant chung: tài khoản học sinh trong tenant chung common do platform (System Admin) tạo/import có chủ đíc… | P1 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 16 op | _suy luận:_ TC-AUTH |
| FR-AUTH-022 | Chỉ học sinh được đăng nhập game: đăng nhập qua /auth/game/login (client eduworld-game-backend) của nhân sự… | P1 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) · [EDU_FIT_REVIEW](EDU_FIT_REVIEW.md) | `gameLogin` | _suy luận:_ TC-AUTH |
| FR-AUTH-023 | Một thiết bị cho mỗi tài khoản học sinh: tối đa 1 game session active; đăng nhập thiết bị khác → hộp thoại… | P1 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 16 op | _suy luận:_ TC-AUTH |
| FR-AUTH-024 | Dọn học sinh không đăng nhập game: worker soft-delete học sinh có last_login_at (hoặc provisioned_at nếu ch… | P2 | [ACCOUNT_PROVISIONING](../09-identity/ACCOUNT_PROVISIONING.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) · [ADR-0085-don-hoc-sinh-giu-user-keycloak](../adr/ADR-0085-don-hoc-sinh-giu-user-keycloak.md) | _suy luận:_ 16 op | TC-AUTH-020 |
| FR-AUTH-025 | Không làm mã QR lớp + PIN 4 số, thẻ QR cá nhân, mật khẩu hình ảnh cho lớp 1–3 (ADR-0076 Rejected 2026-11-17… | — | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 16 op | _suy luận:_ TC-AUTH |
| FR-AUTH-026 | ~~Phiên game dài trên thiết bị~~ — gộp vào FR-AUTH-008 theo ADR-0086 (refresh token trong kho bảo mật của t… | — | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 16 op | _suy luận:_ TC-AUTH |
| FR-AUTH-027 | Hàng chờ đăng nhập: khi vượt ngưỡng toàn cục, POST /auth/game/login trả 429 LOGIN_QUEUE_BUSY + Retry-After,… | P4 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | `gameLogin` | _suy luận:_ TC-AUTH |
| FR-AUTH-028 | Bảo vệ đăng nhập game qua backend (ADR-0068): rate limit 10 lần/phút theo thiết bị và 10 lần/phút theo user… | P1 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 16 op | _suy luận:_ TC-AUTH |
| FR-AUTH-029 | Admin Portal có màn đăng nhập riêng do portal tự vẽ, qua backend (ADR-0091, thay luồng chuyển hướng OIDC củ… | P1 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) · [ADR-0091-dang-nhap-portal-bang-form-rieng-qua-backend](../adr/ADR-0091-dang-nhap-portal-bang-form-rieng-qua-backend.md) · [ADR-0094-bo-mfa-totp-cho-dang-nhap-admin-portal](../adr/ADR-0094-bo-mfa-totp-cho-dang-nhap-admin-portal.md) | `portalLogin` | _suy luận:_ TC-AUTH |
| FR-AUTH-030 | Portal không chọn tenant trước đăng nhập (ADR-0067): role platform → phạm vi platform; 1 tenant → vào thẳng… | P1 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | `listMyTenants` | _suy luận:_ TC-AUTH |
| FR-AUTH-031 | Username và email không trùng trong cùng tenant; trên SSO một người một tài khoản (username/email duy nhất… | P1 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 16 op | _suy luận:_ TC-AUTH |
| FR-AUTH-032 | Quản lý Role/UserRole trên portal (ADR-0066): Tenant Admin quản lý role tùy biến [Confirmed] và UserRole tr… | P3 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 16 op | _suy luận:_ TC-AUTH |
| FR-AUTH-033 | Học sinh tự đăng ký trong game (ADR-0104): màn "Tạo tài khoản" vẽ bằng Godot (mở từ S01) gồm tên đăng nhập,… | P5 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) · [ADR-0104-hoc-sinh-tu-dang-ky-trong-game-vao-tenant-chung](../adr/ADR-0104-hoc-sinh-tu-dang-ky-trong-game-vao-tenant-chung.md) | _suy luận:_ 16 op | _suy luận:_ TC-AUTH |
| FR-AUTH-034 | Xác nhận của phụ huynh khi tự đăng ký (ADR-0104): bước phụ huynh trên màn đăng ký — điều khoản ngắn tiếng V… | P5 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) · [ADR-0104-hoc-sinh-tu-dang-ky-trong-game-vao-tenant-chung](../adr/ADR-0104-hoc-sinh-tu-dang-ky-trong-game-vao-tenant-chung.md) | _suy luận:_ 16 op | _suy luận:_ TC-AUTH |
| FR-AUTH-035 | Khôi phục tài khoản tự đăng ký không cần email (ADR-0104): mã khôi phục dùng một lần hiện một lần ở cuối đă… | P5 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) · [ADR-0104-hoc-sinh-tu-dang-ky-trong-game-vao-tenant-chung](../adr/ADR-0104-hoc-sinh-tu-dang-ky-trong-game-vao-tenant-chung.md) | _suy luận:_ 16 op | _suy luận:_ TC-AUTH |
| FR-AUTH-036 | Chống lạm dụng và quản trị tài khoản tự đăng ký (ADR-0104): giới hạn số tài khoản mỗi thiết bị (bản cài, de… | P5 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) · [ADR-0104-hoc-sinh-tu-dang-ky-trong-game-vao-tenant-chung](../adr/ADR-0104-hoc-sinh-tu-dang-ky-trong-game-vao-tenant-chung.md) | _suy luận:_ 16 op | _suy luận:_ TC-AUTH |

### 3.2. `ORG` Organization & Multi-tenancy

Service: module identity · Tài liệu sở hữu: [MULTITENANCY](../06-architecture/MULTITENANCY.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md)

AC: AC-ORG-001, AC-ORG-002, AC-ORG-003, AC-ORG-004, AC-ORG-005, AC-ORG-006, AC-ORG-007 · US: —

Mở rộng ở tài liệu sở hữu: 0 FR, 21 AC, 0 US — [MULTITENANCY](../06-architecture/MULTITENANCY.md)

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-ORG-001 | Phân cấp Platform → Tenant → School; học sinh thuộc trường với một khối (grade_level 1–9), không có thực th… | P1 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 47 op | _suy luận:_ TC-ORG, TC-TEN |
| FR-ORG-002 | Quản lý AcademicYear của trường và khối (grade_level 1–9) của học sinh; không có Class, không có Enrollment… | P1 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 47 op | _suy luận:_ TC-ORG, TC-TEN |
| FR-ORG-003 | Phân biệt Keycloak User, EduWorld User, Student, Staff (nhân sự nhà trường/platform), School, External Iden… | P1 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 47 op | _suy luận:_ TC-ORG, TC-TEN |
| FR-ORG-004 | Removed (ADR-0072) — phân công giảng dạy lớp × môn không còn; phạm vi nhân sự là trường hoặc môn qua UserRo… | — | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) · [ADR-0095-bo-khai-niem-lop-hoc-sinh-theo-khoi](../adr/ADR-0095-bo-khai-niem-lop-hoc-sinh-theo-khoi.md) | _suy luận:_ 47 op | _suy luận:_ TC-ORG, TC-TEN |
| FR-ORG-005 | Chuyển trường và lên khối: cập nhật trường/khối của học sinh trong một transaction, audit, phát event; giữ… | P1 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 47 op | _suy luận:_ TC-ORG, TC-TEN |
| FR-ORG-006 | Mọi bảng nghiệp vụ có tenant_id; mọi truy vấn phải lọc theo tenant_id, enforcement ở app layer (TenantScope… | P1 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 47 op | _suy luận:_ TC-ORG, TC-TEN |
| FR-ORG-007 | School settings: chính sách chat (gồm preset quick_message_emote_only cho cả trường — ADR-0078), giờ truy c… | P1–P5 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 47 op | _suy luận:_ TC-ORG, TC-TEN |
| FR-ORG-008 | Tại một thời điểm người dùng chỉ làm việc trên một tenant. Chỉ System Admin thấy mọi tenant; mỗi màn hình c… | P1 (API) / P3 (CMS) | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 47 op | _suy luận:_ TC-ORG, TC-TEN |
| FR-ORG-009 | Mỗi Tenant là một Keycloak Organization (platform.tenants.keycloak_org_id, keycloak_org_alias, login_code);… | P1 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 47 op | _suy luận:_ TC-ORG, TC-TEN |
| FR-ORG-010 | Membership Keycloak Organization đồng bộ tự động: học sinh thuộc đúng một tenant; nhân sự thuộc các tenant… | P1 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 47 op | _suy luận:_ TC-ORG, TC-TEN |
| FR-ORG-011 | Chỉ soft-delete: ứng dụng không hard-delete; "xóa" đặt deleted_at/deleted_by; mọi truy vấn mặc định không t… | P1 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 47 op | _suy luận:_ TC-ORG, TC-TEN |
| FR-ORG-012 | System Admin tạo dữ liệu dùng chung (platform) và gán cho các tenant muốn sử dụng; tenant được gán chỉ đọc… | P1 (model) / P3 (CMS) | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 47 op | _suy luận:_ TC-ORG, TC-TEN |
| FR-ORG-013 | System Admin clone dữ liệu (nội dung, cấu hình) từ tenant này sang tenant khác; bản sao có ID mới và nguồn… | P3 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 47 op | _suy luận:_ TC-ORG, TC-TEN |
| FR-ORG-014 | Nghĩa vụ xóa dữ liệu theo luật được đáp ứng bằng ẩn danh hóa; Release 1.0 không xóa vật lý (ngoại lệ duy nh… | P5 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 47 op | _suy luận:_ TC-ORG, TC-TEN |
| FR-ORG-015 | Danh mục quốc gia (ISO 3166-1) và danh mục tỉnh/thành theo quốc gia, có hiệu lực và kế thừa đơn vị cũ → mới… | P1 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 47 op | _suy luận:_ TC-ORG, TC-TEN |
| FR-ORG-016 | Danh mục trường: mỗi trường thuộc một tenant và một tỉnh/thành, có mã ngành (nếu có), cấp học, khối áp dụng… | P1 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 47 op | _suy luận:_ TC-ORG, TC-TEN |
| FR-ORG-017 | Học sinh thuộc đúng một trường với một khối tại một thời điểm (hồ sơ học sinh hiệu lực, không enrollment);… | P1 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 47 op | _suy luận:_ TC-ORG, TC-TEN |
| FR-ORG-018 | Đồng bộ SIS khi đăng nhập: dữ liệu đủ & mới → không gọi SIS; thiếu trường/khối → tra cứu SIS chặn ≤ 3 s; dữ… | P1 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 47 op | _suy luận:_ TC-ORG, TC-TEN |

### 3.3. `SIS` SIS Integration

Service: module identity · Tài liệu sở hữu: [SIS_INTEGRATION](../09-identity/SIS_INTEGRATION.md) · [ACCOUNT_PROVISIONING](../09-identity/ACCOUNT_PROVISIONING.md)

AC: — · US: US-SIS-001, US-SIS-002

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-SIS-001 | EduWorld sở hữu DTO/API contract SIS (EduWorld SIS Contract v1); mỗi trường cung cấp API theo contract; khô… | P1 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 16 op | _suy luận:_ TC-SIS |
| FR-SIS-002 | Đồng bộ School, AcademicYear, Student (trường + khối), Account Status; không đồng bộ giáo viên, phân công g… | P1 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 16 op | _suy luận:_ TC-SIS |
| FR-SIS-003 | Hai cách nhận dữ liệu: contract_v1 (pull HTTP theo contract) và csv_import (cột CSV = tên trường DTO) cho t… | P1 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 16 op | _suy luận:_ TC-SIS |
| FR-SIS-009 | Bộ conformance test + mock server sinh từ contract để trường tự kiểm tra facade trước khi kết nối | P1 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 16 op | _suy luận:_ TC-SIS |
| FR-SIS-004 | Full sync định kỳ + delta sync + webhook (khi trường khai báo capability delta/webhook trong contract) | P1–P3 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 16 op | _suy luận:_ TC-SIS |
| FR-SIS-005 | Reconciliation: phát hiện bản ghi trùng/thiếu/xung đột, tạo ReconciliationIssue cho người duyệt | P1 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 16 op | _suy luận:_ TC-SIS |
| FR-SIS-006 | Sync error handling, retry có backoff, audit mọi lần chạy (SyncRun) | P1 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 16 op | _suy luận:_ TC-SIS |
| FR-SIS-007 | Godot không gọi SIS; SIS credential không lộ qua public gateway (/api/v1/sis chỉ gateway nội bộ) | P1 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 16 op | _suy luận:_ TC-SIS |
| FR-SIS-008 | Account deactivation/lock từ SIS lan truyền tới EduWorld trong ≤ 1 chu kỳ sync | P1 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 16 op | _suy luận:_ TC-SIS |

### 3.4. `CUR` Curriculum

Service: module learning · Tài liệu sở hữu: [CURRICULUM_MODEL](../02-learning/CURRICULUM_MODEL.md)

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-CUR-001 | Cấu trúc Curriculum → Grade → Subject → Topic → Learning Objective → Learning Unit → Lesson → Learning Acti… | P2 | [CURRICULUM_MODEL](../02-learning/CURRICULUM_MODEL.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 7 op | _suy luận:_ TC-LRN |
| FR-CUR-002 | Hỗ trợ nhiều chương trình và phiên bản chương trình; không hardcode một chương trình | P2 | [CURRICULUM_MODEL](../02-learning/CURRICULUM_MODEL.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 7 op | _suy luận:_ TC-LRN |
| FR-CUR-003 | Learning Objective có mã ổn định (vd. MATH.G3.NUM.MUL_TABLE) | P2 | [CURRICULUM_MODEL](../02-learning/CURRICULUM_MODEL.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 7 op | _suy luận:_ TC-LRN |
| FR-CUR-004 | Seed khung GDPT 2018 môn Toán lớp 1–5 cho pilot | P2 | [CONTENT_PRODUCTION_PLAN](../02-learning/CONTENT_PRODUCTION_PLAN.md) · [CURRICULUM_MODEL](../02-learning/CURRICULUM_MODEL.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 7 op | _suy luận:_ TC-LRN |
| FR-CUR-005 | CMS quản lý cấu trúc chương trình (CRUD + workflow) | P3 | [CURRICULUM_MODEL](../02-learning/CURRICULUM_MODEL.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 7 op | _suy luận:_ TC-LRN |
| FR-CUR-006 | API duyệt curriculum cho client: /api/v1/learning/curriculum | P2 | [CURRICULUM_MODEL](../02-learning/CURRICULUM_MODEL.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 7 op | _suy luận:_ TC-LRN |
| FR-CUR-007 | Đồ thị LO tiên quyết (prerequisite_ids[], DAG) dùng cho gợi ý ôn tập và bài chẩn đoán; phát hiện chu trình… | P2 | [CURRICULUM_MODEL](../02-learning/CURRICULUM_MODEL.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 7 op | _suy luận:_ TC-LRN |
| FR-CUR-008 | Trường chọn curriculum version mặc định cho mỗi năm học (school.settings.curriculum_version_id); nhiều vers… | P3 | [CURRICULUM_MODEL](../02-learning/CURRICULUM_MODEL.md) · [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | `getSchoolCurriculumSettings` | _suy luận:_ TC-LRN |
| FR-CUR-009 | LO bám GDPT 2018: thêm yccd_ref (mã yêu cầu cần đạt, ≤ 64 ký tự) và competency_ids[] (năng lực/phẩm chất);… | P2 | [ASSESSMENT_REGULATION_PROFILE](../02-learning/ASSESSMENT_REGULATION_PROFILE.md) · [CURRICULUM_MODEL](../02-learning/CURRICULUM_MODEL.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 7 op | _suy luận:_ TC-LRN |
| FR-CUR-010 | Import/export curriculum dạng YAML/CSV qua use case CMS (có validate, Idempotency-Key, audit; seed cũng đi… | P2 | [CURRICULUM_MODEL](../02-learning/CURRICULUM_MODEL.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 7 op | _suy luận:_ TC-LRN |
| FR-CUR-011 | Khi publish CurriculumVersion mới: bảng mapping LO giữa hai version (same, split, merge, replaces, removed)… | P3 | [CURRICULUM_MODEL](../02-learning/CURRICULUM_MODEL.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 7 op | _suy luận:_ TC-LRN |
| FR-CUR-012 | Mapping bộ sách giáo khoa → LO (bộ sách, tập, bài, n–n với LO) để lọc nội dung theo bài sách và chọn nội du… | P3 | [CURRICULUM_MODEL](../02-learning/CURRICULUM_MODEL.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 7 op | _suy luận:_ TC-LRN |

### 3.5. `CNT` Learning Content

Service: module learning · Tài liệu sở hữu: [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md)

AC: AC-CNT-001, AC-CNT-002, AC-CNT-003, AC-CNT-004, AC-CNT-005, AC-CNT-006, AC-CNT-007, AC-CNT-008 · US: —

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-CNT-001 | Loại nội dung: Lesson, Learning Material, Video, Image, Audio, Interactive Content, Question, Exercise, Que… | P2–P3 | [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 14 op | _suy luận:_ TC-CNT |
| FR-CNT-002 | Metadata bắt buộc: grade, subject, topic, LO, difficulty, version, status, author, reviewer, published_at | P2 | [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 14 op | _suy luận:_ TC-CNT |
| FR-CNT-003 | Trạng thái Draft → Review → Approved → Published → Archived (+ Rejected → Draft) | P2 | [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 14 op | _suy luận:_ TC-CNT |
| FR-CNT-004 | Published version bất biến; attempt tham chiếu content_version_id (ADR-0015) | P2 | [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 14 op | _suy luận:_ TC-CNT |
| FR-CNT-005 | Nội dung thuộc platform (tenant_id NULL, tenant thấy khi được gán) hoặc thuộc một tenant; chia sẻ giữa tena… | P2 | [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 14 op | _suy luận:_ TC-CNT |
| FR-CNT-006 | Media lưu object storage S3-compatible + CDN (không lưu file trong DB); pipeline quét virus, kiểm tra magic… | P2 | [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 14 op | _suy luận:_ TC-CNT |
| FR-CNT-007 | Import nội dung hàng loạt (JSON/CSV theo specs/schemas) cho seed pilot trước khi có CMS UI | P2 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 14 op | _suy luận:_ TC-CNT |
| FR-CNT-008 | Báo cáo "nội dung đang được dùng ở đâu" (quest, bài giao, đề, challenge tham chiếu) trước khi archive (CONT… | P3 | [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 14 op | _suy luận:_ TC-CNT |
| FR-CNT-009 | Draft/Publish cho mọi nội dung có version; release pointer; publish ngay hoặc hẹn giờ; rollback = dùng lại… | P2 | [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 14 op | _suy luận:_ TC-CNT |
| FR-CNT-010 | Xem nội dung từng version đã publish (chỉ đọc), so sánh hai version, preview trong game | P2–P3 | [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 14 op | _suy luận:_ TC-CNT |
| FR-CNT-011 | Version pinning: học sinh đang ở màn/quest/bài/attempt/trận giữ version cũ; lượt/màn mới nhận version mới n… | P2 | [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md) · [VIDEO_LEARNING](../02-learning/VIDEO_LEARNING.md) · [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 14 op | _suy luận:_ TC-CNT |
| FR-CNT-012 | Publish đẩy snapshot bất biến lên Valkey/CDN; content manifest theo tenant (ETag/304); câu hỏi đề và answer… | P2 | [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 14 op | _suy luận:_ TC-CNT |
| FR-CNT-013 | Activity Registry: mọi loại hoạt động học tập trong Release (quiz, interactive_exercise, minigame, video_le… | P2 | [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 14 op | _suy luận:_ TC-CNT |
| FR-CNT-014 | Video bài học: chapter, phụ đề WebVTT vi/en, resume, câu hỏi chèn tại mốc thời gian, nhánh tương tác, hoàn… | P4 | [VIDEO_LEARNING](../02-learning/VIDEO_LEARNING.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 14 op | _suy luận:_ TC-CNT |
| FR-CNT-015 | Bài nghe (audio_lesson): nghe hiểu có câu hỏi tự chấm (AUDIO_LANGUAGE_LEARNING). Bài nói (speaking) Removed… | P4 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) · [EDU_FIT_REVIEW](EDU_FIT_REVIEW.md) | _suy luận:_ 14 op | _suy luận:_ TC-CNT |
| FR-CNT-016 | Mô phỏng: template Godot đóng gói, CMS chỉ tham số hóa, server kiểm chứng xác định bằng checker Go (SIMULAT… | P4 | [SIMULATION_ENGINE](../02-learning/SIMULATION_ENGINE.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 14 op | _suy luận:_ TC-CNT |
| FR-CNT-017 | Công thức Toán/Lý/Hóa: soạn bằng MathLive, hiển thị MathJax 4 trên portal, SVG render sẵn trong game; game… | P2 | [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md) · [CONTENT_PRODUCTION_PLAN](../02-learning/CONTENT_PRODUCTION_PLAN.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 14 op | _suy luận:_ TC-CNT |
| FR-CNT-018 | Ngôn ngữ nội dung (ADR-0056): nội dung game bắt buộc đủ vi + en; nội dung học tập có content_language + bản… | P2 | [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 14 op | _suy luận:_ TC-CNT |
| FR-CNT-019 | Nội dung cho nhóm g1_3 (lớp 1–3): lồng tiếng tiếng Việt bắt buộc cho đề bài và hướng dẫn quest, là điều kiệ… | P2 | [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md) · [LEARNING_EXPERIENCE](../02-learning/LEARNING_EXPERIENCE.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 14 op | _suy luận:_ TC-CNT |
| FR-CNT-020 | Khối nội dung worked_example (lời giải mẫu từng bước) trong lesson/learning material, gợi ý xem lại sau khi… | P2 | [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md) · [QUESTION_BANK](../02-learning/QUESTION_BANK.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 14 op | _suy luận:_ TC-CNT |
| FR-CNT-021 | Tìm kiếm nội dung trong CMS theo metadata (khối, môn, chủ đề, LO, độ khó, trạng thái) và full-text tiếng Vi… | P3 | [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 14 op | _suy luận:_ TC-CNT |

### 3.6. `QB` Question Bank

Service: module learning · Tài liệu sở hữu: [QUESTION_BANK](../02-learning/QUESTION_BANK.md)

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-QB-001 | 11 loại: Single Choice, Multiple Choice, True/False, Fill in the Blank, Short Answer, Numeric Answer, Match… | P2 (6 loại đầu) – P3 (đủ) | [QUESTION_BANK](../02-learning/QUESTION_BANK.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 17 op | _suy luận:_ TC-LRN |
| FR-QB-002 | Thuộc tính: ID, stem, render data, options, answer key, explanation, difficulty, grade, subject, topic, LO,… | P2 | [QUESTION_BANK](../02-learning/QUESTION_BANK.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 17 op | _suy luận:_ TC-LRN |
| FR-QB-003 | Answer key và scoring rule không bao giờ gửi về client (ADR-0016) | P2 | [QUESTION_BANK](../02-learning/QUESTION_BANK.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 17 op | _suy luận:_ TC-LRN |
| FR-QB-004 | Random câu hỏi theo điều kiện (grade/topic/LO/difficulty/loại trừ câu đã làm gần đây) | P2 | [QUESTION_BANK](../02-learning/QUESTION_BANK.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 17 op | _suy luận:_ TC-LRN |
| FR-QB-005 | Tạo đề từ Question Bank theo Assessment Blueprint | P3 | [QUESTION_BANK](../02-learning/QUESTION_BANK.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 17 op | _suy luận:_ TC-LRN |
| FR-QB-006 | Chấm Short Answer bằng so khớp chuẩn hóa (bỏ dấu cách thừa, không phân biệt hoa thường, danh sách đáp án ch… | P2 | [QUESTION_BANK](../02-learning/QUESTION_BANK.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 17 op | _suy luận:_ TC-LRN |
| FR-QB-007 | Nội dung câu hỏi hỗ trợ tiếng Việt có dấu, công thức toán đơn giản, hình ảnh, audio (môn Tiếng Anh) | P2 | [QUESTION_BANK](../02-learning/QUESTION_BANK.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 17 op | _suy luận:_ TC-LRN |
| FR-QB-008 | Câu hỏi tham số hóa: render_payload.params có ràng buộc, server sinh giá trị theo seed của attempt, lưu tro… | P3 | [CONTENT_PRODUCTION_PLAN](../02-learning/CONTENT_PRODUCTION_PLAN.md) · [QUESTION_BANK](../02-learning/QUESTION_BANK.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 17 op | _suy luận:_ TC-LRN |
| FR-QB-009 | Interactive validator registry: mỗi widget câu interactive có validator Go thuần đăng ký trong registry, se… | P3 | [QUESTION_BANK](../02-learning/QUESTION_BANK.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 17 op | _suy luận:_ TC-LRN |
| FR-QB-010 | Item analysis: worker tính p_value, discrimination, distractor_rate, thời gian, tỉ lệ rapid_guess/gợi ý từ… | P3 | [QUESTION_BANK](../02-learning/QUESTION_BANK.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 17 op | _suy luận:_ TC-LRN |
| FR-QB-011 | Import câu hỏi hàng loạt từ Excel/CSV/QTI-lite có validate vào trạng thái Draft; import Word/MathType theo… | P3 | [CONTENT_PRODUCTION_PLAN](../02-learning/CONTENT_PRODUCTION_PLAN.md) · [QUESTION_BANK](../02-learning/QUESTION_BANK.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 17 op | _suy luận:_ TC-LRN |
| FR-QB-013 | Phản hồi theo phương án nhiễu: options[].feedback (i18n) lưu phía đáp án, chỉ trả trong kết quả chấm (Activ… | P2 | [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md) · [QUESTION_BANK](../02-learning/QUESTION_BANK.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 17 op | _suy luận:_ TC-LRN |
| FR-QB-014 | Hiệu chỉnh độ khó bán tự động: hệ thống đề xuất độ khó mới từ p_value khi đủ số lượt làm, người duyệt nội d… | P3 | [QUESTION_BANK](../02-learning/QUESTION_BANK.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 17 op | _suy luận:_ TC-LRN |
| FR-QB-015 | Chấm partial credit cấu hình được theo scoring_rule (all_or_nothing hoặc partial với partial_method theo dạ… | P2 | [QUESTION_BANK](../02-learning/QUESTION_BANK.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 17 op | _suy luận:_ TC-LRN |

### 3.7. `EXR` Activity & Attempt Engine

Service: module learning · Tài liệu sở hữu: [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md)

AC: AC-EXR-001, AC-EXR-002, AC-EXR-003, AC-EXR-004, AC-EXR-005, AC-EXR-006, AC-EXR-007, AC-EXR-008 · US: US-EXR-001, US-EXR-002, US-EXR-003

Mở rộng ở tài liệu sở hữu: 0 FR, 25 AC, 0 US — [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md) · [SIMULATION_ENGINE](../02-learning/SIMULATION_ENGINE.md) · [VIDEO_LEARNING](../02-learning/VIDEO_LEARNING.md)

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-EXR-001 | Chế độ: bài cá nhân, theo quest, theo chủ đề, nhà trường giao theo khối (ADR-0095) (school_admin/academic_m… | P2 (nhà trường giao: P3) | [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 35 op | _suy luận:_ TC-ACT, TC-LRN, TC-VID, TC-SIM, TC-MATH |
| FR-EXR-002 | Lưu Attempt, Answer, Correctness, Score, Duration, Submission Time, Content Version, LO | P2 | [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 35 op | _suy luận:_ TC-ACT, TC-LRN, TC-VID, TC-SIM, TC-MATH |
| FR-EXR-003 | Practice: phản hồi đúng/sai + giải thích ngay sau từng câu | P2 | [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md) · [LEARNING_EXPERIENCE](../02-learning/LEARNING_EXPERIENCE.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 35 op | _suy luận:_ TC-ACT, TC-LRN, TC-VID, TC-SIM, TC-MATH |
| FR-EXR-004 | Resume attempt chưa hoàn thành theo chính sách (TTL, số lần) | P2 | [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md) · [VIDEO_LEARNING](../02-learning/VIDEO_LEARNING.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 35 op | _suy luận:_ TC-ACT, TC-LRN, TC-VID, TC-SIM, TC-MATH |
| FR-EXR-005 | Submit idempotent (Idempotency-Key), một attempt chỉ submit một lần | P2 | [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 35 op | _suy luận:_ TC-ACT, TC-LRN, TC-VID, TC-SIM, TC-MATH |
| FR-EXR-006 | Thời gian tính theo server timestamp; client elapsed chỉ tham khảo | P2 | [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 35 op | _suy luận:_ TC-ACT, TC-LRN, TC-VID, TC-SIM, TC-MATH |
| FR-EXR-007 | Một họ API attempt /api/v1/learning/activity-attempts cho mọi loại hoạt động (start, responses, progress, s… | P2 | [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | `listMyActivityAttempts` · `startAdHocAttempt` | _suy luận:_ TC-ACT, TC-LRN, TC-VID, TC-SIM, TC-MATH |
| FR-EXR-008 | Evaluation tách khỏi Attempt: chiến lược chấm chỉ là tự động; đúng một evaluation có hiệu lực; re-evaluate… | P3 | [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 35 op | _suy luận:_ TC-ACT, TC-LRN, TC-VID, TC-SIM, TC-MATH |
| FR-EXR-009 | Removed (ADR-0072) — tự luận chấm rubric, hàng đợi chấm, trả bài, chấm lại thủ công ngoài Release; ngoài ve… | — | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 35 op | _suy luận:_ TC-ACT, TC-LRN, TC-VID, TC-SIM, TC-MATH |
| FR-EXR-010 | Removed (ADR-0072) — activity project (dự án nhiều bước chấm tay) ngoài Release | — | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 35 op | _suy luận:_ TC-ACT, TC-LRN, TC-VID, TC-SIM, TC-MATH |
| FR-EXR-011 | Removed (ADR-0072) — AI hỗ trợ chấm bài viết/nói phụ thuộc người duyệt chấm; mở lại cần ADR mới (ADR-0053 P… | — | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 35 op | _suy luận:_ TC-ACT, TC-LRN, TC-VID, TC-SIM, TC-MATH |
| FR-EXR-012 | Gợi ý nhiều cấp: hints[] trong câu hỏi; POST /api/v1/learning/activity-attempts/{attempt_id}/items/{positio… | P2 | [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md) · [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md) · [QUESTION_BANK](../02-learning/QUESTION_BANK.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | `revealNextHint` | _suy luận:_ TC-ACT, TC-LRN, TC-VID, TC-SIM, TC-MATH |
| FR-EXR-013 | retry_wrong ưu tiên câu "anh em" (cùng LO, cùng loại/độ khó, khác question_id) thay vì lặp lại đúng câu đã… | P2 | [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md) · [LEARNING_EXPERIENCE](../02-learning/LEARNING_EXPERIENCE.md) · [QUESTION_BANK](../02-learning/QUESTION_BANK.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 35 op | _suy luận:_ TC-ACT, TC-LRN, TC-VID, TC-SIM, TC-MATH |
| FR-EXR-014 | Chế độ luyện trộn chủ đề (interleaving): bộ câu luyện tập trộn nhiều LO đã học theo tỷ lệ cấu hình (LEARNIN… | P3 | [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md) · [LEARNING_EXPERIENCE](../02-learning/LEARNING_EXPERIENCE.md) · [QUESTION_BANK](../02-learning/QUESTION_BANK.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 35 op | _suy luận:_ TC-ACT, TC-LRN, TC-VID, TC-SIM, TC-MATH |
| FR-EXR-015 | Content manifest mang min_client_version theo activity_type; client cũ không nhận activity có loại chưa hỗ… | P2 | [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 35 op | _suy luận:_ TC-ACT, TC-LRN, TC-VID, TC-SIM, TC-MATH |
| FR-EXR-016 | Attempt quá deadline_at chuyển expired; worker tự nộp (auto-submit) nếu đã có câu trả lời; phản hồi đến sau… | P2 | [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 35 op | _suy luận:_ TC-ACT, TC-LRN, TC-VID, TC-SIM, TC-MATH |

### 3.8. `EXM` Exam & Practice

Service: module learning · Tài liệu sở hữu: [EXAM_ENGINE](../02-learning/EXAM_ENGINE.md)

AC: AC-EXM-001, AC-EXM-002 · US: US-EXM-001

Mở rộng ở tài liệu sở hữu: 0 FR, 8 AC, 0 US — [EXAM_ENGINE](../02-learning/EXAM_ENGINE.md)

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-EXM-001 | Hình thức: Practice Test, Mock Exam, Chapter Test, Semester Test, Subject Test, Custom Assessment, Timed Exam | P3 | [EXAM_ENGINE](../02-learning/EXAM_ENGINE.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 13 op | _suy luận:_ TC-EXM |
| FR-EXM-002 | Tạo đề, random đề, random câu, xáo trộn đáp án, cấu hình thời gian & thang điểm | P3 | [EXAM_ENGINE](../02-learning/EXAM_ENGINE.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 13 op | _suy luận:_ TC-EXM |
| FR-EXM-003 | Nộp bài, chấm điểm, hiển thị kết quả theo result_visibility, phân tích câu sai, đề xuất nội dung ôn tập | P3 | [EXAM_ENGINE](../02-learning/EXAM_ENGINE.md) · [LEARNING_EXPERIENCE](../02-learning/LEARNING_EXPERIENCE.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 13 op | _suy luận:_ TC-EXM |
| FR-EXM-004 | Phân biệt luyện tập và kiểm tra chính thức (is_official) | P3 | [EXAM_ENGINE](../02-learning/EXAM_ENGINE.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 13 op | _suy luận:_ TC-EXM |
| FR-EXM-005 | Chống nộp trùng, chống sửa điểm từ client, tự nộp khi hết deadline_at | P3 | [EXAM_ENGINE](../02-learning/EXAM_ENGINE.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 13 op | _suy luận:_ TC-EXM |
| FR-EXM-006 | Kết quả thi lưu bền vững trong PostgreSQL, không phụ thuộc Valkey | P3 | [EXAM_ENGINE](../02-learning/EXAM_ENGINE.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 13 op | _suy luận:_ TC-EXM |
| FR-EXM-007 | Thi chính thức có giá trị pháp lý/proctoring | Ngoài Release | [EXAM_ENGINE](../02-learning/EXAM_ENGINE.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 13 op | _suy luận:_ TC-EXM |
| FR-EXM-008 | Chấm lại đề (regrade) là re-evaluate do hệ thống khi câu hỏi lỗi (RegradeRequest, do content_admin/academic… | P3 | [EXAM_ENGINE](../02-learning/EXAM_ENGINE.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 13 op | _suy luận:_ TC-EXM |
| FR-EXM-009 | Gia hạn thời gian làm bài cho học sinh cụ thể (accommodations[] của ExamSession, vd. +25%) do school_admin/… | P3 | [EXAM_ENGINE](../02-learning/EXAM_ENGINE.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | `putExamAccommodations` | _suy luận:_ TC-EXM |
| FR-EXM-010 | Đề is_official: cân bằng độ khó giữa các biến thể đề theo p_value, báo cáo độ tin cậy (KR-20/alpha) sau kỳ… | P3 | [EXAM_ENGINE](../02-learning/EXAM_ENGINE.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 13 op | _suy luận:_ TC-EXM |

### 3.9. `PRG` Learning Progress & Mastery

Service: module learning · Tài liệu sở hữu: [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md)

AC: AC-PRG-001, AC-PRG-002 · US: US-PRG-001

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-PRG-001 | Theo dõi tiến độ, số bài hoàn thành, tỷ lệ chính xác, thời gian học, số lần làm, LO mastery, kỹ năng yếu, t… | P2–P3 | [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 9 op | _suy luận:_ TC-LRN |
| FR-PRG-002 | Mastery theo (student, LO) tính từ nhiều bằng chứng, không đồng nhất với điểm một lần làm bài | P2 | [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 9 op | _suy luận:_ TC-LRN |
| FR-PRG-003 | Mô hình mastery cấu hình được, mặc định Weighted Evidence (ADR-0017), lưu model_version | P2 | [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 9 op | _suy luận:_ TC-LRN |
| FR-PRG-004 | Gợi ý ôn tập dựa trên LO yếu (/api/v1/learning/recommendations) | P3 | [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md) · [LEARNING_EXPERIENCE](../02-learning/LEARNING_EXPERIENCE.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 9 op | _suy luận:_ TC-LRN |
| FR-PRG-005 | Học sinh xem tiến độ dạng thân thiện theo độ tuổi (sao/huy hiệu cho lớp 1–5, biểu đồ cho 6–9) | P3 | [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md) · [LEARNING_EXPERIENCE](../02-learning/LEARNING_EXPERIENCE.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 9 op | _suy luận:_ TC-LRN |
| FR-PRG-006 | Lọc đoán bừa: response có response_ms dưới ngưỡng tối thiểu của item (mặc định theo loại câu) gắn cờ rapid_… | P2 | [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md) · [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 9 op | _suy luận:_ TC-LRN |
| FR-PRG-007 | Bài chẩn đoán (AttemptMode = diagnostic) ngắn đầu năm học/đầu chủ đề để khởi tạo mastery và gợi ý điểm bắt… | P3 | [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md) · [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 9 op | _suy luận:_ TC-LRN |
| FR-PRG-008 | Gợi ý ôn tập tự động có giải thích (LO yếu, LO tiên quyết) là con đường học chính thay cho chỉ định của giá… | P3 | [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md) · [LEARNING_EXPERIENCE](../02-learning/LEARNING_EXPERIENCE.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 9 op | _suy luận:_ TC-LRN |
| FR-PRG-009 | Recompute mastery: job tính lại mastery từ learning_events khi đổi model_version (chạy shadow, so sánh rồi… | P3 | [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 9 op | _suy luận:_ TC-LRN |

### 3.10. `ANL` Learning Events & Analytics

Service: module learning · module game · Tài liệu sở hữu: [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md) · [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md) · [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md)

Mở rộng ở tài liệu sở hữu: 0 FR, 21 AC, 0 US — [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) · [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md)

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-ANL-001 | Ghi learning events (activity/question/exercise/exam/lesson/quest/learning_objective.*) với event_id, stude… | P2 | [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 24 op | _suy luận:_ TC-ANL, TC-DASH |
| FR-ANL-002 | Lưu bền vững trong learning.learning_events (LRS nội bộ), tương thích tư duy xAPI | P2 | [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 24 op | _suy luận:_ TC-ANL, TC-DASH |
| FR-ANL-003 | Analytics theo học sinh, khối, trường, môn, chủ đề, LO (bỏ chiều giáo viên theo ADR-0072, bỏ chiều lớp theo… | P3 | [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 24 op | _suy luận:_ TC-ANL, TC-DASH |
| FR-ANL-004 | Không dùng dữ liệu hành vi ngoài mục đích đã công bố | P1+ | [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md) · [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 24 op | _suy luận:_ TC-ANL, TC-DASH |
| FR-ANL-005 | Aggregation chạy trong worker, dashboard đọc từ read model | P3 | [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 24 op | _suy luận:_ TC-ANL, TC-DASH |
| FR-ANL-006 | Game Analytics tách khỏi Learning Analytics và Monitoring: chỉ số engagement, retention, funnel, tiến trình… | P4 | [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 24 op | _suy luận:_ TC-ANL, TC-DASH |
| FR-ANL-007 | Nguồn: domain event NATS + telemetry client tối thiểu không PII; lưu schema analytics của module game (ADR-… | P4 | [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 24 op | _suy luận:_ TC-ANL, TC-DASH |
| FR-ANL-008 | Định danh giả danh player_key; không lưu tên, nickname, student_id, nội dung chat trong analytics | P4 | [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 24 op | _suy luận:_ TC-ANL, TC-DASH |
| FR-ANL-009 | Dashboard Game Analytics trong workspace Game Live Operations (platform); báo cáo tổng hợp cho Tenant/Schoo… | P4 | [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 24 op | _suy luận:_ TC-ANL, TC-DASH |
| FR-ANL-010 | Không dùng thời gian chơi làm thước đo thành công; có chỉ số chơi lành mạnh (vượt mốc nhắc nghỉ, chơi khuya) | P4 | [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 24 op | _suy luận:_ TC-ANL, TC-DASH |
| FR-ANL-011 | Tenant tắt được telemetry sản phẩm; telemetry ổn định (hiệu năng, crash) vẫn thu (cơ chế kỹ thuật; cơ sở ph… | P4 | [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 24 op | _suy luận:_ TC-ANL, TC-DASH |
| FR-ANL-012 | Có trang dashboard cho trường → khối → học sinh (School Learning Dashboard), tenant, platform và Game Analy… | P3–P4 | [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) · [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 24 op | _suy luận:_ TC-ANL, TC-DASH |
| FR-ANL-013 | Dashboard chỉ đọc bảng tổng hợp dựng gia tăng; khoảng thời gian tối đa 2 năm, độ hạt tự chọn theo khoảng; h… | P3 | [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) · [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 24 op | _suy luận:_ TC-ANL, TC-DASH |
| FR-ANL-014 | Xuất báo cáo dashboard chạy bất đồng bộ, link tải có hạn, có audit | P3 | [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) · [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 24 op | _suy luận:_ TC-ANL, TC-DASH |
| FR-ANL-015 | Báo cáo tham khảo theo mức đánh giá TT27/2020 (tiểu học) và TT22/2021 (THCS) từ mastery/kết quả; ghi rõ "ch… | P3 | [ASSESSMENT_REGULATION_PROFILE](../02-learning/ASSESSMENT_REGULATION_PROFILE.md) · [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md) · [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 24 op | _suy luận:_ TC-ANL, TC-DASH |
| FR-ANL-016 | Báo cáo learning analytics cấp tổ chức/tenant/platform chỉ dùng dữ liệu gộp, ẩn ô có < 5 học sinh (k-anonym… | P3 | [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) · [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md) · [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) · +1 | `getGradeDashboard` · `getSchoolAnalyticsSeries` · `getSchoolDashboard` · `getSchoolEventProgress` | _suy luận:_ TC-ANL, TC-DASH |
| FR-ANL-017 | Ngân hàng câu hỏi/đề đo hiệu quả (efficacy_instrument) tách khỏi mọi luyện tập, không xuất hiện trong pract… | P3 | [LEARNING_EFFICACY_PLAN](../02-learning/LEARNING_EFFICACY_PLAN.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 24 op | _suy luận:_ TC-ANL, TC-DASH |
| FR-ANL-018 | Bài kiểm tra độc lập trước/sau cho cụm (trường, khối) tham gia và cụm so sánh cùng khối (ADR-0095); không t… | P3 | [LEARNING_EFFICACY_PLAN](../02-learning/LEARNING_EFFICACY_PLAN.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 24 op | _suy luận:_ TC-ANL, TC-DASH |
| FR-ANL-019 | Ghi thỏa thuận tham gia nghiên cứu của tenant (pilot), có thu hồi và audit (LEARNING_EFFICACY_PLAN, ADR-0080) | P3 | [LEARNING_EFFICACY_PLAN](../02-learning/LEARNING_EFFICACY_PLAN.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 24 op | _suy luận:_ TC-ANL, TC-DASH |
| FR-ANL-020 | Thử nghiệm A/B cấu hình mastery và luật gợi ý theo nhánh gán ổn định, ghi experiment_arms vào learning even… | P3 | [LEARNING_EFFICACY_PLAN](../02-learning/LEARNING_EFFICACY_PLAN.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 24 op | _suy luận:_ TC-ANL, TC-DASH |
| FR-ANL-021 | Báo cáo hiệu quả: effect size, tỉ lệ học sinh tiến bộ, tỉ lệ rời nghiên cứu, độ tin cậy dự báo của mastery… | P3 | [LEARNING_EFFICACY_PLAN](../02-learning/LEARNING_EFFICACY_PLAN.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 24 op | _suy luận:_ TC-ANL, TC-DASH |
| FR-ANL-022 | Xuất dữ liệu nghiên cứu định danh giả, chỉ cho nhân sự platform có quyền, áp k-anonymity như FR-ANL-016 (LE… | P3 | [LEARNING_EFFICACY_PLAN](../02-learning/LEARNING_EFFICACY_PLAN.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 24 op | _suy luận:_ TC-ANL, TC-DASH |
| FR-ANL-023 | Nhà trường rút cụm (trường, khối) hoặc học sinh khỏi phần phân tích nghiên cứu (ADR-0095); dữ liệu đã rút b… | P3 | [LEARNING_EFFICACY_PLAN](../02-learning/LEARNING_EFFICACY_PLAN.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 24 op | _suy luận:_ TC-ANL, TC-DASH |

### 3.11. `GAME` Game World & Character

Service: module game · Tài liệu sở hữu: [WORLD_DESIGN](../01-game-design/WORLD_DESIGN.md) · [CHARACTER_SYSTEM](../01-game-design/CHARACTER_SYSTEM.md)

AC: — · US: US-GAME-001, US-GAME-002

Mở rộng ở tài liệu sở hữu: 0 FR, 9 AC, 0 US — [GAME_DESIGN](../01-game-design/GAME_DESIGN.md) · [WORLD_DESIGN](../01-game-design/WORLD_DESIGN.md)

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-GAME-001 | Thế giới 3D stylized/cartoon/low-poly, thân thiện, không bạo lực | P2 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 20 op | _suy luận:_ TC-WLD, TC-GAM |
| FR-GAME-002 | Thế giới gồm 8 vùng đất có cốt truyện, bao trọn 7 khu chức năng bắt buộc (Main Hub, Learning Zone theo môn,… | P2 (Hub, vùng môn học, Practice, Quest) – P3 (Exam) – P4 (Arena) – P5 (Social) | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 20 op | _suy luận:_ TC-WLD, TC-GAM |
| FR-GAME-003 | Character controller, camera controller, animation state machine, NPC interaction | P2 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 20 op | _suy luận:_ TC-WLD, TC-GAM |
| FR-GAME-004 | Avatar customization: skin, outfit, hair, accessories, emotes; character level; inventory | P2 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 20 op | _suy luận:_ TC-WLD, TC-GAM |
| FR-GAME-015 | Luồng tạo avatar lần đầu (preset, dáng người, màu da/tóc, trang phục mặc định, nickname), equipment slot, a… | P2 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 20 op | _suy luận:_ TC-WLD, TC-GAM |
| FR-GAME-016 | Mạch truyện "Thắp lại Tia Sáng" gồm 7 chương gắn vùng; chương môn học mở song song, không khóa học tập theo… | P2–P4 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 20 op | _suy luận:_ TC-WLD, TC-GAM |
| FR-GAME-017 | World map, cổng di chuyển, khám phá (discovery point, collectible, % hoàn thành bản đồ), mở khóa vùng theo… | P2–P3 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 20 op | _suy luận:_ TC-WLD, TC-GAM |
| FR-GAME-018 | Học tập lồng trong ngữ cảnh game: mỗi khối học tập ngắn rồi xen tương tác thế giới; không biến game thành c… | P2 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 20 op | _suy luận:_ TC-WLD, TC-GAM |
| FR-GAME-019 | Nickname tự gõ qua kiểm duyệt server (ADR-0105): cùng bước chuẩn hóa và danh mục từ nhạy cảm của chat (khôn… | P5 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) · [ADR-0105-nickname-tu-go-qua-kiem-duyet](../adr/ADR-0105-nickname-tu-go-qua-kiem-duyet.md) | _suy luận:_ 20 op | _suy luận:_ TC-WLD, TC-GAM |
| FR-GAME-020 | Âm thanh game ở mọi phần (ADR-0106): nhạc nền theo vùng/màn (Main Hub, từng vùng đất, Arena/thi đấu, boss,… | P3 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 20 op | _suy luận:_ TC-WLD, TC-GAM |
| FR-GAME-021 | Âm thanh phù hợp giáo dục và trợ năng (ADR-0106): nhịp chậm, nhạc không lời; không âm thanh gắt/đáng sợ, kh… | P3 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 20 op | _suy luận:_ TC-WLD, TC-GAM |
| FR-GAME-005 | Nickname do học sinh tự gõ (nickname_min_length–32 ký tự: chữ cái tiếng Việt có dấu, chữ số, khoảng trắng)… | P2 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) · [ADR-0105-nickname-tu-go-qua-kiem-duyet](../adr/ADR-0105-nickname-tu-go-qua-kiem-duyet.md) | _suy luận:_ 20 op | _suy luận:_ TC-WLD, TC-GAM |
| FR-GAME-006 | Game client data-driven: quest, mini-game, NPC dialogue, reward, cấu hình khu vực tải từ backend; cập nhật… | P2 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 20 op | _suy luận:_ TC-WLD, TC-GAM |
| FR-GAME-007 | Game session server-side (/api/v1/game/sessions), ghi platform, client version, end reason | P1 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | `createGameSession` | _suy luận:_ TC-WLD, TC-GAM |
| FR-GAME-008 | Kiểm tra phiên bản client tối thiểu; buộc cập nhật khi không tương thích API | P1 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 20 op | _suy luận:_ TC-WLD, TC-GAM |
| FR-GAME-009 | Chạy được trên máy tầm trung: renderer Mobile/Compatibility, preset Thấp/Trung bình/Cao tự phát hiện, độ ph… | P2–P6 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 20 op | _suy luận:_ TC-WLD, TC-GAM |
| FR-GAME-010 | UI theo nhóm tuổi: lớp 1–5 trực quan/ít thao tác/chữ to + hỗ trợ đọc âm thanh; lớp 6–9 nhiều thông tin hơn | P2 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 20 op | _suy luận:_ TC-WLD, TC-GAM |
| FR-GAME-011 | Telemetry client (crash, FPS, load time) không chứa PII, rate limit (/api/v1/game/telemetry) | P2 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | `postTelemetry` | _suy luận:_ TC-WLD, TC-GAM |
| FR-GAME-012 | Nhắc nghỉ sau thời gian chơi liên tục (mặc định 45 phút), giờ truy cập do trường cấu hình; giờ giới nghiêm… | P2 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 20 op | _suy luận:_ TC-WLD, TC-GAM |
| FR-GAME-013 | Lưu thời gian đăng nhập game: mỗi lần vào game ghi game.login_events (thời điểm vào/ra theo giờ server, nền… | P1 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 20 op | _suy luận:_ TC-WLD, TC-GAM |
| FR-GAME-014 | Game có nút "Đăng xuất" (đóng game session, thu hồi phiên, xóa token khỏi kho bảo mật — ADR-0086); đóng gam… | P1 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 20 op | _suy luận:_ TC-WLD, TC-GAM |

### 3.12. `QST` Quest

Service: module game · Tài liệu sở hữu: [QUEST_SYSTEM](../01-game-design/QUEST_SYSTEM.md)

AC: — · US: US-QST-001, US-QST-002

Mở rộng ở tài liệu sở hữu: 0 FR, 15 AC, 0 US — [QUEST_SYSTEM](../01-game-design/QUEST_SYSTEM.md)

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-QST-001 | Quest có: ID, tên, mô tả, loại, khối, môn, chủ đề, LO, điều kiện bắt đầu, các bước, điều kiện hoàn thành, p… | P2 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 40 op | _suy luận:_ TC-GAM |
| FR-QST-002 | 12 loại quest: Main, Side, Daily, Weekly, Learning, Exploration, Social, Party, Challenge, Seasonal, Specia… | P2 (Party/Challenge/Boss: P3–P4) | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 40 op | _suy luận:_ TC-GAM |
| FR-QST-008 | Quest hỗ trợ narrative, NPC, level requirement, learning requirement, repeatability, cooldown, failure cond… | P2 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 40 op | _suy luận:_ TC-GAM |
| FR-QST-003 | Quest liên kết nội dung học tập; bước activity tạo Attempt trong Activity & Attempt Engine | P2 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 40 op | _suy luận:_ TC-GAM |
| FR-QST-004 | Bước: talk_npc, go_to, activity, collect, minigame, party_activity | P2 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 40 op | _suy luận:_ TC-GAM |
| FR-QST-005 | Tiến độ quest lưu server, hoàn thành bước qua domain event (ADR-0019) | P2 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 40 op | _suy luận:_ TC-GAM |
| FR-QST-006 | Quest lọc theo grade_level của học sinh; không giao nội dung sai khối | P2 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 40 op | _suy luận:_ TC-GAM |
| FR-QST-007 | Shared Quest Progress theo cấu hình shared_progress_policy; kết quả học tập vẫn riêng từng học sinh | P4 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 40 op | _suy luận:_ TC-GAM |
| FR-QST-009 | Bước quest kiểu interdependent có role: mỗi thành viên party giữ một phần dữ kiện, cả nhóm phải phối hợp mớ… | P4 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 40 op | _suy luận:_ TC-GAM |

### 3.13. `MG` Mini-game Engine

Service: module learning · game client · Tài liệu sở hữu: [MINIGAME_ENGINE](../08-game-client/MINIGAME_ENGINE.md)

AC: — · US: US-MG-001

Mở rộng ở tài liệu sở hữu: 0 FR, 10 AC, 0 US — [LEARNING_MECHANICS](../01-game-design/LEARNING_MECHANICS.md) · [MINIGAME_ENGINE](../08-game-client/MINIGAME_ENGINE.md)

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-MG-001 | Loại: Multiple Choice, True/False, Fill in the Blank, Drag and Drop, Matching, Ordering, Calculation, Puzzl… | P2 (6 loại) – sau | [MINIGAME_ENGINE](../08-game-client/MINIGAME_ENGINE.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 1 op | _suy luận:_ TC-ACT |
| FR-MG-002 | Mini-game không có ngân hàng câu hỏi riêng; lấy từ Learning Engine qua QuestionSet/selection rule | P2 | [MINIGAME_ENGINE](../08-game-client/MINIGAME_ENGINE.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 1 op | _suy luận:_ TC-ACT |
| FR-MG-003 | Cấu hình: độ khó, thời gian, số câu, điểm, cách tính kết quả, phản hồi đúng/sai, cơ chế hoàn thành, reward… | P2 | [MINIGAME_ENGINE](../08-game-client/MINIGAME_ENGINE.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 1 op | _suy luận:_ TC-ACT |
| FR-MG-004 | Mỗi mini-game là scene Godot tái sử dụng, nhận MiniGameConfig + render payload | P2 | [MINIGAME_ENGINE](../08-game-client/MINIGAME_ENGINE.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 1 op | _suy luận:_ TC-ACT |

### 3.14. `MP` Multiplayer & Presence

Service: module competition · realtime · Tài liệu sở hữu: [MULTIPLAYER_GAMEPLAY](../03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md) · [REALTIME_ARCHITECTURE](../06-architecture/REALTIME_ARCHITECTURE.md)

Mở rộng ở tài liệu sở hữu: 0 FR, 11 AC, 0 US — [MULTIPLAYER_GAMEPLAY](../03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md)

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-MP-001 | Hỗ trợ Multiplayer World, Room, Lobby, Team, Match, Session | P4 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 2 op | _suy luận:_ TC-PTY, TC-CMP |
| FR-MP-002 | Hub instance tối đa 40 người, cấp theo school/grade band; bạn bè/party ưu tiên cùng instance (ADR-0012) | P4 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 2 op | _suy luận:_ TC-PTY, TC-CMP |
| FR-MP-003 | Presence/online status (Valkey), chỉ hiển thị cho bạn bè/party/club theo policy (không theo lớp — ADR-0095) | P4 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 2 op | _suy luận:_ TC-PTY, TC-CMP |
| FR-MP-004 | Đồng bộ vị trí qua WebSocket ≤10 Hz, interpolation, server kiểm tra tốc độ/biên (ADR-0011) | P4 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 2 op | _suy luận:_ TC-PTY, TC-CMP |
| FR-MP-005 | Heartbeat, timeout, reconnect, session recovery, disconnect handling | P4 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 2 op | _suy luận:_ TC-PTY, TC-CMP |
| FR-MP-006 | Không gửi chuyển động 3D qua REST; Valkey không làm game simulation server | P4 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 2 op | _suy luận:_ TC-PTY, TC-CMP |

### 3.15. `PTY` Party

Service: module competition · Tài liệu sở hữu: [PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md)

AC: AC-PTY-001, AC-PTY-002, AC-PTY-003, AC-PTY-004, AC-PTY-005, AC-PTY-006, AC-PTY-007, AC-PTY-008, AC-PTY-009, AC-PTY-010, AC-PTY-011, AC-PTY-012, AC-PTY-013 · US: US-PTY-001, US-PTY-002, US-PTY-003, US-PTY-004

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-PTY-001 | Create, Join, Leave, Invite Friend, Accept, Reject, Kick, Transfer Leader, Disband, Settings, Member Status… | P4 (gồm Party Chat văn bản tự do — channel `party` always-on, FR-CHAT-009) | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) · [SPEC_RECONCILIATION_BACKLOG](SPEC_RECONCILIATION_BACKLOG.md) | _suy luận:_ 18 op | _suy luận:_ TC-PTY |
| FR-PTY-002 | Thuộc tính: Party ID, name, leader, members, max members, type, visibility, invitation policy, current acti… | P4 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 18 op | _suy luận:_ TC-PTY |
| FR-PTY-003 | Party types: Friend Party, Study Party (study_party, đổi tên từ Study Group), Quest Party, Competition Team… | P4 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 18 op | _suy luận:_ TC-PTY |
| FR-PTY-004 | Max members cấu hình được; mặc định 4, cho phép 2–8 | P4 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 18 op | _suy luận:_ TC-PTY |
| FR-PTY-005 | Một học sinh chỉ có một active Party tại một thời điểm (Addendum §6 ghi cho MVP; giữ nguyên cho Release 1.0… | P4 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 18 op | _suy luận:_ TC-PTY |
| FR-PTY-006 | Party Lobby: danh sách thành viên, avatar, online, Ready/Not Ready, leader controls, chọn activity & game m… | P4 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 18 op | _suy luận:_ TC-PTY |
| FR-PTY-007 | Start Session: xác nhận leader → kiểm tra thành viên → kiểm tra điều kiện khối/quyền → tạo Game Session/roo… | P4 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 18 op | _suy luận:_ TC-PTY |
| FR-PTY-008 | Leader disconnect: grace period 60 giây, sau đó tự chuyển leader cho thành viên tham gia sớm nhất còn online | P4 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 18 op | _suy luận:_ TC-PTY |
| FR-PTY-009 | Party private cần lời mời/quyền; lời mời hết hạn sau 5 phút; không mời người đã block/bị block | P4 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 18 op | _suy luận:_ TC-PTY |
| FR-PTY-010 | Giới hạn theo school/grade/activity; mặc định chỉ cùng trường; lớp 1–3 chỉ friend/quest, cùng đúng khối, kh… | P4 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 18 op | _suy luận:_ TC-PTY |
| FR-PTY-011 | Chống spam lời mời (rate limit), audit log cho create/kick/transfer/disband | P4 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 18 op | _suy luận:_ TC-PTY |
| FR-PTY-012 | Thành viên mất kết nối: các thành viên khác tiếp tục nếu activity cho phép (continue_on_disconnect) | P4 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 18 op | _suy luận:_ TC-PTY |
| FR-PTY-013 | Không chia sẻ kết quả học tập cá nhân giữa thành viên; mỗi người có attempt riêng | P4 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 18 op | _suy luận:_ TC-PTY |
| FR-PTY-014 | API /api/v1/multiplayer/parties, /api/v1/multiplayer/invitations; realtime event qua /realtime/v1 | P4 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | `createParty` · `listMyInvitations` | _suy luận:_ TC-PTY |

### 3.16. `MM` Matchmaking

Service: module competition · Tài liệu sở hữu: [MATCHMAKING](../03-social-multiplayer/MATCHMAKING.md)

AC: — · US: US-MM-001

Mở rộng ở tài liệu sở hữu: 0 FR, 8 AC, 0 US — [MATCHMAKING](../03-social-multiplayer/MATCHMAKING.md)

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-MM-001 | Chế độ: Random, Skill-based, Grade-based, Subject-based, Topic-based, Friend, Team, Tournament | P4 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 4 op | _suy luận:_ TC-MM |
| FR-MM-002 | Ràng buộc cứng (khối lớp, độ phù hợp nội dung, policy trường) kiểm tra trước rating; không ghép lớp 1 với l… | P4 | [MATCHMAKING](../03-social-multiplayer/MATCHMAKING.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) · [ADR-0095-bo-khai-niem-lop-hoc-sinh-theo-khoi](../adr/ADR-0095-bo-khai-niem-lop-hoc-sinh-theo-khoi.md) | _suy luận:_ 4 op | _suy luận:_ TC-MM |
| FR-MM-003 | Mặc định: lớp 1–5 chỉ ghép cùng khối (grade_level); lớp 6–9 theo grade_band (±1 khối khi chờ lâu); chỉ cùng… | P4 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 4 op | _suy luận:_ TC-MM |
| FR-MM-004 | Vòng đời: Queue → Ticket → Match Proposal → Lobby → Ready Check → Team Assignment → Match Start → Match End… | P4 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 4 op | _suy luận:_ TC-MM |
| FR-MM-005 | Cửa sổ rating mở rộng theo thời gian chờ; timeout ghép trận 90 giây đề xuất chế độ khác/bot luyện tập | P4 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 4 op | _suy luận:_ TC-MM |
| FR-MM-006 | Ticket chỉ trong Valkey; mất → client tạo lại (ADR-0004) | P4 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 4 op | _suy luận:_ TC-MM |

### 3.17. `CMP` Competition / Battle

Service: module competition · Tài liệu sở hữu: [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md)

AC: AC-CMP-001, AC-CMP-002, AC-CMP-003, AC-CMP-004, AC-CMP-005, AC-CMP-006 · US: —

Mở rộng ở tài liệu sở hữu: 0 FR, 13 AC, 0 US — [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) · [ANTI_CHEAT](../10-security/ANTI_CHEAT.md)

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-CMP-001 | Chế độ: 1v1 Knowledge Battle, Team Battle, Co-op Challenge, Timed Quiz, Subject Tournament, Grade/School/Se… | P4 (Seasonal: P6+) | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 7 op | _suy luận:_ TC-CMP |
| FR-CMP-002 | Battle là state machine server-authoritative: question plan do server chọn, chấm và tính điểm thời gian ser… | P4 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 7 op | _suy luận:_ TC-CMP |
| FR-CMP-003 | Kết quả trận ghi một lần (unique match_id), idempotent, audit | P4 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 7 op | _suy luận:_ TC-CMP |
| FR-CMP-004 | Câu trả lời trong trận ghi learning evidence với context competition (trọng số thấp cho mastery) | P4 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 7 op | _suy luận:_ TC-CMP |
| FR-CMP-005 | Xử lý disconnect: grace 30 giây; quá hạn → xử thua phần còn lại/forfeit theo mode | P4 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 7 op | _suy luận:_ TC-CMP |
| FR-CMP-006 | Không có bonus điểm tốc độ trong 1v1 cho lớp 1–5 (COMPETITION_SYSTEM) | P4 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 7 op | _suy luận:_ TC-CMP |

### 3.18. `RNK` Ranking

Service: module competition · Tài liệu sở hữu: [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md)

Mở rộng ở tài liệu sở hữu: 0 FR, 6 AC, 0 US — [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md)

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-RNK-001 | Hệ thống ranking độc lập; Glicko-2 theo (student, mode, subject, season) (ADR-0018) | P4 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 1 op | _suy luận:_ TC-CMP |
| FR-RNK-002 | Không dùng rating để đánh giá năng lực học tập; tách Learning Score, Mastery, Game Rating, Game XP, Competi… | P4 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 1 op | _suy luận:_ TC-CMP |
| FR-RNK-003 | Hiển thị rating cho học sinh dưới dạng tier thân thiện; không hiển thị số rating hay rating_delta cho học s… | P4 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) · [SPEC_RECONCILIATION_BACKLOG](SPEC_RECONCILIATION_BACKLOG.md) | _suy luận:_ 1 op | _suy luận:_ TC-CMP |
| FR-RNK-004 | Season soft-reset rating | P4 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 1 op | _suy luận:_ TC-CMP |

### 3.19. `LDB` Leaderboard

Service: module competition · Tài liệu sở hữu: [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md)

AC: — · US: US-LDB-001

Mở rộng ở tài liệu sở hữu: 0 FR, 7 AC, 0 US — [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md)

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-LDB-001 | Bảng: Global, School, Grade, Subject, Topic, Game Mode, Friends, Team, Daily, Weekly, Monthly, Season | P4 (Global trong Release 1.0; quy tắc hiển thị nickname theo FR-LDB-003) | [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) · [ADR-0095-bo-khai-niem-lop-hoc-sinh-theo-khoi](../adr/ADR-0095-bo-khai-niem-lop-hoc-sinh-theo-khoi.md) | _suy luận:_ 4 op | _suy luận:_ TC-CMP |
| FR-LDB-002 | Leaderboard cache bằng Valkey sorted set, nguồn từ PostgreSQL | P4 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 4 op | _suy luận:_ TC-CMP |
| FR-LDB-003 | Leaderboard ngoài phạm vi trường chỉ hiển thị nickname, không tên trường/khối chi tiết | P4 | [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 4 op | _suy luận:_ TC-CMP |
| FR-LDB-004 | Trường có thể tắt leaderboard công khai cho lớp 1–3 | P4 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 4 op | _suy luận:_ TC-CMP |
| FR-LDB-005 | Mục tiêu tập thể của khối trong trường (grade_goal) thay bảng của lớp: hiển thị tiến độ chung, không xếp hạ… | P4 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 4 op | _suy luận:_ TC-CMP |

### 3.20. `TRN` Tournament

Service: module competition · Tài liệu sở hữu: [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md)

AC: — · US: US-TRN-001

Mở rộng ở tài liệu sở hữu: 0 FR, 8 AC, 0 US — [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md)

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-TRN-001 | Format: Single Elimination, Double Elimination, Round Robin, Team, School, Grade Tournament (giải khối của… | P4 (Single elim, Round robin bật mặc định; Double elim, vòng bảng, Team, School, Grade xây cùng tournament engine ở P4, tắt theo template tới khi platform bật) | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 12 op | _suy luận:_ TC-CMP |
| FR-TRN-002 | Tạo giải, đăng ký, chia bảng, tạo bracket, lên lịch, xác nhận người tham gia, quản lý trận, xác nhận kết qu… | P4 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 12 op | _suy luận:_ TC-CMP |
| FR-TRN-003 | Bracket lưu PostgreSQL, không chỉ Valkey | P4 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 12 op | _suy luận:_ TC-CMP |

### 3.21. `SOC` Social

Service: module chat · Tài liệu sở hữu: [SOCIAL_SYSTEM](../03-social-multiplayer/SOCIAL_SYSTEM.md)

AC: — · US: US-SOC-001

Mở rộng ở tài liệu sở hữu: 0 FR, 11 AC, 0 US — [SOCIAL_SYSTEM](../03-social-multiplayer/SOCIAL_SYSTEM.md)

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-SOC-001 | Friend list, friend request, invitation; tenant common tắt mặc định kết bạn (ADR-0078) | P5 (cơ bản cho mời party: P4) | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) · [ROADMAP](ROADMAP.md) | _suy luận:_ 37 op | _suy luận:_ TC-SOC |
| FR-SOC-002 | Mặc định chỉ kết bạn cùng trường (như nhau cho mọi khối); tenant unit có thể mở liên trường trong tenant | P5 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 37 op | _suy luận:_ TC-SOC |
| FR-SOC-003 | Block / Mute người dùng; block chặn chat, mời, ghép party | P5 (Block: P4 cùng party) | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 37 op | _suy luận:_ TC-SOC |
| FR-SOC-004 | Team/club của học sinh theo policy (SOCIAL_SYSTEM); nhóm học tập LMS (Study Group, ADR-0084) đã bỏ theo ADR… | P5 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 37 op | _suy luận:_ TC-SOC |

### 3.22. `CHAT` Chat

Service: module chat · Tài liệu sở hữu: [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md)

AC: AC-CHAT-001, AC-CHAT-002, AC-CHAT-003, AC-CHAT-004, AC-CHAT-005, AC-CHAT-006, AC-CHAT-007, AC-CHAT-008, AC-CHAT-009, AC-CHAT-010, AC-CHAT-011, AC-CHAT-012, AC-CHAT-013, AC-CHAT-014, AC-CHAT-015 · US: US-CHAT-001, US-CHAT-002

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-CHAT-001 | Channel: World, Area, Match, Party, Team, Direct Message; cấu hình bằng policy; không có channel Class/Stud… | P4 (Match/Party/Team theo trận), P5 (còn lại) | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 18 op | _suy luận:_ TC-CHAT |
| FR-CHAT-002 | Không có rule = deny; quyền vào channel theo quan hệ (cùng trường/instance/nhóm/bạn bè) và policy, như nhau… | P5 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 18 op | _suy luận:_ TC-CHAT |
| FR-CHAT-003 | Text, emoji, reactions, quick messages, group chat, history, unread count, notifications | P4 (channel gameplay), P5 (còn lại) | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 18 op | _suy luận:_ TC-CHAT |
| FR-CHAT-004 | Một bộ quy tắc chat cho mọi lớp 1–9 (không điều kiện theo grade_level/grade_band): đủ 7 channel; World = cù… | P5 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 18 op | _suy luận:_ TC-CHAT |
| FR-CHAT-005 | Mọi tin nhắn phải hợp lệ: pipeline đồng bộ phía server cho mọi kind — normalize → định dạng/độ dài → profan… | P5 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 18 op | _suy luận:_ TC-CHAT |
| FR-CHAT-006 | Lịch sử chat theo quyền và retention job (mặc định 180 ngày; rà soát thời hạn theo luật: Deferred ADR-0082) | P5 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 18 op | _suy luận:_ TC-CHAT |
| FR-CHAT-007 | Không có voice chat trong Release 1.0; điểm mở rộng chỉ ở mức thiết kế, không hiện thực (ADR-0051, ADR-0070) | — | [DATABASE_DESIGN](../07-backend/DATABASE_DESIGN.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 18 op | _suy luận:_ TC-CHAT |
| FR-CHAT-008 | Giao tin nhắn realtime qua /realtime/v1, gửi qua REST hoặc WS có idempotency client message id | P4 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 18 op | _suy luận:_ TC-CHAT |
| FR-CHAT-009 | Chat gameplay luôn có (ADR-0020 mục 6): vào trận 1v1 → channel match; trận đội → team mỗi đội + match chung… | P4 | [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md) · [PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) · [SPEC_RECONCILIATION_BACKLOG](SPEC_RECONCILIATION_BACKLOG.md) | _suy luận:_ 18 op | _suy luận:_ TC-CHAT |
| FR-CHAT-010 | UI chat trong lobby/trận/party: panel thu gọn được nhưng không tắt; quick message một chạm; tab Đội/Cả trận… | P4 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 18 op | _suy luận:_ TC-CHAT |
| FR-CHAT-011 | Purge chat gameplay (ADR-0033): channel match, team theo trận, party (sau giải tán) bị xóa vật lý sau gamep… | P4 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 18 op | _suy luận:_ TC-CHAT |
| FR-CHAT-012 | Sửa tin trong 15 phút (chạy lại kiểm duyệt, lưu revision) và thu hồi trong 24 giờ (placeholder, giữ bằng ch… | P4 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 18 op | _suy luận:_ TC-CHAT |
| FR-CHAT-013 | Reply, Quote (trích từ bản đã che), Forward (cùng tenant, ≤ 5 đích, chạy lại kiểm duyệt) — ADR-0034 | P4 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 18 op | _suy luận:_ TC-CHAT |
| FR-CHAT-014 | Emoji, Sticker (pack platform, có thể mở khóa bằng phần thưởng), GIF (thư viện nội bộ đã duyệt, không dịch… | P4 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 18 op | _suy luận:_ TC-CHAT |
| FR-CHAT-015 | Mention @nickname (≤ 5/tin, thông báo + đếm riêng), Pin (≤ 3/channel), Copy (bản đã che), Typing indicator… | P4 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 18 op | _suy luận:_ TC-CHAT |
| FR-CHAT-016 | Trạng thái Sent/Delivered/Read theo con trỏ, Unread count + mention count, Lịch sử phân trang và tìm kiếm f… | P4 (tìm kiếm P5) | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 18 op | _suy luận:_ TC-CHAT |
| FR-CHAT-017 | Truyền tải chat qua NATS JetStream (bền vững, trước khi xác nhận Sent) + Valkey Pub/Sub (fan-out realtime),… | P4 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 18 op | _suy luận:_ TC-CHAT |

### 3.23. `MOD` Moderation & Child Safety

Service: module chat · Tài liệu sở hữu: [MODERATION](../03-social-multiplayer/MODERATION.md) · [CHILD_SAFETY](../10-security/CHILD_SAFETY.md)

AC: AC-MOD-020, AC-MOD-021, AC-MOD-022, AC-MOD-026, AC-MOD-027, AC-MOD-030 · US: US-MOD-001, US-MOD-002, US-MOD-003, US-MOD-004

Mở rộng ở tài liệu sở hữu: 0 FR, 25 AC, 0 US — [MODERATION](../03-social-multiplayer/MODERATION.md)

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-MOD-001 | Content/profanity filtering, anti-spam, anti-bullying, PII detection | P5 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 41 op | _suy luận:_ TC-CHAT |
| FR-MOD-002 | Report message, report user, block, mute | P5 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 41 op | _suy luận:_ TC-CHAT |
| FR-MOD-003 | Moderation queue tập trung ở Platform Safety Team (role safety_moderator); mọi case gắn tenant/trường; audi… | P5 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) · [EDU_FIT_REVIEW](EDU_FIT_REVIEW.md) | _suy luận:_ 41 op | _suy luận:_ TC-CHAT |
| FR-MOD-004 | Chat policy theo platform → tenant → school (bỏ cấp class — ADR-0095; không theo grade — thay mục "Age/Grad… | P5 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) · [ADR-0095-bo-khai-niem-lop-hoc-sinh-theo-khoi](../adr/ADR-0095-bo-khai-niem-lop-hoc-sinh-theo-khoi.md) | _suy luận:_ 41 op | _suy luận:_ TC-CHAT |
| FR-MOD-005 | Không ai có quyền đọc toàn bộ chat của mọi trường; truy cập chat đều audit | P5 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 41 op | _suy luận:_ TC-CHAT |
| FR-MOD-006 | Sanction: warn, mute có thời hạn, restrict (chỉ quick message), escalate | P5 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 41 op | _suy luận:_ TC-CHAT |
| FR-MOD-007 | Removed (ADR-0072) — điều khiển channel lớp của giáo viên; channel lớp chịu chat policy cấp trường do schoo… | — | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 41 op | _suy luận:_ TC-CHAT |
| FR-MOD-008 | Không giám sát chat vượt quá mục đích an toàn cần thiết | P5 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 41 op | _suy luận:_ TC-CHAT |
| FR-MOD-009 | Danh mục từ nhạy cảm quản lý trong CMS: category (severity, action mặc định), term (match type, biến thể tự… | P5 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 41 op | _suy luận:_ TC-CHAT |
| FR-MOD-010 | Action theo mức: mask / block / hold_for_review / block_and_alert; phản hồi không lộ từ đã khớp | P5 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 41 op | _suy luận:_ TC-CHAT |
| FR-MOD-011 | Công tắc kiểm duyệt bật/tắt toàn bộ hoặc từng bước (sensitive_terms, spam, bullying, async_analysis) theo p… | P5 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 41 op | _suy luận:_ TC-CHAT |
| FR-MOD-012 | Khi tắt vẫn giữ PII, grooming, độ dài tối đa, rate limit, quyền channel, sanction, block/mute/report, moder… | P5 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 41 op | _suy luận:_ TC-CHAT |
| FR-MOD-013 | Từ bổ sung cấp tenant (chỉ siết chặt) và bật/tắt category cấp tenant (chỉ category có allow_tenant_toggle)… | P5 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) · [SPEC_RECONCILIATION_BACKLOG](SPEC_RECONCILIATION_BACKLOG.md) | _suy luận:_ 41 op | _suy luận:_ TC-CHAT |
| FR-MOD-014 | Mọi lần tắt bắt buộc có expires_at ≤ 24 giờ (moderation_switch_max_duration), tự bật lại khi hết hạn (ADR-0… | P5 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 41 op | _suy luận:_ TC-CHAT |
| FR-MOD-015 | Publish danh mục chạy test hồi quy (recall ≥ 95%, false positive ≤ 3%) và reload không downtime ≤ 60 s | P5 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 41 op | _suy luận:_ TC-CHAT |
| FR-MOD-016 | safety_moderator đọc liên tenant chỉ trong module moderation (case, bằng chứng, tin nhắn liên quan case) qu… | P5 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 41 op | _suy luận:_ TC-CHAT |
| FR-MOD-017 | Thông báo nhà trường về vụ việc nghiêm trọng liên quan học sinh của trường: tóm tắt, không chứa nội dung ch… | P5 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 41 op | _suy luận:_ TC-CHAT |
| FR-MOD-018 | Safety Team trực 24/7 (on-call) cho case mức p1 (grooming, tự hại) (MODERATION, CHILD_SAFETY) | P5 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 41 op | _suy luận:_ TC-CHAT |

### 3.24. `NTF` Notification

Service: module chat · Tài liệu sở hữu: [SOCIAL_SYSTEM](../03-social-multiplayer/SOCIAL_SYSTEM.md)

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-NTF-001 | Thông báo trong game: lời mời party/kết bạn, bài được giao, kết quả, phần thưởng, thông báo trường | P2 (in-game) – P5 (realtime đầy đủ) | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 3 op | _suy luận:_ TC-SOC |
| FR-NTF-002 | Realtime qua WebSocket; lưu bền vững để xem lại (/api/v1/notifications) | P5 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | `listNotifications` | _suy luận:_ TC-SOC |
| FR-NTF-003 | Không gửi push notification ngoài game tới học sinh trong Release 1.0; thông báo chỉ hiển thị trong game (i… | P5 | [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | `listNotifications` | _suy luận:_ TC-SOC |

### 3.25. `GAM` XP, level, tiến trình

Service: module game · Tài liệu sở hữu: [PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md)

AC: AC-GAM-001, AC-GAM-002, AC-GAM-003, AC-GAM-011 · US: US-GAM-001

Mở rộng ở tài liệu sở hữu: 0 FR, 10 AC, 0 US — [PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md) · [WORLD_DESIGN](../01-game-design/WORLD_DESIGN.md)

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-GAM-001 | XP, Level, Achievement, Badge, Daily/Weekly Challenge, Season Reward, Collection | P2 (XP, Level, Badge) – P4 (Season) | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 5 op | _suy luận:_ TC-GAM |
| FR-GAM-002 | XP từ hoàn thành bài học, bài tập, quest, hoạt động, hợp tác, thi đấu | P2–P4 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 5 op | _suy luận:_ TC-GAM |
| FR-GAM-003 | Reward Engine có rule + cap ngày/tuần + diminishing returns khi lặp nội dung; chống farm câu dễ | P2 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 5 op | _suy luận:_ TC-GAM |
| FR-GAM-004 | XP ledger append-only, idempotent theo (source_type, source_id, student_id, rule_id) | P2 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 5 op | _suy luận:_ TC-GAM |
| FR-GAM-005 | Phần thưởng game không ảnh hưởng điểm/mastery/đánh giá chính thức | P2 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 5 op | _suy luận:_ TC-GAM |
| FR-GAM-006 | Ba miền tiến trình (game, learning, competition) tách biệt, không gộp điểm; game level không phải bằng chứn… | P2 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 5 op | _suy luận:_ TC-GAM |
| FR-GAM-007 | Catalog nguồn XP có mã; đường cong XP cấu hình được, có version, không hardcode trong Godot; danh hiệu theo… | P2 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 5 op | _suy luận:_ TC-GAM |
| FR-GAM-008 | Level-up event, phần thưởng lên cấp, progress bar, lịch sử level và giao dịch XP | P2 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 5 op | _suy luận:_ TC-GAM |
| FR-GAM-009 | World progression: mở khóa khu theo level/quest/LO/achievement, điểm khám phá, vật sưu tầm, hoàn thành bản… | P2–P3 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 5 op | _suy luận:_ TC-GAM |
| FR-GAM-010 | Quy tắc chuyển năm học khi học sinh lên lớp: giữ level, vật phẩm và vùng đã mở; nội dung học tập và quest l… | P2 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) · [ADR-0095-bo-khai-niem-lop-hoc-sinh-theo-khoi](../adr/ADR-0095-bo-khai-niem-lop-hoc-sinh-theo-khoi.md) | _suy luận:_ 5 op | _suy luận:_ TC-GAM |
| FR-GAM-011 | Không giảm XP cho hoạt động ôn LO tiên quyết do hệ thống gợi ý hoặc nhà trường giao (kể cả nội dung khối dư… | P2 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 5 op | _suy luận:_ TC-GAM |

### 3.26. `RWD` Phần thưởng

Service: module game · Tài liệu sở hữu: [PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md) · [INVENTORY_ECONOMY](../01-game-design/INVENTORY_ECONOMY.md)

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-RWD-001 | Inventory, cosmetic (outfit, hair, accessory, emote) mở khóa bằng bài tập, quest, sự kiện, thành tựu, thi đấu | P2 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 12 op | _suy luận:_ TC-GAM, TC-ECO |
| FR-RWD-002 | Không cơ chế trả tiền để có kết quả học tập tốt hơn; không loot box | P2 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 12 op | _suy luận:_ TC-GAM, TC-ECO |
| FR-RWD-003 | Reward có giá trị nghiệp vụ chỉ do server trao | P2 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 12 op | _suy luận:_ TC-GAM, TC-ECO |

### 3.27. `CHL` Challenge & Boss

Service: module game · Tài liệu sở hữu: [CHALLENGE_BOSS_SYSTEM](../01-game-design/CHALLENGE_BOSS_SYSTEM.md)

Mở rộng ở tài liệu sở hữu: 0 FR, 8 AC, 0 US — [CHALLENGE_BOSS_SYSTEM](../01-game-design/CHALLENGE_BOSS_SYSTEM.md)

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-CHL-001 | Challenge Engine: 11 loại (time, perfect, survival, puzzle, knowledge, boss, coop, party, weekly, seasonal,… | P3 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 17 op | _suy luận:_ TC-CHL |
| FR-CHL-002 | Boss Battle nhiều phase, đóng góp cá nhân/nhóm, retry, không phạt khi sai; server-authoritative trong modul… | P3–P4 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 17 op | _suy luận:_ TC-CHL |
| FR-CHL-003 | Mỗi câu/hoạt động trong challenge/boss là attempt học tập cá nhân; learning record riêng từng thành viên | P3 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 17 op | _suy luận:_ TC-CHL |

### 3.28. `ECO` Kinh tế, vật phẩm, sưu tầm

Service: module game · Tài liệu sở hữu: [INVENTORY_ECONOMY](../01-game-design/INVENTORY_ECONOMY.md)

Mở rộng ở tài liệu sở hữu: 0 FR, 8 AC, 0 US — [INVENTORY_ECONOMY](../01-game-design/INVENTORY_ECONOMY.md)

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-ECO-001 | Item catalog (cosmetic, outfit, hair, accessory, emote, pet, title, badge, frame, decoration, knowledge car… | P2 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 13 op | _suy luận:_ TC-ECO |
| FR-ECO-002 | Tiền trong game "Xu" + sổ cái append-only, ví không âm, đối soát; không mua bằng tiền thật (ADR-0049) | P2 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 13 op | _suy luận:_ TC-ECO |
| FR-ECO-003 | Shop chỉ cosmetic, giá Xu, giới hạn mua, vật phẩm xác định; không loot box/hộp ngẫu nhiên trả phí | P2 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 13 op | _suy luận:_ TC-ECO |
| FR-ECO-004 | Collection (pet, knowledge card, cosmetic, badge, exploration, event), xử lý trùng lặp, trading tắt mặc định | P2–P3 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 13 op | _suy luận:_ TC-ECO |
| FR-ECO-005 | Achievement có định nghĩa, trigger, điều kiện, tiến độ, ẩn, tier, version; không dựa trên thời gian chơi | P2 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 13 op | _suy luận:_ TC-ECO |

### 3.29. `EVT` Event & Season (live-ops)

Service: module game · Tài liệu sở hữu: [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md)

Mở rộng ở tài liệu sở hữu: 0 FR, 12 AC, 0 US — [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md)

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-EVT-001 | Sự kiện live-ops: lễ hội thế giới, cuối tuần, sự kiện trường, community goal, chiến dịch học tập, boss đặc… | P3 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 19 op | _suy luận:_ TC-EVT |
| FR-EVT-002 | Event Builder trong Game Operations Console: lịch, đối tượng (khối/trường/tenant), nội dung tham chiếu, phầ… | P3 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 19 op | _suy luận:_ TC-EVT |
| FR-EVT-003 | Vòng đời Scheduled → Live → Ended → Archived, dừng khẩn cấp/tiếp tục/hủy có lý do và audit; lượt chơi đang… | P3 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 19 op | _suy luận:_ TC-EVT |
| FR-EVT-004 | Community goal cộng dồn theo trường/tenant từ event server đã kiểm chứng, trần đóng góp mỗi học sinh/ngày,… | P3 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | `getSchoolEventProgress` | _suy luận:_ TC-EVT |
| FR-EVT-005 | Season track miễn phí (không track trả phí), điểm mùa từ hoạt động đa dạng có trần/ngày, thưởng cosmetic; h… | P4 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 19 op | _suy luận:_ TC-EVT |
| FR-EVT-006 | Tenant tổ chức sự kiện trường từ template platform, chỉ với allowed_params (ADR-0027, ADR-0029) | P3 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 19 op | _suy luận:_ TC-EVT |
| FR-EVT-007 | Quy tắc lành mạnh: không phạt mất chuỗi, không ép online, không khóa học tập, tôn trọng giờ truy cập và nhắ… | P3 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 19 op | _suy luận:_ TC-EVT |
| FR-EVT-008 | Nội dung sự kiện platform song ngữ bắt buộc (ADR-0056); trang trí trong ngân sách máy tầm trung, có biến th… | P3 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 19 op | _suy luận:_ TC-EVT |

### 3.30. `CMS` CMS

Service: module learning · module game (game-admin) · Tài liệu sở hữu: [ADMIN_PORTAL](../04-admin-portal/ADMIN_PORTAL.md)

AC: AC-CMS-001, AC-CMS-002, AC-CMS-003, AC-CMS-004, AC-CMS-005 · US: US-CMS-001, US-CMS-002

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-CMS-001 | Vai trò tenant: tenant_admin, school_admin, academic_manager, tenant_content_editor, tenant_content_reviewe… | P3 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 135 op | _suy luận:_ TC-CMS |
| FR-CMS-002 | Curriculum Management (chương trình, khối, môn, chủ đề, LO) | P3 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 135 op | _suy luận:_ TC-CMS |
| FR-CMS-003 | Content Management: lesson, material, media, question bank, question set, assessment, exercise, learning ac… | P3 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 135 op | _suy luận:_ TC-CMS |
| FR-CMS-004 | Game Content: quest, mini-game config, game level, NPC dialogue, reward config, world event, game activity… | P3 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 135 op | _suy luận:_ TC-CMS |
| FR-CMS-005 | Competition Management: matchmaking config, ranking rules, tournament/challenge template, season, competiti… | P4 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 135 op | _suy luận:_ TC-CMS |
| FR-CMS-006 | Workflow, versioning, preview (Godot Web export nhúng cho nội dung game), publishing, rollback theo chính s… | P3 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 135 op | _suy luận:_ TC-CMS |
| FR-CMS-007 | CMS không làm thay đổi lịch sử bài đã hoàn thành | P3 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 135 op | _suy luận:_ TC-CMS |
| FR-CMS-008 | Nội dung phạm vi tenant do tenant_content_editor soạn, tenant_content_reviewer duyệt (OQ-07 đã trả lời bằng… | P3 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 135 op | _suy luận:_ TC-CMS |
| FR-CMS-009 | Một Admin Portal React (TypeScript) src/cms/frontend/apps/portal với 5 workspace (School & Learning Managem… | P3 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 135 op | _suy luận:_ TC-CMS |
| FR-CMS-016 | Dashboard live-ops trong CMS/Admin: CCU, trận đang chạy, hàng chờ ghép trận, đăng nhập/hàng chờ đăng nhập,… | P3 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 135 op | _suy luận:_ TC-CMS |
| FR-CMS-017 | Thumbnail/turntable render tự động từ asset 3D của game cho mọi version publish, hiển thị trong CMS như dữ… | P3 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 135 op | _suy luận:_ TC-CMS |
| FR-CMS-018 | Danh mục menu/action của Admin Portal và bật/tắt trong DB (ADR-0090): danh mục workspace → menu → action là… | P1 | _suy luận:_ [ADMIN_PORTAL](../04-admin-portal/ADMIN_PORTAL.md) | _suy luận:_ 135 op | _suy luận:_ TC-CMS |
| FR-CMS-010 | Vận hành của mỗi tenant đăng nhập Admin Portal (workspace School & Learning Management, Learning Content St… | P3 | [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 135 op | _suy luận:_ TC-CMS |
| FR-CMS-011 | Vận hành tenant không thấy và không thao tác được menu/cấu hình game; các lớp chặn: gateway nội bộ, azp + r… | P3 | [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 135 op | _suy luận:_ TC-CMS |
| FR-CMS-012 | Menu dựng từ GET /api/v1/me/permissions; permission catalog trong code, role tenant không có game.*/competi… | P1 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | `getMyPermissions` | _suy luận:_ TC-CMS |
| FR-CMS-013 | System Admin là người duy nhất xem dữ liệu liên tenant: dropdown tìm kiếm tenant (mặc định "Tất cả", chọn n… | P3 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 135 op | _suy luận:_ TC-CMS |
| FR-CMS-014 | Xóa = soft-delete; dữ liệu đã xóa không hiển thị; Thùng rác cho System Admin (Tenant Admin nếu được cấp) | P2 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 135 op | _suy luận:_ TC-CMS |
| FR-CMS-015 | System Admin gán dữ liệu platform cho tenant (chỉ đọc với tenant) và clone dữ liệu cấu hình/nội dung giữa t… | P2 (gán) – P3 (clone) | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 135 op | _suy luận:_ TC-CMS |

### 3.31. `LMS` LMS

Service: module learning · Tài liệu sở hữu: [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md)

AC: — · US: US-LMS-001, US-LMS-002, US-LMS-003, US-LMS-004, US-LMS-005, US-LMS-006, US-LMS-007, US-LMS-008, US-LMS-009, US-LMS-010, US-LMS-011

Mở rộng ở tài liệu sở hữu: 0 FR, 17 AC, 0 US — [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md)

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-LMS-001 | Module: Assignment, Lesson Delivery, Assessment, Learning Progress, Learning Analytics, School Learning Das… | P3 | [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 34 op | _suy luận:_ TC-LMS, TC-DASH |
| FR-LMS-002 | school_admin/academic_manager giao bài tập, giao đề ôn tập (target: khối của một trường + thêm học sinh khá… | P3 | [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 34 op | _suy luận:_ TC-LMS, TC-DASH |
| FR-LMS-003 | Nhà trường theo dõi kết quả, tiến độ, kỹ năng yếu theo LO qua School Learning Dashboard (FR-LMS-011) | P3 | [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 34 op | _suy luận:_ TC-LMS, TC-DASH |
| FR-LMS-004 | Chỉ school_admin tổ chức thử thách khối/giải trường từ template; nhóm học tập đã bỏ theo ADR-0095 | P4 | [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 34 op | _suy luận:_ TC-LMS, TC-DASH |
| FR-LMS-005 | Phân quyền theo trường và môn (scope school, subject của UserRole); không có scope lớp theo phân công giảng… | P3 | [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 34 op | _suy luận:_ TC-LMS, TC-DASH |
| FR-LMS-006 | Bài được giao xuất hiện trong game như quest/nhiệm vụ đặc biệt, ở tab "Nhà trường giao" | P3 | [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md) · [STUDENT_GAME_UX_CORE](../05-ux/STUDENT_GAME_UX_CORE.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 34 op | _suy luận:_ TC-LMS, TC-DASH |
| FR-LMS-007 | Nhà trường chỉ chọn quest/mini-game đã publish và được gán cho tenant để giao bài; thử thách/giải trường tạ… | P2–P4 | [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 34 op | _suy luận:_ TC-LMS, TC-DASH |
| FR-LMS-011 | School Learning Dashboard cho school_admin/academic_manager: trường → khối → học sinh (ADR-0095), tiến độ b… | P3 | [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | `getSchoolDashboard` | _suy luận:_ TC-LMS, TC-DASH |
| FR-LMS-012 | Thao tác cá nhân trên bài giao: gia hạn due_at (extend), miễn (excuse), nhắc học sinh chưa làm (remind, giớ… | P3 | [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 34 op | _suy luận:_ TC-LMS, TC-DASH |
| FR-LMS-013 | Bảng kết quả bài giao (theo khối, không course — ADR-0095) theo điểm tự chấm (không sổ điểm chấm tay, không… | P3 | [ASSESSMENT_REGULATION_PROFILE](../02-learning/ASSESSMENT_REGULATION_PROFILE.md) · [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md) · [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 34 op | _suy luận:_ TC-LMS, TC-DASH |

### 3.32. `AUD` Audit

Service: module identity · Tài liệu sở hữu: [SECURITY_ARCHITECTURE](../10-security/SECURITY_ARCHITECTURE.md)

Mở rộng ở tài liệu sở hữu: 0 FR, 4 AC, 0 US — [AUDIO_LANGUAGE_LEARNING](../02-learning/AUDIO_LANGUAGE_LEARNING.md)

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-AUD-001 | Audit log cho: đăng nhập admin, thay đổi quyền, truy cập dữ liệu học sinh chéo phạm vi, publish nội dung, k… | P1+ | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 28 op | _suy luận:_ TC-AUD |
| FR-AUD-002 | Audit log append-only, partition theo tháng, retention theo data inventory ở DATA_PRIVACY §2 (rà soát thời… | P1 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 28 op | _suy luận:_ TC-AUD |

### 3.33. `OPS` Vận hành

Service: src/backend/pkg/ (platform kit) · mọi module · Tài liệu sở hữu: [DEPLOYMENT](../11-operations/DEPLOYMENT.md) · [MONITORING](../11-operations/MONITORING.md)

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-OPS-001 | Health, readiness, liveness endpoint cho mọi binary | P1 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 20 op | _suy luận:_ TC-SVC, TC-PERF |
| FR-OPS-002 | Docker + Docker Compose local; environment example; migration; backup script; deployment docs | P1 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 20 op | _suy luận:_ TC-SVC, TC-PERF |
| FR-OPS-003 | Metrics: online players, active sessions, matchmaking queue, match duration, API latency, error rate, chat… | P1–P6 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 20 op | _suy luận:_ TC-SVC, TC-PERF |
| FR-OPS-004 | Không commit secrets; secret qua secret manager/Docker secrets | P1 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 20 op | _suy luận:_ TC-SVC, TC-PERF |
| FR-OPS-005 | CI: lint, test, spec lint/breaking check, build Go/Godot/Web theo path | P1 | [CODE_PLAN_CMS_LMS_FIRST](CODE_PLAN_CMS_LMS_FIRST.md) | _suy luận:_ 20 op | _suy luận:_ TC-SVC, TC-PERF |

## 4. Acceptance criteria → TC

| AC | Domain FR | TC tường minh |
|---|---|---|
| AC-AUTH-001 | `AUTH` | — |
| AC-AUTH-002 | `AUTH` | — |
| AC-AUTH-003 | `AUTH` | — |
| AC-AUTH-004 | `AUTH` | — |
| AC-AUTH-005 | `AUTH` | — |
| AC-AUTH-006 | `AUTH` | — |
| AC-AUTH-007 | `AUTH` | — |
| AC-AUTH-008 | `AUTH` | — |
| AC-AUTH-009 | `AUTH` | — |
| AC-AUTH-010 | `AUTH` | — |
| AC-AUTH-011 | `AUTH` | — |
| AC-AUTH-012 | `AUTH` | — |
| AC-AUTH-013 | `AUTH` | — |
| AC-AUTH-014 | `AUTH` | — |
| AC-AUTH-015 | `AUTH` | — |
| AC-AUTH-016 | `AUTH` | — |
| AC-AUTH-017 | `AUTH` | — |
| AC-AUTH-018 | `AUTH` | — |
| AC-AUTH-019 | `AUTH` | — |
| AC-AUTH-020 | `AUTH` | — |
| AC-AUTH-021 | `AUTH` | — |
| AC-AUTH-022 | `AUTH` | — |
| AC-AUTH-023 | `AUTH` | — |
| AC-AUTH-024 | `AUTH` | — |
| AC-AUTH-025 | `AUTH` | — |
| AC-AUTH-026 | `AUTH` | — |
| AC-AUTH-027 | `AUTH` | — |
| AC-AUTH-028 | `AUTH` | — |
| AC-AUTH-029 | `AUTH` | — |
| AC-AUTH-030 | `AUTH` | — |
| AC-AUTH-031 | `AUTH` | — |
| AC-AUTH-032 | `AUTH` | — |
| AC-AUTH-033 | `AUTH` | — |
| AC-AUTH-034 | `AUTH` | — |
| AC-AUTH-035 | `AUTH` | — |
| AC-AUTH-036 | `AUTH` | — |
| AC-CHAT-001 | `CHAT` | — |
| AC-CHAT-002 | `CHAT` | — |
| AC-CHAT-003 | `CHAT` | — |
| AC-CHAT-004 | `CHAT` | — |
| AC-CHAT-005 | `CHAT` | — |
| AC-CHAT-006 | `CHAT` | — |
| AC-CHAT-007 | `CHAT` | — |
| AC-CHAT-008 | `CHAT` | — |
| AC-CHAT-009 | `CHAT` | — |
| AC-CHAT-010 | `CHAT` | — |
| AC-CHAT-011 | `CHAT` | — |
| AC-CHAT-012 | `CHAT` | — |
| AC-CHAT-013 | `CHAT` | — |
| AC-CHAT-014 | `CHAT` | — |
| AC-CHAT-015 | `CHAT` | — |
| AC-CMP-001 | `CMP` | — |
| AC-CMP-002 | `CMP` | — |
| AC-CMP-003 | `CMP` | — |
| AC-CMP-004 | `CMP` | — |
| AC-CMP-005 | `CMP` | — |
| AC-CMP-006 | `CMP` | — |
| AC-CMS-001 | `CMS` | — |
| AC-CMS-002 | `CMS` | — |
| AC-CMS-003 | `CMS` | — |
| AC-CMS-004 | `CMS` | — |
| AC-CMS-005 | `CMS` | — |
| AC-CNT-001 | `CNT` | — |
| AC-CNT-002 | `CNT` | — |
| AC-CNT-003 | `CNT` | — |
| AC-CNT-004 | `CNT` | — |
| AC-CNT-005 | `CNT` | — |
| AC-CNT-006 | `CNT` | — |
| AC-CNT-007 | `CNT` | — |
| AC-CNT-008 | `CNT` | — |
| AC-EXM-001 | `EXM` | — |
| AC-EXM-002 | `EXM` | — |
| AC-EXR-001 | `EXR` | — |
| AC-EXR-002 | `EXR` | — |
| AC-EXR-003 | `EXR` | — |
| AC-EXR-004 | `EXR` | — |
| AC-EXR-005 | `EXR` | — |
| AC-EXR-006 | `EXR` | — |
| AC-EXR-007 | `EXR` | — |
| AC-EXR-008 | `EXR` | — |
| AC-GAM-001 | `GAM` | — |
| AC-GAM-002 | `GAM` | — |
| AC-GAM-003 | `GAM` | — |
| AC-GAM-011 | `GAM` | — |
| AC-MOD-020 | `MOD` | — |
| AC-MOD-021 | `MOD` | — |
| AC-MOD-022 | `MOD` | — |
| AC-MOD-026 | `MOD` | — |
| AC-MOD-027 | `MOD` | — |
| AC-MOD-030 | `MOD` | — |
| AC-ORG-001 | `ORG` | — |
| AC-ORG-002 | `ORG` | — |
| AC-ORG-003 | `ORG` | — |
| AC-ORG-004 | `ORG` | — |
| AC-ORG-005 | `ORG` | — |
| AC-ORG-006 | `ORG` | — |
| AC-ORG-007 | `ORG` | — |
| AC-PRG-001 | `PRG` | — |
| AC-PRG-002 | `PRG` | — |
| AC-PTY-001 | `PTY` | — |
| AC-PTY-002 | `PTY` | — |
| AC-PTY-003 | `PTY` | — |
| AC-PTY-004 | `PTY` | — |
| AC-PTY-005 | `PTY` | — |
| AC-PTY-006 | `PTY` | — |
| AC-PTY-007 | `PTY` | — |
| AC-PTY-008 | `PTY` | — |
| AC-PTY-009 | `PTY` | — |
| AC-PTY-010 | `PTY` | — |
| AC-PTY-011 | `PTY` | — |
| AC-PTY-012 | `PTY` | — |
| AC-PTY-013 | `PTY` | — |

## 5. Báo cáo mồ côi

Tính trên 341 FR thuộc Release 1.0 (phase P1–P6).

| Kiểm tra | Không có liên kết tường minh | Không có cả suy luận (mồ côi thật) |
|---|---|---|
| FR không có tài liệu thiết kế | 1 | 0 |
| FR không có operation OpenAPI | 318 | 0 |
| FR không có TC | 340 | 0 |
| AC không thuộc domain FR nào | — | 0 |
| US không thuộc domain FR nào | — | 0 |
| AC chưa có TC tường minh | 112 | — |
| AC mở rộng (tài liệu sở hữu) chưa có TC tường minh | 439 / 477 | — |
| FR/AC mở rộng mang mã không phải domain FR | — | 164 |
| Mã FR/US/AC được nhắc nhưng không được định nghĩa ở đâu | — | 0 |
| Tiền tố TC không ánh xạ domain FR | — | 3 |

### 5.1. Mồ côi thật

- **FR không có tài liệu thiết kế:** Không có.
- **FR không có operation (kể cả suy luận):** Không có. — có thể hợp lệ nếu yêu cầu thuần client/realtime/hạ tầng; khi đó ghi rõ trong tài liệu sở hữu.
- **FR không có TC (kể cả suy luận):** Không có.
- **AC không thuộc FR nào:** Không có.
- **US không thuộc FR nào:** Không có.
- **FR/AC mở rộng mang mã không phải domain FR** (UX/NFR; cần gắn domain FR hoặc NFR): **ARP** 001, 002, 003, 004, 005, 006 · **CHS** 001, 002, 003, 004, 005, 006, 007, 008, 009, 010, 011, 012, 013, 014, 015, 016, 017, 018, 019, 020, 021, 022, 023, 024, 025, 026, 027, 028, 029, 030, 031, 032, 033, 034, 035, 036, 037, 038, 039, 040, 041, 042, 043, 044, 045, 046 · **CPP** 001, 002, 003, 004 · **EFF** 001, 002, 003, 004, 005 · **PERF** 001, 002, 003, 004, 005, 006, 007, 008, 009, 010, 011, 012 · **UX** 0001, 0002, 0003, 0004, 0005, 0006, 0007, 0008, 0101, 0102, 0103, 0104, 0105, 0106, 0107, 0108, 0109, 0110, 0111, 0112, 0113, 0114, 0115, 0116, 0117, 0118, 0119, 0201, 0202, 0203, 0204, 0301, 0302, 0303, 0304, 0401, 0402, 0403, 0404, 0501, 0502, 0503, 0504, 0601, 0602, 0603, 0604, 0605, 0701, 0702, 0703, 0704, 101, 102, 103, 104, 105, 106, 107, 108, 109, 110, 111, 112, 113, 114, 201, 202, 203, 204, 205, 206, 207, 208, 209, 210, 211, 212, 213, 214, 215, 216, 217 · **WR** 001, 002, 003, 004, 005, 006, 007, 008
- **Mã được nhắc nhưng không được định nghĩa ở đâu:** Không có.
- **Tiền tố TC chưa ánh xạ domain:** TC-I18N (11), TC-UI (17), TC-WR (8)

### 5.2. Chỉ có liên kết suy luận (cần bổ sung liên kết tường minh)

- **FR chưa được tài liệu thiết kế nhắc mã:** **CMS** 018
- **FR chưa có TC nhắc mã:** **ANL** 001, 002, 003, 004, 005, 006, 007, 008, 009, 010, 011, 012, 013, 014, 015, 016, 017, 018, 019, 020, 021, 022, 023 · **AUD** 001, 002 · **AUTH** 001, 002, 003, 004, 005, 007, 008, 009, 010, 011, 012, 013, 014, 016, 017, 018, 019, 021, 022, 023, 027, 028, 029, 030, 031, 032, 033, 034, 035, 036 · **CHAT** 001, 002, 003, 004, 005, 006, 008, 009, 010, 011, 012, 013, 014, 015, 016, 017 · **CHL** 001, 002, 003 · **CMP** 001, 002, 003, 004, 005, 006 · **CMS** 001, 002, 003, 004, 005, 006, 007, 008, 009, 010, 011, 012, 013, 014, 015, 016, 017, 018 · **CNT** 001, 002, 003, 004, 005, 006, 007, 008, 009, 010, 011, 012, 013, 014, 015, 016, 017, 018, 019, 020, 021 · **CUR** 001, 002, 003, 004, 005, 006, 007, 008, 009, 010, 011, 012 · **ECO** 001, 002, 003, 004, 005 · **EVT** 001, 002, 003, 004, 005, 006, 007, 008 · **EXM** 001, 002, 003, 004, 005, 006, 008, 009, 010 · **EXR** 001, 002, 003, 004, 005, 006, 007, 008, 012, 013, 014, 015, 016 · **GAM** 001, 002, 003, 004, 005, 006, 007, 008, 009, 010, 011 · **GAME** 001, 002, 003, 004, 005, 006, 007, 008, 009, 010, 011, 012, 013, 014, 015, 016, 017, 018, 019, 020, 021 · **LDB** 001, 002, 003, 004, 005 · **LMS** 001, 002, 003, 004, 005, 006, 007, 011, 012, 013 · **MG** 001, 002, 003, 004 · **MM** 001, 002, 003, 004, 005, 006 · **MOD** 001, 002, 003, 004, 005, 006, 008, 009, 010, 011, 012, 013, 014, 015, 016, 017, 018 · **MP** 001, 002, 003, 004, 005, 006 · **NTF** 001, 002, 003 · **OPS** 001, 002, 003, 004, 005 · **ORG** 001, 002, 003, 005, 006, 007, 008, 009, 010, 011, 012, 013, 014, 015, 016, 017, 018 · **PRG** 001, 002, 003, 004, 005, 006, 007, 008, 009 · **PTY** 001, 002, 003, 004, 005, 006, 007, 008, 009, 010, 011, 012, 013, 014 · **QB** 001, 002, 003, 004, 005, 006, 007, 008, 009, 010, 011, 013, 014, 015 · **QST** 001, 002, 003, 004, 005, 006, 007, 008, 009 · **RNK** 001, 002, 003, 004 · **RWD** 001, 002, 003 · **SIS** 001, 002, 003, 004, 005, 006, 007, 008, 009 · **SOC** 001, 002, 003, 004 · **TRN** 001, 002, 003
- **AC (PRD) chưa có TC nhắc mã:** **AUTH** 001, 002, 003, 004, 005, 006, 007, 008, 009, 010, 011, 012, 013, 014, 015, 016, 017, 018, 019, 020, 021, 022, 023, 024, 025, 026, 027, 028, 029, 030, 031, 032, 033, 034, 035, 036 · **CHAT** 001, 002, 003, 004, 005, 006, 007, 008, 009, 010, 011, 012, 013, 014, 015 · **CMP** 001, 002, 003, 004, 005, 006 · **CMS** 001, 002, 003, 004, 005 · **CNT** 001, 002, 003, 004, 005, 006, 007, 008 · **EXM** 001, 002 · **EXR** 001, 002, 003, 004, 005, 006, 007, 008 · **GAM** 001, 002, 003, 011 · **MOD** 020, 021, 022, 026, 027, 030 · **ORG** 001, 002, 003, 004, 005, 006, 007 · **PRG** 001, 002 · **PTY** 001, 002, 003, 004, 005, 006, 007, 008, 009, 010, 011, 012, 013
- **AC mở rộng chưa có TC nhắc mã:** **ACT** 001, 002, 003, 004, 005, 006, 007, 008, 009, 010, 011, 012, 013, 014, 015 · **ANL** 102, 109 · **ARP** 001, 002, 003, 004, 005, 006 · **AUD** 001, 002, 003, 004 · **AUTH** 101, 102, 103, 104, 105, 106, 107, 108, 109, 110, 111, 112, 113, 114, 115, 116, 117, 118, 120, 121, 122, 123, 124, 125, 127, 128, 130, 131, 140, 141, 142, 143, 144, 145, 150, 151, 152, 153, 154, 155, 156, 157, 158, 159, 160, 161, 162, 163, 164, 165, 166, 170, 171, 172, 173, 174 · **CHL** 001, 002, 003, 004, 005, 006, 007, 008 · **CHS** 001, 002, 003, 004, 005, 006, 007, 008, 009, 010, 011, 012, 013, 014, 015, 016, 017, 018, 019, 020, 021, 022, 023, 024, 025, 026, 027, 028, 029, 030, 031, 032, 033, 034, 035, 036, 037, 038, 039, 040, 041, 042, 043, 044, 045, 046 · **CMP** 101, 102, 103, 201, 202, 203, 204, 205, 206, 207, 208, 209, 210 · **CPP** 001, 002, 003, 004 · **DASH** 005, 008 · **ECO** 001, 002, 003, 004, 005, 006, 007, 008 · **EFF** 001, 002, 003, 004, 005 · **EXM** 003, 004, 005, 006, 007, 008, 009, 010 · **GAM** 004, 005, 006, 007, 008, 009, 010, 014, 015, 016 · **GDD** 001, 002, 003, 004, 005 · **LDB** 001, 002, 003, 004, 005, 006, 007 · **LMS** 001, 002, 003, 004, 005, 006, 007, 008, 010, 011, 012, 013, 014, 015, 016, 017, 018 · **MG** 001, 002, 003, 004, 005, 006, 101, 102, 103, 104 · **MM** 001, 002, 003, 004, 005, 006, 007, 008 · **MOD** 001, 002, 003, 004, 005, 006, 007, 008, 009, 010, 011, 012, 013, 014, 015, 016, 017, 018, 019, 023, 024, 025, 028, 029, 031 · **MP** 001, 002, 003, 004, 005, 006, 007, 008, 009, 010, 011 · **ORG** 008, 009, 010, 011, 012, 013, 014, 015, 016, 017, 018, 019, 020, 021, 022, 023, 024, 025, 026, 027, 028 · **PERF** 003, 008, 012 · **QST** 001, 002, 003, 004, 005, 006, 007, 008, 009, 010, 011, 012, 013, 014, 015 · **RNK** 001, 002, 003, 004, 005, 006 · **SIM** 001, 002, 003, 004, 005 · **SOC** 001, 002, 003, 004, 005, 006, 007, 008, 009, 010, 011 · **TRN** 001, 002, 003, 004, 005, 006, 007, 008 · **UX** 0001, 0002, 0003, 0004, 0005, 0006, 0007, 0008, 0101, 0102, 0103, 0104, 0105, 0106, 0107, 0108, 0109, 0110, 0111, 0112, 0113, 0114, 0115, 0116, 0117, 0118, 0119, 0201, 0202, 0203, 0204, 0301, 0302, 0303, 0304, 0401, 0402, 0403, 0404, 0501, 0502, 0503, 0504, 0601, 0602, 0603, 0604, 0605, 0701, 0702, 0703, 0704, 101, 102, 103, 104, 105, 106, 107, 108, 109, 110, 111, 112, 113, 114, 201, 202, 203, 204, 205, 206, 207, 208, 209, 210, 211, 212, 213, 214, 215, 216, 217 · **VID** 001, 002, 003, 004, 005 · **WLD** 001, 002, 003, 004 · **WR** 001, 002, 003, 004, 005, 006, 007, 008
