# Ma trận truy vết yêu cầu (RTM)

> Trạng thái: Draft · Owner: Product Manager + QA Lead · **File sinh tự động** bởi `scripts/docs/build-rtm.mjs`: không sửa tay; sửa nguồn rồi chạy `node scripts/docs/build-rtm.mjs` (CI chạy `--check`).
> Nguồn: [PRODUCT_REQUIREMENTS](../00-overview/PRODUCT_REQUIREMENTS.md) (FR/US/AC) · [TEST_STRATEGY](TEST_STRATEGY.md) và bảng `TC-*` trong tài liệu domain · `specs/openapi/eduworld-api.v1.yaml` · [AGENTS.md §6](../../AGENTS.md#6-domain--tài-liệu--service) (service sở hữu)

## 1. Cách đọc

- **Tường minh**: tài liệu/operation/TC có nhắc đúng mã FR (hoặc FR nhắc đúng path/operationId). **Suy luận**: ghép theo domain của mã (tài liệu sở hữu, mẫu path, tiền tố TC) — cần rà khi review, không coi là bằng chứng.
- FR có phase `—` (mở rộng sau, ngoài Release 1.0) vẫn liệt kê nhưng không tính vào báo cáo mồ côi.
- AC/US gắn với FR theo domain (PRD chưa ghi FR cho từng AC). Muốn liên kết tường minh: ghi mã FR/AC trong cột kịch bản của bảng TC, hoặc trong `description` của operation OpenAPI.

## 2. Tổng hợp theo domain

| Domain | Service sở hữu | FR | FR có tài liệu tường minh | FR có operation tường minh | Operation suy luận | FR có TC tường minh | TC suy luận | US | AC | FR / AC mở rộng ở tài liệu sở hữu |
|---|---|---|---|---|---|---|---|---|---|---|
| `AUTH` Identity & Authentication | identity-service | 32 | 0 | 8 | 10 | 0 | AUTH (59) | 3 | 26 | 0 / 52 |
| `ORG` Organization & Multi-tenancy | identity-service | 18 | 0 | 0 | 36 | 0 | ORG (9), TEN (24) | 0 | 7 | 0 / 17 |
| `SIS` SIS Integration | identity-service | 9 | 0 | 0 | 5 | 0 | SIS (7) | 2 | 0 | 0 / 0 |
| `CUR` Curriculum | learning-service | 6 | 6 | 0 | 6 | 0 | LRN (10) | 0 | 0 | 2 / 0 |
| `CNT` Learning Content | learning-service | 16 | 11 | 0 | 13 | 0 | CNT (9) | 0 | 8 | 2 / 0 |
| `QB` Question Bank | learning-service | 7 | 7 | 0 | 5 | 0 | LRN (10) | 0 | 0 | 4 / 0 |
| `EXR` Activity & Attempt Engine | learning-service | 11 | 7 | 1 | 36 | 0 | ACT (5), LRN (10), VID (5), WR (8), SIM (5), MATH (3) | 2 | 6 | 4 / 24 |
| `EXM` Exam & Practice | learning-service | 7 | 7 | 0 | 5 | 0 | EXM (6) | 1 | 2 | 2 / 3 |
| `PRG` Learning Progress & Mastery | learning-service | 5 | 4 | 0 | 8 | 0 | LRN (10) | 0 | 1 | 0 / 0 |
| `ANL` Learning Events & Analytics | learning-service · game-service | 14 | 4 | 0 | 18 | 0 | ANL (10), DASH (10) | 0 | 0 | 0 / 21 |
| `GAME` Game World & Character | game-service | 18 | 0 | 2 | 21 | 0 | WLD (6), GAM (12) | 2 | 0 | 0 / 9 |
| `QST` Quest | game-service | 8 | 0 | 0 | 19 | 0 | GAM (12) | 2 | 0 | 0 / 13 |
| `MG` Mini-game Engine | learning-service · game client | 4 | 0 | 0 | 1 | 0 | ACT (5) | 1 | 0 | 0 / 6 |
| `MP` Multiplayer & Presence | competition-service · realtime-gateway | 6 | 0 | 0 | 3 | 0 | PTY (14), CMP (10) | 0 | 0 | 0 / 11 |
| `PTY` Party | competition-service | 14 | 0 | 1 | 17 | 0 | PTY (14) | 4 | 11 | 0 / 0 |
| `MM` Matchmaking | competition-service | 6 | 0 | 0 | 3 | 0 | MM (4) | 1 | 0 | 0 / 8 |
| `CMP` Competition / Battle | competition-service | 5 | 0 | 0 | 6 | 0 | CMP (10) | 0 | 6 | 0 / 12 |
| `RNK` Ranking | competition-service | 4 | 0 | 0 | 1 | 0 | CMP (10) | 0 | 0 | 0 / 6 |
| `LDB` Leaderboard | competition-service | 4 | 0 | 0 | 2 | 0 | CMP (10) | 1 | 0 | 0 / 6 |
| `TRN` Tournament | competition-service | 3 | 0 | 0 | 1 | 0 | CMP (10) | 1 | 0 | 0 / 8 |
| `SOC` Social | chat-service | 4 | 0 | 0 | 35 | 0 | SOC (9) | 1 | 0 | 0 / 10 |
| `CHAT` Chat | chat-service | 17 | 0 | 0 | 15 | 0 | CHAT (41) | 2 | 14 | 0 / 29 |
| `MOD` Moderation & Child Safety | chat-service | 15 | 0 | 0 | 32 | 0 | CHAT (41) | 2 | 3 | 0 / 22 |
| `NTF` Notification | chat-service | 3 | 1 | 1 | 1 | 0 | SOC (9) | 0 | 0 | 0 / 0 |
| `GAM` XP, level, tiến trình | game-service | 9 | 0 | 0 | 5 | 0 | GAM (12) | 1 | 4 | 0 / 7 |
| `RWD` Phần thưởng | game-service | 3 | 0 | 0 | 10 | 0 | GAM (12), ECO (8) | 0 | 0 | 0 / 0 |
| `CHL` Challenge & Boss | game-service | 3 | 0 | 0 | 13 | 0 | CHL (8) | 0 | 0 | 0 / 8 |
| `ECO` Kinh tế, vật phẩm, sưu tầm | game-service | 5 | 0 | 0 | 11 | 0 | ECO (8) | 0 | 0 | 0 / 8 |
| `EVT` Event & Season (live-ops) | game-service | 8 | 0 | 0 | 18 | 0 | EVT (12) | 0 | 0 | 0 / 12 |
| `CMS` CMS | learning-service · game-service (game-admin) | 17 | 0 | 1 | 48 | 0 | CMS (4) | 2 | 5 | 0 / 0 |
| `LMS` LMS | learning-service | 7 | 7 | 0 | 21 | 0 | LMS (3), DASH (10) | 2 | 0 | 3 / 14 |
| `AUD` Audit | identity-service | 2 | 0 | 0 | 27 | 0 | AUD (5) | 0 | 0 | 0 / 5 |
| `OPS` Vận hành | pkg/ (platform kit) · mọi service | 5 | 0 | 0 | 12 | 0 | SVC (15), PERF (25) | 0 | 0 | 0 / 0 |
| **Tổng** | | **295** | 54 | 14 | 364 op trong spec | 0 | 371 TC | 30 | 93 | 17 / 393 |

## 3. Ma trận FR

### 3.1. `AUTH` Identity & Authentication

Service: identity-service · Tài liệu sở hữu: [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md)

AC: AC-AUTH-001, AC-AUTH-002, AC-AUTH-003, AC-AUTH-004, AC-AUTH-005, AC-AUTH-006, AC-AUTH-007, AC-AUTH-008, AC-AUTH-009, AC-AUTH-010, AC-AUTH-011, AC-AUTH-012, AC-AUTH-013, AC-AUTH-014, AC-AUTH-015, AC-AUTH-016, AC-AUTH-017, AC-AUTH-018, AC-AUTH-019, AC-AUTH-020, AC-AUTH-021, AC-AUTH-022, AC-AUTH-023, AC-AUTH-024, AC-AUTH-025, AC-AUTH-026 · US: US-AUTH-001, US-AUTH-002, US-AUTH-003

Mở rộng ở tài liệu sở hữu: 0 FR, 52 AC, 0 US — [ACCOUNT_PROVISIONING](../09-identity/ACCOUNT_PROVISIONING.md) · [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md)

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-AUTH-001 | Học sinh dùng tài khoản do nhà trường cấp; không có màn hình tự đăng ký trong game | P1 | _suy luận:_ [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) | _suy luận:_ 10 op | _suy luận:_ TC-AUTH |
| FR-AUTH-002 | Godot đăng nhập bằng form username/password trong game, gửi qua HTTPS tới POST /api/v1/auth/game/login; ide… | P1 | _suy luận:_ [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) | `gameLogin` | _suy luận:_ TC-AUTH |
| FR-AUTH-003 | EduWorld không lưu mật khẩu học sinh; xác thực ở Keycloak/IdP. Mật khẩu đi qua game và identity-service (AD… | P1 | _suy luận:_ [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) | _suy luận:_ 10 op | _suy luận:_ TC-AUTH |
| FR-AUTH-004 | Backend xác minh token (chữ ký JWKS, iss, aud=eduworld-api, exp) độc lập với APISIX (ADR-0008, ADR-0009) | P1 | _suy luận:_ [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) | _suy luận:_ 10 op | _suy luận:_ TC-AUTH |
| FR-AUTH-005 | Định danh ngoài bằng (issuer, subject); không dùng email/tên hiển thị làm khóa | P1 | _suy luận:_ [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) | _suy luận:_ 10 op | _suy luận:_ TC-AUTH |
| FR-AUTH-006 | Just-in-Time provisioning khi đăng nhập lần đầu qua IdP trường, chỉ khi khớp bản ghi SIS — ngoài Release 1.… | — | _suy luận:_ [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) | _suy luận:_ 10 op | _suy luận:_ TC-AUTH |
| FR-AUTH-007 | POST /api/v1/auth/game/login (game) / POST /api/v1/auth/session (portal) trả bootstrap: tenant_id của phiên… | P1 | _suy luận:_ [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) | `createAuthSession` · `gameLogin` | _suy luận:_ TC-AUTH |
| FR-AUTH-008 | Không tự đăng nhập khi mở lại game (ADR-0064): mỗi lần mở game nhập username + password vào form trong game… | P1 | _suy luận:_ [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) | _suy luận:_ 10 op | _suy luận:_ TC-AUTH |
| FR-AUTH-009 | Logout (và thoát game bình thường, ADR-0064): POST /api/v1/auth/game/logout — identity-service thu hồi refr… | P1 | _suy luận:_ [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) | `gameLogout` | _suy luận:_ TC-AUTH |
| FR-AUTH-010 | Tài khoản bị khóa/vô hiệu hóa: từ chối request REST, cắt kết nối realtime qua event user.access_revoked | P1 | _suy luận:_ [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) | _suy luận:_ 10 op | _suy luận:_ TC-AUTH |
| FR-AUTH-011 | Realtime ticket dùng 1 lần, TTL 60 giây để mở WebSocket /realtime/v1 (ADR-0008) | P1 | _suy luận:_ [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) | _suy luận:_ 10 op | _suy luận:_ TC-AUTH |
| FR-AUTH-012 | Cấu hình realm/client/issuer qua environment, không hardcode | P1 | _suy luận:_ [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) | _suy luận:_ 10 op | _suy luận:_ TC-AUTH |
| FR-AUTH-013 | Keycloak clients: eduworld-game-backend (confidential, chỉ Direct Access Grants, secret chỉ ở identity-serv… | P1 | _suy luận:_ [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) | _suy luận:_ 10 op | _suy luận:_ TC-AUTH |
| FR-AUTH-014 | Hỗ trợ nhiều issuer (multi-realm sau này) nhờ khóa (issuer, subject) | P1 | _suy luận:_ [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) | _suy luận:_ 10 op | _suy luận:_ TC-AUTH |
| FR-AUTH-015 | Device Authorization Grant cho máy dùng chung trong lớp — ngoài Release 1.0 (ADR-0063) | — | _suy luận:_ [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) | _suy luận:_ 10 op | _suy luận:_ TC-AUTH |
| FR-AUTH-016 | Authorization theo Permission (trong code) → Role (DB, identity.roles) → UserRole (DB, identity.user_roles:… | P1 | _suy luận:_ [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) | _suy luận:_ 10 op | _suy luận:_ TC-AUTH |
| FR-AUTH-017 | Đăng nhập username + password, không chọn trường (ADR-0063, ADR-0068): form đăng nhập vẽ trong game gồm log… | P1 | _suy luận:_ [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) | `selectGameLoginTenant` | _suy luận:_ TC-AUTH |
| FR-AUTH-018 | Backend xác định và kiểm tra chéo tenant khi tạo phiên và ở mọi request theo membership trong DB. Mọi reque… | P1 | _suy luận:_ [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) | `createAuthSession` · `gameLogin` · `getClientConfig` · `listMyTenants` · `selectGameLoginTenant` | _suy luận:_ TC-AUTH |
| FR-AUTH-019 | Mỗi phiên có đúng một tenant; người thuộc nhiều tenant đổi tenant bằng token mới + header X-Tenant-Id mới (… | P1 | _suy luận:_ [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) | _suy luận:_ 10 op | _suy luận:_ TC-AUTH |
| FR-AUTH-020 | Trang đăng nhập Keycloak hiển thị tên/logo và IdP của tenant được gợi ý trước đăng nhập — ngoài Release 1.0… | — | _suy luận:_ [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) | _suy luận:_ 10 op | _suy luận:_ TC-AUTH |
| FR-AUTH-021 | Tenant chung: tài khoản học sinh trong tenant chung common chỉ do platform (System Admin) tạo/import có chủ… | P1 | _suy luận:_ [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) | _suy luận:_ 10 op | _suy luận:_ TC-AUTH |
| FR-AUTH-022 | Chỉ học sinh được đăng nhập game: đăng nhập qua /auth/game/login (client eduworld-game-backend) của giáo vi… | P1 | _suy luận:_ [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) | `gameLogin` | _suy luận:_ TC-AUTH |
| FR-AUTH-023 | Một thiết bị cho mỗi tài khoản học sinh: tối đa 1 game session active; đăng nhập thiết bị khác → hộp thoại… | P1 | _suy luận:_ [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) | _suy luận:_ 10 op | _suy luận:_ TC-AUTH |
| FR-AUTH-024 | Dọn học sinh không đăng nhập game: worker soft-delete học sinh có last_login_at (hoặc provisioned_at nếu ch… | P2 | _suy luận:_ [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) | _suy luận:_ 10 op | _suy luận:_ TC-AUTH |
| FR-AUTH-025 | Ngoài Release 1.0 (ADR-0063) — đăng nhập nhanh không cần mật khẩu: mã QR lớp + PIN 4 số, thẻ QR cá nhân, mậ… | — | _suy luận:_ [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) | _suy luận:_ 10 op | _suy luận:_ TC-AUTH |
| FR-AUTH-026 | ~~Phiên game dài trên thiết bị (SSO idle 7 ngày / tối đa 30 ngày, refresh token lưu trên thiết bị)~~ — bỏ k… | — | _suy luận:_ [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) | _suy luận:_ 10 op | _suy luận:_ TC-AUTH |
| FR-AUTH-027 | Hàng chờ đăng nhập: khi vượt ngưỡng toàn cục, POST /auth/game/login trả 429 LOGIN_QUEUE_BUSY + Retry-After,… | P4 | _suy luận:_ [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) | `gameLogin` | _suy luận:_ TC-AUTH |
| FR-AUTH-028 | Bảo vệ đăng nhập game qua backend (ADR-0068): rate limit 10 lần/phút theo thiết bị và 10 lần/phút theo user… | P1 | _suy luận:_ [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) | _suy luận:_ 10 op | _suy luận:_ TC-AUTH |
| FR-AUTH-029 | Admin Portal có màn đăng nhập riêng (theme vận hành eduworld-ops, username hoặc email + mật khẩu, MFA với r… | P1 | _suy luận:_ [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) | _suy luận:_ 10 op | _suy luận:_ TC-AUTH |
| FR-AUTH-030 | Portal không chọn tenant trước đăng nhập (ADR-0067): role platform → phạm vi platform; 1 tenant → vào thẳng… | P1 | _suy luận:_ [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) | `listMyTenants` | _suy luận:_ TC-AUTH |
| FR-AUTH-031 | Username và email không trùng trong cùng tenant; trên SSO một người một tài khoản (username/email duy nhất… | P1 | _suy luận:_ [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) | _suy luận:_ 10 op | _suy luận:_ TC-AUTH |
| FR-AUTH-032 | Quản lý Role/UserRole trên portal (ADR-0066): Tenant Admin quản lý role tùy biến [Proposed] và UserRole tro… | P3 | _suy luận:_ [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) | _suy luận:_ 10 op | _suy luận:_ TC-AUTH |

### 3.2. `ORG` Organization & Multi-tenancy

Service: identity-service · Tài liệu sở hữu: [MULTITENANCY](../06-architecture/MULTITENANCY.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md)

AC: AC-ORG-001, AC-ORG-002, AC-ORG-003, AC-ORG-004, AC-ORG-005, AC-ORG-006, AC-ORG-007 · US: —

Mở rộng ở tài liệu sở hữu: 0 FR, 17 AC, 0 US — [MULTITENANCY](../06-architecture/MULTITENANCY.md)

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-ORG-001 | Phân cấp Platform → Tenant → School → Class. Tenant là một trường độc lập (school), một đơn vị nhiều trường… | P1 | _suy luận:_ [MULTITENANCY](../06-architecture/MULTITENANCY.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) | _suy luận:_ 36 op | _suy luận:_ TC-ORG, TC-TEN |
| FR-ORG-002 | Quản lý AcademicYear, Class (grade_level 1–9), Enrollment có lịch sử | P1 | _suy luận:_ [MULTITENANCY](../06-architecture/MULTITENANCY.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) | _suy luận:_ 36 op | _suy luận:_ TC-ORG, TC-TEN |
| FR-ORG-003 | Phân biệt Keycloak User, EduWorld User, Student, Teacher, School, Class, Enrollment, External Identity | P1 | _suy luận:_ [MULTITENANCY](../06-architecture/MULTITENANCY.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) | _suy luận:_ 36 op | _suy luận:_ TC-ORG, TC-TEN |
| FR-ORG-004 | TeachingAssignment giới hạn giáo viên vào lớp × môn được phân công | P1 | _suy luận:_ [MULTITENANCY](../06-architecture/MULTITENANCY.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) | _suy luận:_ 36 op | _suy luận:_ TC-ORG, TC-TEN |
| FR-ORG-005 | Chuyển lớp, chuyển trường: đóng Enrollment cũ, mở mới, giữ lịch sử học tập gắn trường cũ theo policy | P1 | _suy luận:_ [MULTITENANCY](../06-architecture/MULTITENANCY.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) | _suy luận:_ 36 op | _suy luận:_ TC-ORG, TC-TEN |
| FR-ORG-006 | Mọi bảng nghiệp vụ có tenant_id; mọi truy vấn phải lọc theo tenant_id, enforcement ở app layer (TenantScope… | P1 | _suy luận:_ [MULTITENANCY](../06-architecture/MULTITENANCY.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) | _suy luận:_ 36 op | _suy luận:_ TC-ORG, TC-TEN |
| FR-ORG-007 | School settings: chính sách chat, giờ truy cập, cho phép liên trường, JIT provisioning (/api/v1/schools) | P1–P5 | _suy luận:_ [MULTITENANCY](../06-architecture/MULTITENANCY.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) | _suy luận:_ 36 op | _suy luận:_ TC-ORG, TC-TEN |
| FR-ORG-008 | Tại một thời điểm người dùng chỉ làm việc trên một tenant. Chỉ System Admin thấy mọi tenant; mỗi màn hình c… | P1 (API) / P3 (CMS) | _suy luận:_ [MULTITENANCY](../06-architecture/MULTITENANCY.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) | _suy luận:_ 36 op | _suy luận:_ TC-ORG, TC-TEN |
| FR-ORG-009 | Mỗi Tenant là một Keycloak Organization (platform.tenants.keycloak_org_id, keycloak_org_alias, login_code);… | P1 | _suy luận:_ [MULTITENANCY](../06-architecture/MULTITENANCY.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) | _suy luận:_ 36 op | _suy luận:_ TC-ORG, TC-TEN |
| FR-ORG-010 | Membership Keycloak Organization đồng bộ tự động: học sinh thuộc đúng một tenant; giáo viên thuộc các tenan… | P1 | _suy luận:_ [MULTITENANCY](../06-architecture/MULTITENANCY.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) | _suy luận:_ 36 op | _suy luận:_ TC-ORG, TC-TEN |
| FR-ORG-011 | Chỉ soft-delete: ứng dụng không hard-delete; "xóa" đặt deleted_at/deleted_by; mọi truy vấn mặc định không t… | P1 | _suy luận:_ [MULTITENANCY](../06-architecture/MULTITENANCY.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) | _suy luận:_ 36 op | _suy luận:_ TC-ORG, TC-TEN |
| FR-ORG-012 | System Admin tạo dữ liệu dùng chung (platform) và gán cho các tenant muốn sử dụng; tenant được gán chỉ đọc… | P1 (model) / P3 (CMS) | _suy luận:_ [MULTITENANCY](../06-architecture/MULTITENANCY.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) | _suy luận:_ 36 op | _suy luận:_ TC-ORG, TC-TEN |
| FR-ORG-013 | System Admin clone dữ liệu (nội dung, cấu hình) từ tenant này sang tenant khác; bản sao có ID mới và nguồn… | P3 | _suy luận:_ [MULTITENANCY](../06-architecture/MULTITENANCY.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) | _suy luận:_ 36 op | _suy luận:_ TC-ORG, TC-TEN |
| FR-ORG-014 | Nghĩa vụ xóa dữ liệu theo luật được đáp ứng bằng ẩn danh hóa; Release 1.0 không xóa vật lý (ngoại lệ duy nh… | P5 | _suy luận:_ [MULTITENANCY](../06-architecture/MULTITENANCY.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) | _suy luận:_ 36 op | _suy luận:_ TC-ORG, TC-TEN |
| FR-ORG-015 | Danh mục quốc gia (ISO 3166-1) và danh mục tỉnh/thành theo quốc gia, có hiệu lực và kế thừa đơn vị cũ → mới… | P1 | _suy luận:_ [MULTITENANCY](../06-architecture/MULTITENANCY.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) | _suy luận:_ 36 op | _suy luận:_ TC-ORG, TC-TEN |
| FR-ORG-016 | Danh mục trường: mỗi trường thuộc một tenant và một tỉnh/thành, có mã ngành (nếu có), cấp học, khối áp dụng… | P1 | _suy luận:_ [MULTITENANCY](../06-architecture/MULTITENANCY.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) | _suy luận:_ 36 op | _suy luận:_ TC-ORG, TC-TEN |
| FR-ORG-017 | Học sinh thuộc đúng một trường và một lớp tại một thời điểm (một enrollment active); học sinh SIS thiếu lớp… | P1 | _suy luận:_ [MULTITENANCY](../06-architecture/MULTITENANCY.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) | _suy luận:_ 36 op | _suy luận:_ TC-ORG, TC-TEN |
| FR-ORG-018 | Đồng bộ SIS khi đăng nhập: dữ liệu đủ & mới → không gọi SIS; thiếu trường/lớp → tra cứu SIS chặn ≤ 3 s; dữ… | P1 | _suy luận:_ [MULTITENANCY](../06-architecture/MULTITENANCY.md) · [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) | _suy luận:_ 36 op | _suy luận:_ TC-ORG, TC-TEN |

### 3.3. `SIS` SIS Integration

Service: identity-service · Tài liệu sở hữu: [SIS_INTEGRATION](../09-identity/SIS_INTEGRATION.md) · [ACCOUNT_PROVISIONING](../09-identity/ACCOUNT_PROVISIONING.md)

AC: — · US: US-SIS-001, US-SIS-002

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-SIS-001 | EduWorld sở hữu DTO/API contract SIS (EduWorld SIS Contract v1); mỗi trường cung cấp API theo contract; khô… | P1 | _suy luận:_ [SIS_INTEGRATION](../09-identity/SIS_INTEGRATION.md) · [ACCOUNT_PROVISIONING](../09-identity/ACCOUNT_PROVISIONING.md) | _suy luận:_ 5 op | _suy luận:_ TC-SIS |
| FR-SIS-002 | Đồng bộ School, Class, Student, Teacher, Enrollment, Account Status | P1 | _suy luận:_ [SIS_INTEGRATION](../09-identity/SIS_INTEGRATION.md) · [ACCOUNT_PROVISIONING](../09-identity/ACCOUNT_PROVISIONING.md) | _suy luận:_ 5 op | _suy luận:_ TC-SIS |
| FR-SIS-003 | Hai cách nhận dữ liệu: contract_v1 (pull HTTP theo contract) và csv_import (cột CSV = tên trường DTO) cho t… | P1 | _suy luận:_ [SIS_INTEGRATION](../09-identity/SIS_INTEGRATION.md) · [ACCOUNT_PROVISIONING](../09-identity/ACCOUNT_PROVISIONING.md) | _suy luận:_ 5 op | _suy luận:_ TC-SIS |
| FR-SIS-009 | Bộ conformance test + mock server sinh từ contract để trường tự kiểm tra facade trước khi kết nối | P1 | _suy luận:_ [SIS_INTEGRATION](../09-identity/SIS_INTEGRATION.md) · [ACCOUNT_PROVISIONING](../09-identity/ACCOUNT_PROVISIONING.md) | _suy luận:_ 5 op | _suy luận:_ TC-SIS |
| FR-SIS-004 | Full sync định kỳ + delta sync + webhook (khi trường khai báo capability delta/webhook trong contract) | P1–P3 | _suy luận:_ [SIS_INTEGRATION](../09-identity/SIS_INTEGRATION.md) · [ACCOUNT_PROVISIONING](../09-identity/ACCOUNT_PROVISIONING.md) | _suy luận:_ 5 op | _suy luận:_ TC-SIS |
| FR-SIS-005 | Reconciliation: phát hiện bản ghi trùng/thiếu/xung đột, tạo ReconciliationIssue cho người duyệt | P1 | _suy luận:_ [SIS_INTEGRATION](../09-identity/SIS_INTEGRATION.md) · [ACCOUNT_PROVISIONING](../09-identity/ACCOUNT_PROVISIONING.md) | _suy luận:_ 5 op | _suy luận:_ TC-SIS |
| FR-SIS-006 | Sync error handling, retry có backoff, audit mọi lần chạy (SyncRun) | P1 | _suy luận:_ [SIS_INTEGRATION](../09-identity/SIS_INTEGRATION.md) · [ACCOUNT_PROVISIONING](../09-identity/ACCOUNT_PROVISIONING.md) | _suy luận:_ 5 op | _suy luận:_ TC-SIS |
| FR-SIS-007 | Godot không gọi SIS; SIS credential không lộ qua public gateway (/api/v1/sis chỉ gateway nội bộ) | P1 | _suy luận:_ [SIS_INTEGRATION](../09-identity/SIS_INTEGRATION.md) · [ACCOUNT_PROVISIONING](../09-identity/ACCOUNT_PROVISIONING.md) | _suy luận:_ 5 op | _suy luận:_ TC-SIS |
| FR-SIS-008 | Account deactivation/lock từ SIS lan truyền tới EduWorld trong ≤ 1 chu kỳ sync | P1 | _suy luận:_ [SIS_INTEGRATION](../09-identity/SIS_INTEGRATION.md) · [ACCOUNT_PROVISIONING](../09-identity/ACCOUNT_PROVISIONING.md) | _suy luận:_ 5 op | _suy luận:_ TC-SIS |

### 3.4. `CUR` Curriculum

Service: learning-service · Tài liệu sở hữu: [CURRICULUM_MODEL](../02-learning/CURRICULUM_MODEL.md)

Mở rộng ở tài liệu sở hữu: 2 FR, 0 AC, 0 US — [CURRICULUM_MODEL](../02-learning/CURRICULUM_MODEL.md)

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-CUR-001 | Cấu trúc Curriculum → Grade → Subject → Topic → Learning Objective → Learning Unit → Lesson → Learning Acti… | P2 | [CURRICULUM_MODEL](../02-learning/CURRICULUM_MODEL.md) | _suy luận:_ 6 op | _suy luận:_ TC-LRN |
| FR-CUR-002 | Hỗ trợ nhiều chương trình và phiên bản chương trình; không hardcode một chương trình | P2 | [CURRICULUM_MODEL](../02-learning/CURRICULUM_MODEL.md) | _suy luận:_ 6 op | _suy luận:_ TC-LRN |
| FR-CUR-003 | Learning Objective có mã ổn định (vd. MATH.G3.NUM.MUL_TABLE) | P2 | [CURRICULUM_MODEL](../02-learning/CURRICULUM_MODEL.md) | _suy luận:_ 6 op | _suy luận:_ TC-LRN |
| FR-CUR-004 | Seed khung GDPT 2018 môn Toán lớp 1–5 cho pilot | P2 | [CURRICULUM_MODEL](../02-learning/CURRICULUM_MODEL.md) | _suy luận:_ 6 op | _suy luận:_ TC-LRN |
| FR-CUR-005 | CMS quản lý cấu trúc chương trình (CRUD + workflow) | P3 | [CURRICULUM_MODEL](../02-learning/CURRICULUM_MODEL.md) | _suy luận:_ 6 op | _suy luận:_ TC-LRN |
| FR-CUR-006 | API duyệt curriculum cho client: /api/v1/learning/curriculum | P2 | [CURRICULUM_MODEL](../02-learning/CURRICULUM_MODEL.md) | _suy luận:_ 6 op | _suy luận:_ TC-LRN |

### 3.5. `CNT` Learning Content

Service: learning-service · Tài liệu sở hữu: [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md)

AC: AC-CNT-001, AC-CNT-002, AC-CNT-003, AC-CNT-004, AC-CNT-005, AC-CNT-006, AC-CNT-007, AC-CNT-008 · US: —

Mở rộng ở tài liệu sở hữu: 2 FR, 0 AC, 0 US — [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md)

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-CNT-001 | Loại nội dung: Lesson, Learning Material, Video, Image, Audio, Interactive Content, Question, Exercise, Que… | P2–P3 | [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md) | _suy luận:_ 13 op | _suy luận:_ TC-CNT |
| FR-CNT-002 | Metadata bắt buộc: grade, subject, topic, LO, difficulty, version, status, author, reviewer, published_at | P2 | [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md) | _suy luận:_ 13 op | _suy luận:_ TC-CNT |
| FR-CNT-003 | Trạng thái Draft → Review → Approved → Published → Archived (+ Rejected → Draft) | P2 | [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md) | _suy luận:_ 13 op | _suy luận:_ TC-CNT |
| FR-CNT-004 | Published version bất biến; attempt tham chiếu content_version_id (ADR-0015) | P2 | [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md) | _suy luận:_ 13 op | _suy luận:_ TC-CNT |
| FR-CNT-005 | Nội dung thuộc platform (tenant_id NULL, tenant thấy khi được gán) hoặc thuộc một tenant; chia sẻ giữa tena… | P2 | [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md) | _suy luận:_ 13 op | _suy luận:_ TC-CNT |
| FR-CNT-006 | Media lưu object storage S3-compatible + CDN (không lưu file trong DB); pipeline quét virus, kiểm tra magic… | P2 | [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md) | _suy luận:_ 13 op | _suy luận:_ TC-CNT |
| FR-CNT-007 | Import nội dung hàng loạt (JSON/CSV theo specs/schemas) cho seed pilot trước khi có CMS UI | P2 | [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md) | _suy luận:_ 13 op | _suy luận:_ TC-CNT |
| FR-CNT-009 | Draft/Publish cho mọi nội dung có version; release pointer; publish ngay hoặc hẹn giờ; rollback = dùng lại… | P2 | [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md) | _suy luận:_ 13 op | _suy luận:_ TC-CNT |
| FR-CNT-010 | Xem nội dung từng version đã publish (chỉ đọc), so sánh hai version, preview trong game | P2–P3 | [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md) | _suy luận:_ 13 op | _suy luận:_ TC-CNT |
| FR-CNT-011 | Version pinning: học sinh đang ở màn/quest/bài/attempt/trận giữ version cũ; lượt/màn mới nhận version mới n… | P2 | [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md) | _suy luận:_ 13 op | _suy luận:_ TC-CNT |
| FR-CNT-012 | Publish đẩy snapshot bất biến lên Valkey/CDN; content manifest theo tenant (ETag/304); câu hỏi đề và answer… | P2 | [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md) | _suy luận:_ 13 op | _suy luận:_ TC-CNT |
| FR-CNT-013 | Activity Registry: mọi loại hoạt động học tập (quiz, interactive_exercise, written_response, video_lesson,… | P2 | _suy luận:_ [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md) | _suy luận:_ 13 op | _suy luận:_ TC-CNT |
| FR-CNT-014 | Video bài học: chapter, phụ đề WebVTT vi/en, resume, câu hỏi chèn tại mốc thời gian, nhánh tương tác, hoàn… | P4 | _suy luận:_ [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md) | _suy luận:_ 13 op | _suy luận:_ TC-CNT |
| FR-CNT-015 | Bài nghe và nói: listening có câu hỏi; bài nói: học sinh ghi âm trong game, giáo viên nghe và chấm tay theo… | P4 | _suy luận:_ [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md) | _suy luận:_ 13 op | _suy luận:_ TC-CNT |
| FR-CNT-016 | Mô phỏng: template Godot đóng gói, CMS chỉ tham số hóa, server kiểm chứng xác định bằng checker Go (SIMULAT… | P4 | _suy luận:_ [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md) | _suy luận:_ 13 op | _suy luận:_ TC-CNT |
| FR-CNT-017 | Công thức Toán/Lý/Hóa: soạn bằng MathLive, hiển thị MathJax 4 trên portal, SVG render sẵn trong game; game… | P2 | _suy luận:_ [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md) | _suy luận:_ 13 op | _suy luận:_ TC-CNT |

### 3.6. `QB` Question Bank

Service: learning-service · Tài liệu sở hữu: [QUESTION_BANK](../02-learning/QUESTION_BANK.md)

Mở rộng ở tài liệu sở hữu: 4 FR, 0 AC, 0 US — [QUESTION_BANK](../02-learning/QUESTION_BANK.md)

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-QB-001 | 11 loại: Single Choice, Multiple Choice, True/False, Fill in the Blank, Short Answer, Numeric Answer, Match… | P2 (6 loại đầu) – P3 (đủ) | [QUESTION_BANK](../02-learning/QUESTION_BANK.md) | _suy luận:_ 5 op | _suy luận:_ TC-LRN |
| FR-QB-002 | Thuộc tính: ID, stem, render data, options, answer key, explanation, difficulty, grade, subject, topic, LO,… | P2 | [QUESTION_BANK](../02-learning/QUESTION_BANK.md) | _suy luận:_ 5 op | _suy luận:_ TC-LRN |
| FR-QB-003 | Answer key và scoring rule không bao giờ gửi về client (ADR-0016) | P2 | [QUESTION_BANK](../02-learning/QUESTION_BANK.md) | _suy luận:_ 5 op | _suy luận:_ TC-LRN |
| FR-QB-004 | Random câu hỏi theo điều kiện (grade/topic/LO/difficulty/loại trừ câu đã làm gần đây) | P2 | [QUESTION_BANK](../02-learning/QUESTION_BANK.md) | _suy luận:_ 5 op | _suy luận:_ TC-LRN |
| FR-QB-005 | Tạo đề từ Question Bank theo Assessment Blueprint | P3 | [QUESTION_BANK](../02-learning/QUESTION_BANK.md) | _suy luận:_ 5 op | _suy luận:_ TC-LRN |
| FR-QB-006 | Chấm Short Answer bằng so khớp chuẩn hóa (bỏ dấu cách thừa, không phân biệt hoa thường, danh sách đáp án ch… | P2 | [QUESTION_BANK](../02-learning/QUESTION_BANK.md) | _suy luận:_ 5 op | _suy luận:_ TC-LRN |
| FR-QB-007 | Nội dung câu hỏi hỗ trợ tiếng Việt có dấu, công thức toán đơn giản, hình ảnh, audio (môn Tiếng Anh) | P2 | [QUESTION_BANK](../02-learning/QUESTION_BANK.md) | _suy luận:_ 5 op | _suy luận:_ TC-LRN |

### 3.7. `EXR` Activity & Attempt Engine

Service: learning-service · Tài liệu sở hữu: [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md)

AC: AC-EXR-001, AC-EXR-002, AC-EXR-003, AC-EXR-004, AC-EXR-005, AC-EXR-006 · US: US-EXR-001, US-EXR-002

Mở rộng ở tài liệu sở hữu: 4 FR, 24 AC, 0 US — [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md) · [SIMULATION_ENGINE](../02-learning/SIMULATION_ENGINE.md) · [VIDEO_LEARNING](../02-learning/VIDEO_LEARNING.md) · [WRITTEN_RESPONSE_RUBRIC](../02-learning/WRITTEN_RESPONSE_RUBRIC.md)

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-EXR-001 | Chế độ: bài cá nhân, theo quest, theo chủ đề, theo lớp, giáo viên giao, tự luyện, làm lại câu sai | P2 (giáo viên giao: P3) | [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md) | _suy luận:_ 36 op | _suy luận:_ TC-ACT, TC-LRN, TC-VID, TC-WR, TC-SIM, TC-MATH |
| FR-EXR-002 | Lưu Attempt, Answer, Correctness, Score, Duration, Submission Time, Content Version, LO | P2 | [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md) | _suy luận:_ 36 op | _suy luận:_ TC-ACT, TC-LRN, TC-VID, TC-WR, TC-SIM, TC-MATH |
| FR-EXR-003 | Practice: phản hồi đúng/sai + giải thích ngay sau từng câu | P2 | [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md) | _suy luận:_ 36 op | _suy luận:_ TC-ACT, TC-LRN, TC-VID, TC-WR, TC-SIM, TC-MATH |
| FR-EXR-004 | Resume attempt chưa hoàn thành theo chính sách (TTL, số lần) | P2 | [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md) | _suy luận:_ 36 op | _suy luận:_ TC-ACT, TC-LRN, TC-VID, TC-WR, TC-SIM, TC-MATH |
| FR-EXR-005 | Submit idempotent (Idempotency-Key), một attempt chỉ submit một lần | P2 | [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md) | _suy luận:_ 36 op | _suy luận:_ TC-ACT, TC-LRN, TC-VID, TC-WR, TC-SIM, TC-MATH |
| FR-EXR-006 | Thời gian tính theo server timestamp; client elapsed chỉ tham khảo | P2 | [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md) | _suy luận:_ 36 op | _suy luận:_ TC-ACT, TC-LRN, TC-VID, TC-WR, TC-SIM, TC-MATH |
| FR-EXR-007 | Một họ API attempt /api/v1/learning/activity-attempts cho mọi loại hoạt động (start, responses, progress, s… | P2 | [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md) | `listMyActivityAttempts` · `startAdHocAttempt` | _suy luận:_ TC-ACT, TC-LRN, TC-VID, TC-WR, TC-SIM, TC-MATH |
| FR-EXR-008 | Evaluation tách khỏi Attempt: nhiều evaluation/attempt (system, teacher, AI gợi ý), đúng một evaluation có… | P3 | _suy luận:_ [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md) | _suy luận:_ 36 op | _suy luận:_ TC-ACT, TC-LRN, TC-VID, TC-WR, TC-SIM, TC-MATH |
| FR-EXR-009 | Tự luận/trả lời dài/lời giải toán (văn bản + đáp số) chấm bằng rubric và hàng đợi chấm của giáo viên; trả b… | P3 | _suy luận:_ [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md) | _suy luận:_ 36 op | _suy luận:_ TC-ACT, TC-LRN, TC-VID, TC-WR, TC-SIM, TC-MATH |
| FR-EXR-010 | Dự án nhiều bước, cá nhân/nhóm; mỗi học sinh có attempt và learning record riêng | P4 | _suy luận:_ [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md) | _suy luận:_ 36 op | _suy luận:_ TC-ACT, TC-LRN, TC-VID, TC-WR, TC-SIM, TC-MATH |
| FR-EXR-011 | AI hỗ trợ chấm chỉ là gợi ý, tắt mặc định, giáo viên phải duyệt (ADR-0053) | Sau 1.0 | _suy luận:_ [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md) | _suy luận:_ 36 op | _suy luận:_ TC-ACT, TC-LRN, TC-VID, TC-WR, TC-SIM, TC-MATH |

### 3.8. `EXM` Exam & Practice

Service: learning-service · Tài liệu sở hữu: [EXAM_ENGINE](../02-learning/EXAM_ENGINE.md)

AC: AC-EXM-001, AC-EXM-002 · US: US-EXM-001

Mở rộng ở tài liệu sở hữu: 2 FR, 3 AC, 0 US — [EXAM_ENGINE](../02-learning/EXAM_ENGINE.md)

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-EXM-001 | Hình thức: Practice Test, Mock Exam, Chapter Test, Semester Test, Subject Test, Custom Assessment, Timed Exam | P3 | [EXAM_ENGINE](../02-learning/EXAM_ENGINE.md) | _suy luận:_ 5 op | _suy luận:_ TC-EXM |
| FR-EXM-002 | Tạo đề, random đề, random câu, xáo trộn đáp án, cấu hình thời gian & thang điểm | P3 | [EXAM_ENGINE](../02-learning/EXAM_ENGINE.md) | _suy luận:_ 5 op | _suy luận:_ TC-EXM |
| FR-EXM-003 | Nộp bài, chấm điểm, hiển thị kết quả theo result_visibility, phân tích câu sai, đề xuất nội dung ôn tập | P3 | [EXAM_ENGINE](../02-learning/EXAM_ENGINE.md) | _suy luận:_ 5 op | _suy luận:_ TC-EXM |
| FR-EXM-004 | Phân biệt luyện tập và kiểm tra chính thức (is_official) | P3 | [EXAM_ENGINE](../02-learning/EXAM_ENGINE.md) | _suy luận:_ 5 op | _suy luận:_ TC-EXM |
| FR-EXM-005 | Chống nộp trùng, chống sửa điểm từ client, tự nộp khi hết deadline_at | P3 | [EXAM_ENGINE](../02-learning/EXAM_ENGINE.md) | _suy luận:_ 5 op | _suy luận:_ TC-EXM |
| FR-EXM-006 | Kết quả thi lưu bền vững trong PostgreSQL, không phụ thuộc Valkey | P3 | [EXAM_ENGINE](../02-learning/EXAM_ENGINE.md) | _suy luận:_ 5 op | _suy luận:_ TC-EXM |
| FR-EXM-007 | Thi chính thức có giá trị pháp lý/proctoring | Chỉ khi có quyết định | [EXAM_ENGINE](../02-learning/EXAM_ENGINE.md) | _suy luận:_ 5 op | _suy luận:_ TC-EXM |

### 3.9. `PRG` Learning Progress & Mastery

Service: learning-service · Tài liệu sở hữu: [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md)

AC: AC-PRG-001 · US: —

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-PRG-001 | Theo dõi tiến độ, số bài hoàn thành, tỷ lệ chính xác, thời gian học, số lần làm, LO mastery, kỹ năng yếu, t… | P2–P3 | [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md) | _suy luận:_ 8 op | _suy luận:_ TC-LRN |
| FR-PRG-002 | Mastery theo (student, LO) tính từ nhiều bằng chứng, không đồng nhất với điểm một lần làm bài | P2 | [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md) | _suy luận:_ 8 op | _suy luận:_ TC-LRN |
| FR-PRG-003 | Mô hình mastery cấu hình được, mặc định Weighted Evidence (ADR-0017), lưu model_version | P2 | [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md) | _suy luận:_ 8 op | _suy luận:_ TC-LRN |
| FR-PRG-004 | Gợi ý ôn tập dựa trên LO yếu (/api/v1/learning/recommendations) | P3 | [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md) | _suy luận:_ 8 op | _suy luận:_ TC-LRN |
| FR-PRG-005 | Học sinh xem tiến độ dạng thân thiện theo độ tuổi (sao/huy hiệu cho lớp 1–5, biểu đồ cho 6–9) | P3 | _suy luận:_ [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md) | _suy luận:_ 8 op | _suy luận:_ TC-LRN |

### 3.10. `ANL` Learning Events & Analytics

Service: learning-service · game-service · Tài liệu sở hữu: [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md) · [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md) · [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md)

Mở rộng ở tài liệu sở hữu: 0 FR, 21 AC, 0 US — [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) · [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md)

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-ANL-001 | Ghi learning events (activity/question/exercise/exam/lesson/quest/learning_objective.*) với event_id, stude… | P2 | [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md) | _suy luận:_ 18 op | _suy luận:_ TC-ANL, TC-DASH |
| FR-ANL-002 | Lưu bền vững trong learning.learning_events (LRS nội bộ), tương thích tư duy xAPI | P2 | [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md) | _suy luận:_ 18 op | _suy luận:_ TC-ANL, TC-DASH |
| FR-ANL-003 | Analytics theo học sinh, giáo viên, lớp, trường, môn, chủ đề, LO | P3 | [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md) | _suy luận:_ 18 op | _suy luận:_ TC-ANL, TC-DASH |
| FR-ANL-004 | Không dùng dữ liệu hành vi ngoài mục đích đã công bố | P1+ | [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md) | _suy luận:_ 18 op | _suy luận:_ TC-ANL, TC-DASH |
| FR-ANL-005 | Aggregation chạy trong worker, dashboard đọc từ read model | P3 | _suy luận:_ [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md) · [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md) · [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) | _suy luận:_ 18 op | _suy luận:_ TC-ANL, TC-DASH |
| FR-ANL-006 | Game Analytics tách khỏi Learning Analytics và Monitoring: chỉ số engagement, retention, funnel, tiến trình… | P4 | _suy luận:_ [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md) · [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md) · [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) | _suy luận:_ 18 op | _suy luận:_ TC-ANL, TC-DASH |
| FR-ANL-007 | Nguồn: domain event NATS + telemetry client tối thiểu không PII; lưu schema analytics của game-service (ADR… | P4 | _suy luận:_ [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md) · [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md) · [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) | _suy luận:_ 18 op | _suy luận:_ TC-ANL, TC-DASH |
| FR-ANL-008 | Định danh giả danh player_key; không lưu tên, nickname, student_id, nội dung chat trong analytics | P4 | _suy luận:_ [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md) · [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md) · [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) | _suy luận:_ 18 op | _suy luận:_ TC-ANL, TC-DASH |
| FR-ANL-009 | Dashboard Game Analytics trong workspace Game Live Operations (platform); báo cáo tổng hợp cho Tenant/Schoo… | P4 | _suy luận:_ [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md) · [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md) · [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) | _suy luận:_ 18 op | _suy luận:_ TC-ANL, TC-DASH |
| FR-ANL-010 | Không dùng thời gian chơi làm thước đo thành công; có chỉ số chơi lành mạnh (vượt mốc nhắc nghỉ, chơi khuya) | P4 | _suy luận:_ [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md) · [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md) · [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) | _suy luận:_ 18 op | _suy luận:_ TC-ANL, TC-DASH |
| FR-ANL-011 | Tenant tắt được telemetry sản phẩm; telemetry ổn định (hiệu năng, crash) vẫn thu | P4 | _suy luận:_ [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md) · [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md) · [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) | _suy luận:_ 18 op | _suy luận:_ TC-ANL, TC-DASH |
| FR-ANL-012 | Có trang dashboard cho giáo viên/lớp, trường, tenant, platform và Game Analytics; số liệu theo ngày/tuần/tháng | P3–P4 | _suy luận:_ [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md) · [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md) · [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) | _suy luận:_ 18 op | _suy luận:_ TC-ANL, TC-DASH |
| FR-ANL-013 | Dashboard chỉ đọc bảng tổng hợp dựng gia tăng; khoảng thời gian tối đa 2 năm, độ hạt tự chọn theo khoảng; h… | P3 | _suy luận:_ [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md) · [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md) · [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) | _suy luận:_ 18 op | _suy luận:_ TC-ANL, TC-DASH |
| FR-ANL-014 | Xuất báo cáo dashboard chạy bất đồng bộ, link tải có hạn, có audit | P3 | _suy luận:_ [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md) · [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md) · [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) | _suy luận:_ 18 op | _suy luận:_ TC-ANL, TC-DASH |

### 3.11. `GAME` Game World & Character

Service: game-service · Tài liệu sở hữu: [WORLD_DESIGN](../01-game-design/WORLD_DESIGN.md) · [CHARACTER_SYSTEM](../01-game-design/CHARACTER_SYSTEM.md)

AC: — · US: US-GAME-001, US-GAME-002

Mở rộng ở tài liệu sở hữu: 0 FR, 9 AC, 0 US — [GAME_DESIGN](../01-game-design/GAME_DESIGN.md) · [WORLD_DESIGN](../01-game-design/WORLD_DESIGN.md)

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-GAME-001 | Thế giới 3D stylized/cartoon/low-poly, thân thiện, không bạo lực | P2 | _suy luận:_ [WORLD_DESIGN](../01-game-design/WORLD_DESIGN.md) · [CHARACTER_SYSTEM](../01-game-design/CHARACTER_SYSTEM.md) | _suy luận:_ 21 op | _suy luận:_ TC-WLD, TC-GAM |
| FR-GAME-002 | Thế giới gồm 8 vùng đất có cốt truyện, bao trọn 7 khu chức năng bắt buộc (Main Hub, Learning Zone theo môn,… | P2 (Hub, vùng môn học, Practice, Quest) – P3 (Exam) – P4 (Arena) – P5 (Social) | _suy luận:_ [WORLD_DESIGN](../01-game-design/WORLD_DESIGN.md) · [CHARACTER_SYSTEM](../01-game-design/CHARACTER_SYSTEM.md) | _suy luận:_ 21 op | _suy luận:_ TC-WLD, TC-GAM |
| FR-GAME-003 | Character controller, camera controller, animation state machine, NPC interaction | P2 | _suy luận:_ [WORLD_DESIGN](../01-game-design/WORLD_DESIGN.md) · [CHARACTER_SYSTEM](../01-game-design/CHARACTER_SYSTEM.md) | _suy luận:_ 21 op | _suy luận:_ TC-WLD, TC-GAM |
| FR-GAME-004 | Avatar customization: skin, outfit, hair, accessories, emotes; character level; inventory | P2 | _suy luận:_ [WORLD_DESIGN](../01-game-design/WORLD_DESIGN.md) · [CHARACTER_SYSTEM](../01-game-design/CHARACTER_SYSTEM.md) | _suy luận:_ 21 op | _suy luận:_ TC-WLD, TC-GAM |
| FR-GAME-015 | Luồng tạo avatar lần đầu (preset, dáng người, màu da/tóc, trang phục mặc định, nickname), equipment slot, a… | P2 | _suy luận:_ [WORLD_DESIGN](../01-game-design/WORLD_DESIGN.md) · [CHARACTER_SYSTEM](../01-game-design/CHARACTER_SYSTEM.md) | _suy luận:_ 21 op | _suy luận:_ TC-WLD, TC-GAM |
| FR-GAME-016 | Mạch truyện "Thắp lại Tia Sáng" gồm 7 chương gắn vùng; chương môn học mở song song, không khóa học tập theo… | P2–P4 | _suy luận:_ [WORLD_DESIGN](../01-game-design/WORLD_DESIGN.md) · [CHARACTER_SYSTEM](../01-game-design/CHARACTER_SYSTEM.md) | _suy luận:_ 21 op | _suy luận:_ TC-WLD, TC-GAM |
| FR-GAME-017 | World map, cổng di chuyển, khám phá (discovery point, collectible, % hoàn thành bản đồ), mở khóa vùng theo… | P2–P3 | _suy luận:_ [WORLD_DESIGN](../01-game-design/WORLD_DESIGN.md) · [CHARACTER_SYSTEM](../01-game-design/CHARACTER_SYSTEM.md) | _suy luận:_ 21 op | _suy luận:_ TC-WLD, TC-GAM |
| FR-GAME-018 | Học tập lồng trong ngữ cảnh game: mỗi khối học tập ngắn rồi xen tương tác thế giới; không biến game thành c… | P2 | _suy luận:_ [WORLD_DESIGN](../01-game-design/WORLD_DESIGN.md) · [CHARACTER_SYSTEM](../01-game-design/CHARACTER_SYSTEM.md) | _suy luận:_ 21 op | _suy luận:_ TC-WLD, TC-GAM |
| FR-GAME-005 | Nickname sinh tự động/chọn từ danh sách duyệt; tên thật không hiển thị cho người chơi khác | P2 | _suy luận:_ [WORLD_DESIGN](../01-game-design/WORLD_DESIGN.md) · [CHARACTER_SYSTEM](../01-game-design/CHARACTER_SYSTEM.md) | _suy luận:_ 21 op | _suy luận:_ TC-WLD, TC-GAM |
| FR-GAME-006 | Game client data-driven: quest, mini-game, NPC dialogue, reward, cấu hình khu vực tải từ backend; cập nhật… | P2 | _suy luận:_ [WORLD_DESIGN](../01-game-design/WORLD_DESIGN.md) · [CHARACTER_SYSTEM](../01-game-design/CHARACTER_SYSTEM.md) | _suy luận:_ 21 op | _suy luận:_ TC-WLD, TC-GAM |
| FR-GAME-007 | Game session server-side (/api/v1/game/sessions), ghi platform, client version, end reason | P1 | _suy luận:_ [WORLD_DESIGN](../01-game-design/WORLD_DESIGN.md) · [CHARACTER_SYSTEM](../01-game-design/CHARACTER_SYSTEM.md) | `createGameSession` | _suy luận:_ TC-WLD, TC-GAM |
| FR-GAME-008 | Kiểm tra phiên bản client tối thiểu; buộc cập nhật khi không tương thích API | P1 | _suy luận:_ [WORLD_DESIGN](../01-game-design/WORLD_DESIGN.md) · [CHARACTER_SYSTEM](../01-game-design/CHARACTER_SYSTEM.md) | _suy luận:_ 21 op | _suy luận:_ TC-WLD, TC-GAM |
| FR-GAME-009 | Chạy được trên máy tầm trung: renderer Mobile/Compatibility, preset Thấp/Trung bình/Cao tự phát hiện, độ ph… | P2–P6 | _suy luận:_ [WORLD_DESIGN](../01-game-design/WORLD_DESIGN.md) · [CHARACTER_SYSTEM](../01-game-design/CHARACTER_SYSTEM.md) | _suy luận:_ 21 op | _suy luận:_ TC-WLD, TC-GAM |
| FR-GAME-010 | UI theo nhóm tuổi: lớp 1–5 trực quan/ít thao tác/chữ to + hỗ trợ đọc âm thanh; lớp 6–9 nhiều thông tin hơn | P2 | _suy luận:_ [WORLD_DESIGN](../01-game-design/WORLD_DESIGN.md) · [CHARACTER_SYSTEM](../01-game-design/CHARACTER_SYSTEM.md) | _suy luận:_ 21 op | _suy luận:_ TC-WLD, TC-GAM |
| FR-GAME-011 | Telemetry client (crash, FPS, load time) không chứa PII, rate limit (/api/v1/game/telemetry) | P2 | _suy luận:_ [WORLD_DESIGN](../01-game-design/WORLD_DESIGN.md) · [CHARACTER_SYSTEM](../01-game-design/CHARACTER_SYSTEM.md) | `postTelemetry` | _suy luận:_ TC-WLD, TC-GAM |
| FR-GAME-012 | Nhắc nghỉ sau thời gian chơi liên tục (mặc định 45 phút), giờ truy cập do trường cấu hình | P2 | _suy luận:_ [WORLD_DESIGN](../01-game-design/WORLD_DESIGN.md) · [CHARACTER_SYSTEM](../01-game-design/CHARACTER_SYSTEM.md) | _suy luận:_ 21 op | _suy luận:_ TC-WLD, TC-GAM |
| FR-GAME-013 | Lưu thời gian đăng nhập game: mỗi lần vào game ghi game.login_events (thời điểm vào/ra theo giờ server, nền… | P1 | _suy luận:_ [WORLD_DESIGN](../01-game-design/WORLD_DESIGN.md) · [CHARACTER_SYSTEM](../01-game-design/CHARACTER_SYSTEM.md) | _suy luận:_ 21 op | _suy luận:_ TC-WLD, TC-GAM |
| FR-GAME-014 | Game có nút "Đăng xuất"; đóng game kết thúc phiên; thiết bị phòng máy dùng chung luôn xóa token khi đóng; p… | P1 | _suy luận:_ [WORLD_DESIGN](../01-game-design/WORLD_DESIGN.md) · [CHARACTER_SYSTEM](../01-game-design/CHARACTER_SYSTEM.md) | _suy luận:_ 21 op | _suy luận:_ TC-WLD, TC-GAM |

### 3.12. `QST` Quest

Service: game-service · Tài liệu sở hữu: [QUEST_SYSTEM](../01-game-design/QUEST_SYSTEM.md)

AC: — · US: US-QST-001, US-QST-002

Mở rộng ở tài liệu sở hữu: 0 FR, 13 AC, 0 US — [QUEST_SYSTEM](../01-game-design/QUEST_SYSTEM.md)

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-QST-001 | Quest có: ID, tên, mô tả, loại, lớp, môn, chủ đề, LO, điều kiện bắt đầu, các bước, điều kiện hoàn thành, ph… | P2 | _suy luận:_ [QUEST_SYSTEM](../01-game-design/QUEST_SYSTEM.md) | _suy luận:_ 19 op | _suy luận:_ TC-GAM |
| FR-QST-002 | 12 loại quest: Main, Side, Daily, Weekly, Learning, Exploration, Social, Party, Challenge, Seasonal, Specia… | P2 (Party/Challenge/Boss: P3–P4) | _suy luận:_ [QUEST_SYSTEM](../01-game-design/QUEST_SYSTEM.md) | _suy luận:_ 19 op | _suy luận:_ TC-GAM |
| FR-QST-008 | Quest hỗ trợ narrative, NPC, level requirement, learning requirement, repeatability, cooldown, failure cond… | P2 | _suy luận:_ [QUEST_SYSTEM](../01-game-design/QUEST_SYSTEM.md) | _suy luận:_ 19 op | _suy luận:_ TC-GAM |
| FR-QST-003 | Quest liên kết nội dung học tập; bước activity tạo Attempt trong Activity & Attempt Engine | P2 | _suy luận:_ [QUEST_SYSTEM](../01-game-design/QUEST_SYSTEM.md) | _suy luận:_ 19 op | _suy luận:_ TC-GAM |
| FR-QST-004 | Bước: talk_npc, go_to, activity, collect, minigame, party_activity | P2 | _suy luận:_ [QUEST_SYSTEM](../01-game-design/QUEST_SYSTEM.md) | _suy luận:_ 19 op | _suy luận:_ TC-GAM |
| FR-QST-005 | Tiến độ quest lưu server, hoàn thành bước qua domain event (ADR-0019) | P2 | _suy luận:_ [QUEST_SYSTEM](../01-game-design/QUEST_SYSTEM.md) | _suy luận:_ 19 op | _suy luận:_ TC-GAM |
| FR-QST-006 | Quest lọc theo grade_level của học sinh; không giao nội dung sai khối | P2 | _suy luận:_ [QUEST_SYSTEM](../01-game-design/QUEST_SYSTEM.md) | _suy luận:_ 19 op | _suy luận:_ TC-GAM |
| FR-QST-007 | Shared Quest Progress theo cấu hình shared_progress_policy; kết quả học tập vẫn riêng từng học sinh | P4 | _suy luận:_ [QUEST_SYSTEM](../01-game-design/QUEST_SYSTEM.md) | _suy luận:_ 19 op | _suy luận:_ TC-GAM |

### 3.13. `MG` Mini-game Engine

Service: learning-service · game client · Tài liệu sở hữu: [MINIGAME_ENGINE](../08-game-client/MINIGAME_ENGINE.md)

AC: — · US: US-MG-001

Mở rộng ở tài liệu sở hữu: 0 FR, 6 AC, 0 US — [MINIGAME_ENGINE](../08-game-client/MINIGAME_ENGINE.md)

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-MG-001 | Loại: Multiple Choice, True/False, Fill in the Blank, Drag and Drop, Matching, Ordering, Calculation, Puzzl… | P2 (6 loại) – sau | _suy luận:_ [MINIGAME_ENGINE](../08-game-client/MINIGAME_ENGINE.md) | _suy luận:_ 1 op | _suy luận:_ TC-ACT |
| FR-MG-002 | Mini-game không có ngân hàng câu hỏi riêng; lấy từ Learning Engine qua QuestionSet/selection rule | P2 | _suy luận:_ [MINIGAME_ENGINE](../08-game-client/MINIGAME_ENGINE.md) | _suy luận:_ 1 op | _suy luận:_ TC-ACT |
| FR-MG-003 | Cấu hình: độ khó, thời gian, số câu, điểm, cách tính kết quả, phản hồi đúng/sai, cơ chế hoàn thành, reward… | P2 | _suy luận:_ [MINIGAME_ENGINE](../08-game-client/MINIGAME_ENGINE.md) | _suy luận:_ 1 op | _suy luận:_ TC-ACT |
| FR-MG-004 | Mỗi mini-game là scene Godot tái sử dụng, nhận MiniGameConfig + render payload | P2 | _suy luận:_ [MINIGAME_ENGINE](../08-game-client/MINIGAME_ENGINE.md) | _suy luận:_ 1 op | _suy luận:_ TC-ACT |

### 3.14. `MP` Multiplayer & Presence

Service: competition-service · realtime-gateway · Tài liệu sở hữu: [MULTIPLAYER_GAMEPLAY](../03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md) · [REALTIME_ARCHITECTURE](../06-architecture/REALTIME_ARCHITECTURE.md)

Mở rộng ở tài liệu sở hữu: 0 FR, 11 AC, 0 US — [MULTIPLAYER_GAMEPLAY](../03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md)

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-MP-001 | Hỗ trợ Multiplayer World, Room, Lobby, Team, Match, Session | P4 | _suy luận:_ [MULTIPLAYER_GAMEPLAY](../03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md) · [REALTIME_ARCHITECTURE](../06-architecture/REALTIME_ARCHITECTURE.md) | _suy luận:_ 3 op | _suy luận:_ TC-PTY, TC-CMP |
| FR-MP-002 | Hub instance tối đa 40 người, cấp theo school/grade band; bạn bè/party ưu tiên cùng instance (ADR-0012) | P4 | _suy luận:_ [MULTIPLAYER_GAMEPLAY](../03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md) · [REALTIME_ARCHITECTURE](../06-architecture/REALTIME_ARCHITECTURE.md) | _suy luận:_ 3 op | _suy luận:_ TC-PTY, TC-CMP |
| FR-MP-003 | Presence/online status (Valkey), chỉ hiển thị cho bạn bè/lớp theo policy | P4 | _suy luận:_ [MULTIPLAYER_GAMEPLAY](../03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md) · [REALTIME_ARCHITECTURE](../06-architecture/REALTIME_ARCHITECTURE.md) | _suy luận:_ 3 op | _suy luận:_ TC-PTY, TC-CMP |
| FR-MP-004 | Đồng bộ vị trí qua WebSocket ≤10 Hz, interpolation, server kiểm tra tốc độ/biên (ADR-0011) | P4 | _suy luận:_ [MULTIPLAYER_GAMEPLAY](../03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md) · [REALTIME_ARCHITECTURE](../06-architecture/REALTIME_ARCHITECTURE.md) | _suy luận:_ 3 op | _suy luận:_ TC-PTY, TC-CMP |
| FR-MP-005 | Heartbeat, timeout, reconnect, session recovery, disconnect handling | P4 | _suy luận:_ [MULTIPLAYER_GAMEPLAY](../03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md) · [REALTIME_ARCHITECTURE](../06-architecture/REALTIME_ARCHITECTURE.md) | _suy luận:_ 3 op | _suy luận:_ TC-PTY, TC-CMP |
| FR-MP-006 | Không gửi chuyển động 3D qua REST; Valkey không làm game simulation server | P4 | _suy luận:_ [MULTIPLAYER_GAMEPLAY](../03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md) · [REALTIME_ARCHITECTURE](../06-architecture/REALTIME_ARCHITECTURE.md) | _suy luận:_ 3 op | _suy luận:_ TC-PTY, TC-CMP |

### 3.15. `PTY` Party

Service: competition-service · Tài liệu sở hữu: [PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md)

AC: AC-PTY-001, AC-PTY-002, AC-PTY-003, AC-PTY-004, AC-PTY-005, AC-PTY-006, AC-PTY-007, AC-PTY-008, AC-PTY-009, AC-PTY-010, AC-PTY-011 · US: US-PTY-001, US-PTY-002, US-PTY-003, US-PTY-004

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-PTY-001 | Create, Join, Leave, Invite Friend, Accept, Reject, Kick, Transfer Leader, Disband, Settings, Member Status… | P4 (Party Chat văn bản tự do: P5) | _suy luận:_ [PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md) | _suy luận:_ 17 op | _suy luận:_ TC-PTY |
| FR-PTY-002 | Thuộc tính: Party ID, name, leader, members, max members, type, visibility, invitation policy, current acti… | P4 | _suy luận:_ [PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md) | _suy luận:_ 17 op | _suy luận:_ TC-PTY |
| FR-PTY-003 | Party types: Friend Party, Study Group, Quest Party, Competition Team, Class Group, Custom Group | P4 | _suy luận:_ [PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md) | _suy luận:_ 17 op | _suy luận:_ TC-PTY |
| FR-PTY-004 | Max members cấu hình được; mặc định 4, cho phép 2–8 | P4 | _suy luận:_ [PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md) | _suy luận:_ 17 op | _suy luận:_ TC-PTY |
| FR-PTY-005 | Một học sinh chỉ có một active Party tại một thời điểm (Addendum §6 ghi cho MVP; giữ nguyên cho Release 1.0… | P4 | _suy luận:_ [PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md) | _suy luận:_ 17 op | _suy luận:_ TC-PTY |
| FR-PTY-006 | Party Lobby: danh sách thành viên, avatar, online, Ready/Not Ready, leader controls, chọn activity & game m… | P4 | _suy luận:_ [PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md) | _suy luận:_ 17 op | _suy luận:_ TC-PTY |
| FR-PTY-007 | Start Session: xác nhận leader → kiểm tra thành viên → kiểm tra điều kiện lớp/quyền → tạo Game Session/room… | P4 | _suy luận:_ [PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md) | _suy luận:_ 17 op | _suy luận:_ TC-PTY |
| FR-PTY-008 | Leader disconnect: grace period 60 giây, sau đó tự chuyển leader cho thành viên tham gia sớm nhất còn online | P4 | _suy luận:_ [PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md) | _suy luận:_ 17 op | _suy luận:_ TC-PTY |
| FR-PTY-009 | Party private cần lời mời/quyền; lời mời hết hạn sau 5 phút; không mời người đã block/bị block | P4 | _suy luận:_ [PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md) | _suy luận:_ 17 op | _suy luận:_ TC-PTY |
| FR-PTY-010 | Giới hạn theo school/class/grade/activity; mặc định chỉ cùng trường | P4 | _suy luận:_ [PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md) | _suy luận:_ 17 op | _suy luận:_ TC-PTY |
| FR-PTY-011 | Chống spam lời mời (rate limit), audit log cho create/kick/transfer/disband | P4 | _suy luận:_ [PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md) | _suy luận:_ 17 op | _suy luận:_ TC-PTY |
| FR-PTY-012 | Thành viên mất kết nối: các thành viên khác tiếp tục nếu activity cho phép (continue_on_disconnect) | P4 | _suy luận:_ [PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md) | _suy luận:_ 17 op | _suy luận:_ TC-PTY |
| FR-PTY-013 | Không chia sẻ kết quả học tập cá nhân giữa thành viên; mỗi người có attempt riêng | P4 | _suy luận:_ [PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md) | _suy luận:_ 17 op | _suy luận:_ TC-PTY |
| FR-PTY-014 | API /api/v1/multiplayer/parties, /api/v1/multiplayer/invitations; realtime event qua /realtime/v1 | P4 | _suy luận:_ [PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md) | `createParty` · `listMyInvitations` | _suy luận:_ TC-PTY |

### 3.16. `MM` Matchmaking

Service: competition-service · Tài liệu sở hữu: [MATCHMAKING](../03-social-multiplayer/MATCHMAKING.md)

AC: — · US: US-MM-001

Mở rộng ở tài liệu sở hữu: 0 FR, 8 AC, 0 US — [MATCHMAKING](../03-social-multiplayer/MATCHMAKING.md)

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-MM-001 | Chế độ: Random, Skill-based, Grade-based, Subject-based, Topic-based, Friend, Team, Tournament | P4 | _suy luận:_ [MATCHMAKING](../03-social-multiplayer/MATCHMAKING.md) | _suy luận:_ 3 op | _suy luận:_ TC-MM |
| FR-MM-002 | Ràng buộc cứng (khối lớp, độ phù hợp nội dung, policy trường) kiểm tra trước rating; không ghép lớp 1 với l… | P4 | _suy luận:_ [MATCHMAKING](../03-social-multiplayer/MATCHMAKING.md) | _suy luận:_ 3 op | _suy luận:_ TC-MM |
| FR-MM-003 | Mặc định: lớp 1–5 chỉ ghép cùng khối (grade_level); lớp 6–9 theo grade_band (±1 khối khi chờ lâu); chỉ cùng… | P4 | _suy luận:_ [MATCHMAKING](../03-social-multiplayer/MATCHMAKING.md) | _suy luận:_ 3 op | _suy luận:_ TC-MM |
| FR-MM-004 | Vòng đời: Queue → Ticket → Match Proposal → Lobby → Ready Check → Team Assignment → Match Start → Match End… | P4 | _suy luận:_ [MATCHMAKING](../03-social-multiplayer/MATCHMAKING.md) | _suy luận:_ 3 op | _suy luận:_ TC-MM |
| FR-MM-005 | Cửa sổ rating mở rộng theo thời gian chờ; timeout ghép trận 90 giây đề xuất chế độ khác/bot luyện tập | P4 | _suy luận:_ [MATCHMAKING](../03-social-multiplayer/MATCHMAKING.md) | _suy luận:_ 3 op | _suy luận:_ TC-MM |
| FR-MM-006 | Ticket chỉ trong Valkey; mất → client tạo lại (ADR-0004) | P4 | _suy luận:_ [MATCHMAKING](../03-social-multiplayer/MATCHMAKING.md) | _suy luận:_ 3 op | _suy luận:_ TC-MM |

### 3.17. `CMP` Competition / Battle

Service: competition-service · Tài liệu sở hữu: [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md)

AC: AC-CMP-001, AC-CMP-002, AC-CMP-003, AC-CMP-004, AC-CMP-005, AC-CMP-006 · US: —

Mở rộng ở tài liệu sở hữu: 0 FR, 12 AC, 0 US — [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) · [ANTI_CHEAT](../10-security/ANTI_CHEAT.md)

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-CMP-001 | Chế độ: 1v1 Knowledge Battle, Team Battle, Co-op Challenge, Timed Quiz, Subject Tournament, Class/School/Se… | P4 (Seasonal: P6+) | _suy luận:_ [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) | _suy luận:_ 6 op | _suy luận:_ TC-CMP |
| FR-CMP-002 | Battle là state machine server-authoritative: question plan do server chọn, chấm và tính điểm thời gian ser… | P4 | _suy luận:_ [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) | _suy luận:_ 6 op | _suy luận:_ TC-CMP |
| FR-CMP-003 | Kết quả trận ghi một lần (unique match_id), idempotent, audit | P4 | _suy luận:_ [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) | _suy luận:_ 6 op | _suy luận:_ TC-CMP |
| FR-CMP-004 | Câu trả lời trong trận ghi learning evidence với context competition (trọng số thấp cho mastery) | P4 | _suy luận:_ [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) | _suy luận:_ 6 op | _suy luận:_ TC-CMP |
| FR-CMP-005 | Xử lý disconnect: grace 30 giây; quá hạn → xử thua phần còn lại/forfeit theo mode | P4 | _suy luận:_ [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) | _suy luận:_ 6 op | _suy luận:_ TC-CMP |

### 3.18. `RNK` Ranking

Service: competition-service · Tài liệu sở hữu: [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md)

Mở rộng ở tài liệu sở hữu: 0 FR, 6 AC, 0 US — [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md)

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-RNK-001 | Hệ thống ranking độc lập; Glicko-2 theo (student, mode, subject, season) (ADR-0018) | P4 | _suy luận:_ [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) | _suy luận:_ 1 op | _suy luận:_ TC-CMP |
| FR-RNK-002 | Không dùng rating để đánh giá năng lực học tập; tách Learning Score, Mastery, Game Rating, Game XP, Competi… | P4 | _suy luận:_ [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) | _suy luận:_ 1 op | _suy luận:_ TC-CMP |
| FR-RNK-003 | Hiển thị rating cho học sinh dưới dạng tier thân thiện, không số chính xác cho lớp 1–5 | P4 | _suy luận:_ [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) | _suy luận:_ 1 op | _suy luận:_ TC-CMP |
| FR-RNK-004 | Season soft-reset rating | P4 | _suy luận:_ [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) | _suy luận:_ 1 op | _suy luận:_ TC-CMP |

### 3.19. `LDB` Leaderboard

Service: competition-service · Tài liệu sở hữu: [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md)

AC: — · US: US-LDB-001

Mở rộng ở tài liệu sở hữu: 0 FR, 6 AC, 0 US — [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md)

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-LDB-001 | Bảng: Global, School, Class, Grade, Subject, Topic, Game Mode, Friends, Team, Daily, Weekly, Monthly, Season | P4 (Global trong Release 1.0; quy tắc hiển thị nickname theo FR-LDB-003) | _suy luận:_ [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) | _suy luận:_ 2 op | _suy luận:_ TC-CMP |
| FR-LDB-002 | Leaderboard cache bằng Valkey sorted set, nguồn từ PostgreSQL | P4 | _suy luận:_ [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) | _suy luận:_ 2 op | _suy luận:_ TC-CMP |
| FR-LDB-003 | Leaderboard ngoài phạm vi trường chỉ hiển thị nickname, không tên trường/lớp chi tiết | P4 | _suy luận:_ [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) | _suy luận:_ 2 op | _suy luận:_ TC-CMP |
| FR-LDB-004 | Trường có thể tắt leaderboard công khai cho lớp 1–3 | P4 | _suy luận:_ [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) | _suy luận:_ 2 op | _suy luận:_ TC-CMP |

### 3.20. `TRN` Tournament

Service: competition-service · Tài liệu sở hữu: [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md)

AC: — · US: US-TRN-001

Mở rộng ở tài liệu sở hữu: 0 FR, 8 AC, 0 US — [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md)

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-TRN-001 | Format: Single Elimination, Double Elimination, Round Robin, Team, School, Class Tournament | P4 (Single elim, Round robin) – P6 (còn lại) | _suy luận:_ [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) | _suy luận:_ 1 op | _suy luận:_ TC-CMP |
| FR-TRN-002 | Tạo giải, đăng ký, chia bảng, tạo bracket, lên lịch, xác nhận người tham gia, quản lý trận, xác nhận kết qu… | P4 | _suy luận:_ [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) | _suy luận:_ 1 op | _suy luận:_ TC-CMP |
| FR-TRN-003 | Bracket lưu PostgreSQL, không chỉ Valkey | P4 | _suy luận:_ [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) | _suy luận:_ 1 op | _suy luận:_ TC-CMP |

### 3.21. `SOC` Social

Service: chat-service · Tài liệu sở hữu: [SOCIAL_SYSTEM](../03-social-multiplayer/SOCIAL_SYSTEM.md)

AC: — · US: US-SOC-001

Mở rộng ở tài liệu sở hữu: 0 FR, 10 AC, 0 US — [SOCIAL_SYSTEM](../03-social-multiplayer/SOCIAL_SYSTEM.md)

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-SOC-001 | Friend list, friend request, invitation | P5 (cơ bản cho mời party: P4) | _suy luận:_ [SOCIAL_SYSTEM](../03-social-multiplayer/SOCIAL_SYSTEM.md) | _suy luận:_ 35 op | _suy luận:_ TC-SOC |
| FR-SOC-002 | Mặc định chỉ kết bạn cùng trường (như nhau cho mọi lớp); tenant unit có thể mở liên trường trong tenant | P5 | _suy luận:_ [SOCIAL_SYSTEM](../03-social-multiplayer/SOCIAL_SYSTEM.md) | _suy luận:_ 35 op | _suy luận:_ TC-SOC |
| FR-SOC-003 | Block / Mute người dùng; block chặn chat, mời, ghép party | P5 (Block: P4 cùng party) | _suy luận:_ [SOCIAL_SYSTEM](../03-social-multiplayer/SOCIAL_SYSTEM.md) | _suy luận:_ 35 op | _suy luận:_ TC-SOC |
| FR-SOC-004 | Team, Study Group do giáo viên hoặc học sinh tạo theo policy | P5 | _suy luận:_ [SOCIAL_SYSTEM](../03-social-multiplayer/SOCIAL_SYSTEM.md) | _suy luận:_ 35 op | _suy luận:_ TC-SOC |

### 3.22. `CHAT` Chat

Service: chat-service · Tài liệu sở hữu: [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md)

AC: AC-CHAT-001, AC-CHAT-002, AC-CHAT-003, AC-CHAT-004, AC-CHAT-005, AC-CHAT-006, AC-CHAT-007, AC-CHAT-008, AC-CHAT-009, AC-CHAT-010, AC-CHAT-011, AC-CHAT-012, AC-CHAT-013, AC-CHAT-014 · US: US-CHAT-001, US-CHAT-002

Mở rộng ở tài liệu sở hữu: 0 FR, 29 AC, 0 US — [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md)

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-CHAT-001 | Channel: World, Area, Match, Party, Team, Class, Study Group, Direct Message; cấu hình bằng policy | P4 (Match/Party/Team theo trận), P5 (còn lại) | _suy luận:_ [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md) | _suy luận:_ 15 op | _suy luận:_ TC-CHAT |
| FR-CHAT-002 | Không có rule = deny; quyền vào channel theo quan hệ (cùng trường/instance/nhóm/lớp/bạn bè) và policy, như… | P5 | _suy luận:_ [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md) | _suy luận:_ 15 op | _suy luận:_ TC-CHAT |
| FR-CHAT-003 | Text, emoji, reactions, quick messages, group chat, history, unread count, notifications | P4 (channel gameplay), P5 (còn lại) | _suy luận:_ [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md) | _suy luận:_ 15 op | _suy luận:_ TC-CHAT |
| FR-CHAT-004 | Một bộ quy tắc chat cho mọi lớp 1–9 (không điều kiện theo grade_level/grade_band): đủ 7 channel; World = cù… | P5 | _suy luận:_ [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md) | _suy luận:_ 15 op | _suy luận:_ TC-CHAT |
| FR-CHAT-005 | Mọi tin nhắn phải hợp lệ: pipeline đồng bộ phía server cho mọi kind — normalize → định dạng/độ dài → profan… | P5 | _suy luận:_ [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md) | _suy luận:_ 15 op | _suy luận:_ TC-CHAT |
| FR-CHAT-006 | Lịch sử chat theo quyền và retention (mặc định 180 ngày) | P5 | _suy luận:_ [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md) | _suy luận:_ 15 op | _suy luận:_ TC-CHAT |
| FR-CHAT-007 | Không có voice chat trong Release 1.0; điểm mở rộng chỉ ở mức thiết kế, không hiện thực (ADR-0051, ADR-0070) | — | _suy luận:_ [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md) | _suy luận:_ 15 op | _suy luận:_ TC-CHAT |
| FR-CHAT-008 | Giao tin nhắn realtime qua /realtime/v1, gửi qua REST hoặc WS có idempotency client message id | P4 | _suy luận:_ [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md) | _suy luận:_ 15 op | _suy luận:_ TC-CHAT |
| FR-CHAT-009 | Chat gameplay luôn có (ADR-0020 mục 6): vào trận 1v1 → channel match; trận đội → team mỗi đội + match chung… | P4 | _suy luận:_ [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md) | _suy luận:_ 15 op | _suy luận:_ TC-CHAT |
| FR-CHAT-010 | UI chat trong lobby/trận/party: panel thu gọn được nhưng không tắt; quick message một chạm; tab Đội/Cả trận… | P4 | _suy luận:_ [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md) | _suy luận:_ 15 op | _suy luận:_ TC-CHAT |
| FR-CHAT-011 | Purge chat gameplay (ADR-0033): channel match, team theo trận, party (sau giải tán) bị xóa vật lý sau gamep… | P4 | _suy luận:_ [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md) | _suy luận:_ 15 op | _suy luận:_ TC-CHAT |
| FR-CHAT-012 | Sửa tin trong 15 phút (chạy lại kiểm duyệt, lưu revision) và thu hồi trong 24 giờ (placeholder, giữ bằng ch… | P4 | _suy luận:_ [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md) | _suy luận:_ 15 op | _suy luận:_ TC-CHAT |
| FR-CHAT-013 | Reply, Quote (trích từ bản đã che), Forward (cùng tenant, ≤ 5 đích, chạy lại kiểm duyệt) — ADR-0034 | P4 | _suy luận:_ [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md) | _suy luận:_ 15 op | _suy luận:_ TC-CHAT |
| FR-CHAT-014 | Emoji, Sticker (pack platform, có thể mở khóa bằng phần thưởng), GIF (thư viện nội bộ đã duyệt, không dịch… | P4 | _suy luận:_ [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md) | _suy luận:_ 15 op | _suy luận:_ TC-CHAT |
| FR-CHAT-015 | Mention @nickname (≤ 5/tin, thông báo + đếm riêng), Pin (≤ 3/channel), Copy (bản đã che), Typing indicator… | P4 | _suy luận:_ [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md) | _suy luận:_ 15 op | _suy luận:_ TC-CHAT |
| FR-CHAT-016 | Trạng thái Sent/Delivered/Read theo con trỏ, Unread count + mention count, Lịch sử phân trang và tìm kiếm f… | P4 (tìm kiếm P5) | _suy luận:_ [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md) | _suy luận:_ 15 op | _suy luận:_ TC-CHAT |
| FR-CHAT-017 | Truyền tải chat qua NATS JetStream (bền vững, trước khi xác nhận Sent) + Valkey Pub/Sub (fan-out realtime),… | P4 | _suy luận:_ [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md) | _suy luận:_ 15 op | _suy luận:_ TC-CHAT |

### 3.23. `MOD` Moderation & Child Safety

Service: chat-service · Tài liệu sở hữu: [MODERATION](../03-social-multiplayer/MODERATION.md) · [CHILD_SAFETY](../10-security/CHILD_SAFETY.md)

AC: AC-MOD-020, AC-MOD-021, AC-MOD-022 · US: US-MOD-001, US-MOD-002

Mở rộng ở tài liệu sở hữu: 0 FR, 22 AC, 0 US — [MODERATION](../03-social-multiplayer/MODERATION.md)

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-MOD-001 | Content/profanity filtering, anti-spam, anti-bullying, PII detection | P5 | _suy luận:_ [MODERATION](../03-social-multiplayer/MODERATION.md) · [CHILD_SAFETY](../10-security/CHILD_SAFETY.md) | _suy luận:_ 32 op | _suy luận:_ TC-CHAT |
| FR-MOD-002 | Report message, report user, block, mute | P5 | _suy luận:_ [MODERATION](../03-social-multiplayer/MODERATION.md) · [CHILD_SAFETY](../10-security/CHILD_SAFETY.md) | _suy luận:_ 32 op | _suy luận:_ TC-CHAT |
| FR-MOD-003 | Moderation queue, xử lý theo phạm vi trường (teacher/school admin), audit mọi hành động | P5 | _suy luận:_ [MODERATION](../03-social-multiplayer/MODERATION.md) · [CHILD_SAFETY](../10-security/CHILD_SAFETY.md) | _suy luận:_ 32 op | _suy luận:_ TC-CHAT |
| FR-MOD-004 | Chat policy theo platform → tenant → school → class (không theo grade — thay mục "Age/Grade-based Policy" c… | P5 | _suy luận:_ [MODERATION](../03-social-multiplayer/MODERATION.md) · [CHILD_SAFETY](../10-security/CHILD_SAFETY.md) | _suy luận:_ 32 op | _suy luận:_ TC-CHAT |
| FR-MOD-005 | Không ai có quyền đọc toàn bộ chat của mọi trường; truy cập chat đều audit | P5 | _suy luận:_ [MODERATION](../03-social-multiplayer/MODERATION.md) · [CHILD_SAFETY](../10-security/CHILD_SAFETY.md) | _suy luận:_ 32 op | _suy luận:_ TC-CHAT |
| FR-MOD-006 | Sanction: warn, mute có thời hạn, restrict (chỉ quick message), escalate | P5 | _suy luận:_ [MODERATION](../03-social-multiplayer/MODERATION.md) · [CHILD_SAFETY](../10-security/CHILD_SAFETY.md) | _suy luận:_ 32 op | _suy luận:_ TC-CHAT |
| FR-MOD-007 | Teacher controls: mute lớp, khóa channel lớp, xem report lớp mình | P5 | _suy luận:_ [MODERATION](../03-social-multiplayer/MODERATION.md) · [CHILD_SAFETY](../10-security/CHILD_SAFETY.md) | _suy luận:_ 32 op | _suy luận:_ TC-CHAT |
| FR-MOD-008 | Không giám sát chat vượt quá mục đích an toàn cần thiết | P5 | _suy luận:_ [MODERATION](../03-social-multiplayer/MODERATION.md) · [CHILD_SAFETY](../10-security/CHILD_SAFETY.md) | _suy luận:_ 32 op | _suy luận:_ TC-CHAT |
| FR-MOD-009 | Danh mục từ nhạy cảm quản lý trong CMS: category (severity, action mặc định), term (match type, biến thể tự… | P5 | _suy luận:_ [MODERATION](../03-social-multiplayer/MODERATION.md) · [CHILD_SAFETY](../10-security/CHILD_SAFETY.md) | _suy luận:_ 32 op | _suy luận:_ TC-CHAT |
| FR-MOD-010 | Action theo mức: mask / block / hold_for_review / block_and_alert; phản hồi không lộ từ đã khớp | P5 | _suy luận:_ [MODERATION](../03-social-multiplayer/MODERATION.md) · [CHILD_SAFETY](../10-security/CHILD_SAFETY.md) | _suy luận:_ 32 op | _suy luận:_ TC-CHAT |
| FR-MOD-011 | Công tắc kiểm duyệt bật/tắt toàn bộ hoặc từng bước (sensitive_terms, pii, spam, bullying, async_analysis) t… | P5 | _suy luận:_ [MODERATION](../03-social-multiplayer/MODERATION.md) · [CHILD_SAFETY](../10-security/CHILD_SAFETY.md) | _suy luận:_ 32 op | _suy luận:_ TC-CHAT |
| FR-MOD-012 | Khi tắt vẫn giữ độ dài tối đa, rate limit, quyền channel, sanction, block/mute/report, moderation thủ công | P5 | _suy luận:_ [MODERATION](../03-social-multiplayer/MODERATION.md) · [CHILD_SAFETY](../10-security/CHILD_SAFETY.md) | _suy luận:_ 32 op | _suy luận:_ TC-CHAT |
| FR-MOD-013 | Tenant thêm từ bổ sung (chỉ siết chặt) và bật/tắt category khi allow_tenant_toggle | P5 | _suy luận:_ [MODERATION](../03-social-multiplayer/MODERATION.md) · [CHILD_SAFETY](../10-security/CHILD_SAFETY.md) | _suy luận:_ 32 op | _suy luận:_ TC-CHAT |
| FR-MOD-014 | Công tắc có thời hạn expires_at tự bật lại | P5 | _suy luận:_ [MODERATION](../03-social-multiplayer/MODERATION.md) · [CHILD_SAFETY](../10-security/CHILD_SAFETY.md) | _suy luận:_ 32 op | _suy luận:_ TC-CHAT |
| FR-MOD-015 | Publish danh mục chạy test hồi quy (recall ≥ 95%, false positive ≤ 3%) và reload không downtime ≤ 60 s | P5 | _suy luận:_ [MODERATION](../03-social-multiplayer/MODERATION.md) · [CHILD_SAFETY](../10-security/CHILD_SAFETY.md) | _suy luận:_ 32 op | _suy luận:_ TC-CHAT |

### 3.24. `NTF` Notification

Service: chat-service · Tài liệu sở hữu: [SOCIAL_SYSTEM](../03-social-multiplayer/SOCIAL_SYSTEM.md)

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-NTF-001 | Thông báo trong game: lời mời party/kết bạn, bài được giao, kết quả, phần thưởng, thông báo trường | P2 (in-game) – P5 (realtime đầy đủ) | _suy luận:_ [SOCIAL_SYSTEM](../03-social-multiplayer/SOCIAL_SYSTEM.md) | _suy luận:_ 1 op | _suy luận:_ TC-SOC |
| FR-NTF-002 | Realtime qua WebSocket; lưu bền vững để xem lại (/api/v1/notifications) | P5 | _suy luận:_ [SOCIAL_SYSTEM](../03-social-multiplayer/SOCIAL_SYSTEM.md) | `listNotifications` | _suy luận:_ TC-SOC |
| FR-NTF-003 | Không gửi push notification ngoài game tới học sinh trong Release 1.0; thông báo chỉ hiển thị trong game (i… | P5 | [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md) | _suy luận:_ 1 op | _suy luận:_ TC-SOC |

### 3.25. `GAM` XP, level, tiến trình

Service: game-service · Tài liệu sở hữu: [PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md)

AC: AC-GAM-001, AC-GAM-002, AC-GAM-003, AC-GAM-011 · US: US-GAM-001

Mở rộng ở tài liệu sở hữu: 0 FR, 7 AC, 0 US — [PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md) · [WORLD_DESIGN](../01-game-design/WORLD_DESIGN.md)

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-GAM-001 | XP, Level, Achievement, Badge, Daily/Weekly Challenge, Season Reward, Collection | P2 (XP, Level, Badge) – P4 (Season) | _suy luận:_ [PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md) | _suy luận:_ 5 op | _suy luận:_ TC-GAM |
| FR-GAM-002 | XP từ hoàn thành bài học, bài tập, quest, hoạt động, hợp tác, thi đấu | P2–P4 | _suy luận:_ [PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md) | _suy luận:_ 5 op | _suy luận:_ TC-GAM |
| FR-GAM-003 | Reward Engine có rule + cap ngày/tuần + diminishing returns khi lặp nội dung; chống farm câu dễ | P2 | _suy luận:_ [PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md) | _suy luận:_ 5 op | _suy luận:_ TC-GAM |
| FR-GAM-004 | XP ledger append-only, idempotent theo (source_type, source_id, student_id, rule_id) | P2 | _suy luận:_ [PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md) | _suy luận:_ 5 op | _suy luận:_ TC-GAM |
| FR-GAM-005 | Phần thưởng game không ảnh hưởng điểm/mastery/đánh giá chính thức | P2 | _suy luận:_ [PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md) | _suy luận:_ 5 op | _suy luận:_ TC-GAM |
| FR-GAM-006 | Ba miền tiến trình (game, learning, competition) tách biệt, không gộp điểm; game level không phải bằng chứn… | P2 | _suy luận:_ [PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md) | _suy luận:_ 5 op | _suy luận:_ TC-GAM |
| FR-GAM-007 | Catalog nguồn XP có mã; đường cong XP cấu hình được, có version, không hardcode trong Godot; danh hiệu theo… | P2 | _suy luận:_ [PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md) | _suy luận:_ 5 op | _suy luận:_ TC-GAM |
| FR-GAM-008 | Level-up event, phần thưởng lên cấp, progress bar, lịch sử level và giao dịch XP | P2 | _suy luận:_ [PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md) | _suy luận:_ 5 op | _suy luận:_ TC-GAM |
| FR-GAM-009 | World progression: mở khóa khu theo level/quest/LO/achievement, điểm khám phá, vật sưu tầm, hoàn thành bản… | P2–P3 | _suy luận:_ [PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md) | _suy luận:_ 5 op | _suy luận:_ TC-GAM |

### 3.26. `RWD` Phần thưởng

Service: game-service · Tài liệu sở hữu: [PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md) · [INVENTORY_ECONOMY](../01-game-design/INVENTORY_ECONOMY.md)

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-RWD-001 | Inventory, cosmetic (outfit, hair, accessory, emote) mở khóa bằng bài tập, quest, sự kiện, thành tựu, thi đấu | P2 | _suy luận:_ [PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md) · [INVENTORY_ECONOMY](../01-game-design/INVENTORY_ECONOMY.md) | _suy luận:_ 10 op | _suy luận:_ TC-GAM, TC-ECO |
| FR-RWD-002 | Không cơ chế trả tiền để có kết quả học tập tốt hơn; không loot box | P2 | _suy luận:_ [PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md) · [INVENTORY_ECONOMY](../01-game-design/INVENTORY_ECONOMY.md) | _suy luận:_ 10 op | _suy luận:_ TC-GAM, TC-ECO |
| FR-RWD-003 | Reward có giá trị nghiệp vụ chỉ do server trao | P2 | _suy luận:_ [PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md) · [INVENTORY_ECONOMY](../01-game-design/INVENTORY_ECONOMY.md) | _suy luận:_ 10 op | _suy luận:_ TC-GAM, TC-ECO |

### 3.27. `CHL` Challenge & Boss

Service: game-service · Tài liệu sở hữu: [CHALLENGE_BOSS_SYSTEM](../01-game-design/CHALLENGE_BOSS_SYSTEM.md)

Mở rộng ở tài liệu sở hữu: 0 FR, 8 AC, 0 US — [CHALLENGE_BOSS_SYSTEM](../01-game-design/CHALLENGE_BOSS_SYSTEM.md)

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-CHL-001 | Challenge Engine: 11 loại (time, perfect, survival, puzzle, knowledge, boss, coop, party, weekly, seasonal,… | P3 | _suy luận:_ [CHALLENGE_BOSS_SYSTEM](../01-game-design/CHALLENGE_BOSS_SYSTEM.md) | _suy luận:_ 13 op | _suy luận:_ TC-CHL |
| FR-CHL-002 | Boss Battle nhiều phase, đóng góp cá nhân/nhóm, retry, không phạt khi sai; server-authoritative trong game-… | P3–P4 | _suy luận:_ [CHALLENGE_BOSS_SYSTEM](../01-game-design/CHALLENGE_BOSS_SYSTEM.md) | _suy luận:_ 13 op | _suy luận:_ TC-CHL |
| FR-CHL-003 | Mỗi câu/hoạt động trong challenge/boss là attempt học tập cá nhân; learning record riêng từng thành viên | P3 | _suy luận:_ [CHALLENGE_BOSS_SYSTEM](../01-game-design/CHALLENGE_BOSS_SYSTEM.md) | _suy luận:_ 13 op | _suy luận:_ TC-CHL |

### 3.28. `ECO` Kinh tế, vật phẩm, sưu tầm

Service: game-service · Tài liệu sở hữu: [INVENTORY_ECONOMY](../01-game-design/INVENTORY_ECONOMY.md)

Mở rộng ở tài liệu sở hữu: 0 FR, 8 AC, 0 US — [INVENTORY_ECONOMY](../01-game-design/INVENTORY_ECONOMY.md)

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-ECO-001 | Item catalog (cosmetic, outfit, hair, accessory, emote, pet, title, badge, frame, decoration, knowledge car… | P2 | _suy luận:_ [INVENTORY_ECONOMY](../01-game-design/INVENTORY_ECONOMY.md) | _suy luận:_ 11 op | _suy luận:_ TC-ECO |
| FR-ECO-002 | Tiền trong game "Xu" + sổ cái append-only, ví không âm, đối soát; không mua bằng tiền thật (ADR-0049) | P2 | _suy luận:_ [INVENTORY_ECONOMY](../01-game-design/INVENTORY_ECONOMY.md) | _suy luận:_ 11 op | _suy luận:_ TC-ECO |
| FR-ECO-003 | Shop chỉ cosmetic, giá Xu, giới hạn mua, vật phẩm xác định; không loot box/hộp ngẫu nhiên trả phí | P2 | _suy luận:_ [INVENTORY_ECONOMY](../01-game-design/INVENTORY_ECONOMY.md) | _suy luận:_ 11 op | _suy luận:_ TC-ECO |
| FR-ECO-004 | Collection (pet, knowledge card, cosmetic, badge, exploration, event), xử lý trùng lặp, trading tắt mặc định | P2–P3 | _suy luận:_ [INVENTORY_ECONOMY](../01-game-design/INVENTORY_ECONOMY.md) | _suy luận:_ 11 op | _suy luận:_ TC-ECO |
| FR-ECO-005 | Achievement có định nghĩa, trigger, điều kiện, tiến độ, ẩn, tier, version; không dựa trên thời gian chơi | P2 | _suy luận:_ [INVENTORY_ECONOMY](../01-game-design/INVENTORY_ECONOMY.md) | _suy luận:_ 11 op | _suy luận:_ TC-ECO |

### 3.29. `EVT` Event & Season (live-ops)

Service: game-service · Tài liệu sở hữu: [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md)

Mở rộng ở tài liệu sở hữu: 0 FR, 12 AC, 0 US — [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md)

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-EVT-001 | Sự kiện live-ops: lễ hội thế giới, cuối tuần, sự kiện trường, community goal, chiến dịch học tập, boss đặc… | P3 | _suy luận:_ [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md) | _suy luận:_ 18 op | _suy luận:_ TC-EVT |
| FR-EVT-002 | Event Builder trong Game Operations Console: lịch, đối tượng (khối/trường/tenant), nội dung tham chiếu, phầ… | P3 | _suy luận:_ [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md) | _suy luận:_ 18 op | _suy luận:_ TC-EVT |
| FR-EVT-003 | Vòng đời Scheduled → Live → Ended → Archived, dừng khẩn cấp/tiếp tục/hủy có lý do và audit; lượt chơi đang… | P3 | _suy luận:_ [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md) | _suy luận:_ 18 op | _suy luận:_ TC-EVT |
| FR-EVT-004 | Community goal cộng dồn theo trường/tenant từ event server đã kiểm chứng, trần đóng góp mỗi học sinh/ngày,… | P3 | _suy luận:_ [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md) | _suy luận:_ 18 op | _suy luận:_ TC-EVT |
| FR-EVT-005 | Season track miễn phí (không track trả phí), điểm mùa từ hoạt động đa dạng có trần/ngày, thưởng cosmetic; h… | P4 | _suy luận:_ [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md) | _suy luận:_ 18 op | _suy luận:_ TC-EVT |
| FR-EVT-006 | Tenant tổ chức sự kiện trường từ template platform, chỉ với allowed_params (ADR-0027, ADR-0029) | P3 | _suy luận:_ [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md) | _suy luận:_ 18 op | _suy luận:_ TC-EVT |
| FR-EVT-007 | Quy tắc lành mạnh: không phạt mất chuỗi, không ép online, không khóa học tập, tôn trọng giờ truy cập và nhắ… | P3 | _suy luận:_ [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md) | _suy luận:_ 18 op | _suy luận:_ TC-EVT |
| FR-EVT-008 | Nội dung sự kiện platform song ngữ bắt buộc (ADR-0056); trang trí trong ngân sách máy tầm trung, có biến th… | P3 | _suy luận:_ [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md) | _suy luận:_ 18 op | _suy luận:_ TC-EVT |

### 3.30. `CMS` CMS

Service: learning-service · game-service (game-admin) · Tài liệu sở hữu: [ADMIN_PORTAL](../04-admin-portal/ADMIN_PORTAL.md)

AC: AC-CMS-001, AC-CMS-002, AC-CMS-003, AC-CMS-004, AC-CMS-005 · US: US-CMS-001, US-CMS-002

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-CMS-001 | Vai trò: Platform Admin (→ system_admin), Content Admin, Academic Manager, School Admin, Teacher, Content R… | P3 | _suy luận:_ [ADMIN_PORTAL](../04-admin-portal/ADMIN_PORTAL.md) | _suy luận:_ 48 op | _suy luận:_ TC-CMS |
| FR-CMS-002 | Curriculum Management (chương trình, khối, môn, chủ đề, LO) | P3 | _suy luận:_ [ADMIN_PORTAL](../04-admin-portal/ADMIN_PORTAL.md) | _suy luận:_ 48 op | _suy luận:_ TC-CMS |
| FR-CMS-003 | Content Management: lesson, material, media, question bank, question set, assessment, exercise, learning ac… | P3 | _suy luận:_ [ADMIN_PORTAL](../04-admin-portal/ADMIN_PORTAL.md) | _suy luận:_ 48 op | _suy luận:_ TC-CMS |
| FR-CMS-004 | Game Content: quest, mini-game config, game level, NPC dialogue, reward config, world event, game activity… | P3 | _suy luận:_ [ADMIN_PORTAL](../04-admin-portal/ADMIN_PORTAL.md) | _suy luận:_ 48 op | _suy luận:_ TC-CMS |
| FR-CMS-005 | Competition Management: matchmaking config, ranking rules, tournament/challenge template, season, competiti… | P4 | _suy luận:_ [ADMIN_PORTAL](../04-admin-portal/ADMIN_PORTAL.md) | _suy luận:_ 48 op | _suy luận:_ TC-CMS |
| FR-CMS-006 | Workflow, versioning, preview (Godot Web export nhúng cho nội dung game), publishing, rollback theo chính s… | P3 | _suy luận:_ [ADMIN_PORTAL](../04-admin-portal/ADMIN_PORTAL.md) | _suy luận:_ 48 op | _suy luận:_ TC-CMS |
| FR-CMS-007 | CMS không làm thay đổi lịch sử bài đã hoàn thành | P3 | _suy luận:_ [ADMIN_PORTAL](../04-admin-portal/ADMIN_PORTAL.md) | _suy luận:_ 48 op | _suy luận:_ TC-CMS |
| FR-CMS-008 | Giáo viên tạo nội dung phạm vi trường; reviewer cấp trường duyệt | P3 | _suy luận:_ [ADMIN_PORTAL](../04-admin-portal/ADMIN_PORTAL.md) | _suy luận:_ 48 op | _suy luận:_ TC-CMS |
| FR-CMS-009 | Một Admin Portal React (TypeScript) web/apps/portal với 5 workspace (School & Learning Management, Learning… | P3 | _suy luận:_ [ADMIN_PORTAL](../04-admin-portal/ADMIN_PORTAL.md) | _suy luận:_ 48 op | _suy luận:_ TC-CMS |
| FR-CMS-016 | Dashboard live-ops trong CMS/Admin: CCU, trận đang chạy, hàng chờ ghép trận, đăng nhập/hàng chờ đăng nhập,… | P3 | _suy luận:_ [ADMIN_PORTAL](../04-admin-portal/ADMIN_PORTAL.md) | _suy luận:_ 48 op | _suy luận:_ TC-CMS |
| FR-CMS-017 | Thumbnail/turntable render tự động từ asset 3D của game cho mọi version publish, hiển thị trong CMS như dữ… | P3 | _suy luận:_ [ADMIN_PORTAL](../04-admin-portal/ADMIN_PORTAL.md) | _suy luận:_ 48 op | _suy luận:_ TC-CMS |
| FR-CMS-010 | Vận hành của mỗi tenant đăng nhập Admin Portal (workspace School & Learning Management, Learning Content St… | P3 | _suy luận:_ [ADMIN_PORTAL](../04-admin-portal/ADMIN_PORTAL.md) | _suy luận:_ 48 op | _suy luận:_ TC-CMS |
| FR-CMS-011 | Vận hành tenant không thấy và không thao tác được menu/cấu hình game; 4 lớp chặn: gateway, azp+MFA, permiss… | P3 | _suy luận:_ [ADMIN_PORTAL](../04-admin-portal/ADMIN_PORTAL.md) | _suy luận:_ 48 op | _suy luận:_ TC-CMS |
| FR-CMS-012 | Menu dựng từ GET /api/v1/me/permissions; permission catalog trong code, role tenant không có game.*/competi… | P1 | _suy luận:_ [ADMIN_PORTAL](../04-admin-portal/ADMIN_PORTAL.md) | `getMyPermissions` | _suy luận:_ TC-CMS |
| FR-CMS-013 | System Admin là người duy nhất xem dữ liệu liên tenant: dropdown tìm kiếm tenant (mặc định "Tất cả", chọn n… | P3 | _suy luận:_ [ADMIN_PORTAL](../04-admin-portal/ADMIN_PORTAL.md) | _suy luận:_ 48 op | _suy luận:_ TC-CMS |
| FR-CMS-014 | Xóa = soft-delete; dữ liệu đã xóa không hiển thị; Thùng rác cho System Admin (Tenant Admin nếu được cấp) | P2 | _suy luận:_ [ADMIN_PORTAL](../04-admin-portal/ADMIN_PORTAL.md) | _suy luận:_ 48 op | _suy luận:_ TC-CMS |
| FR-CMS-015 | System Admin gán dữ liệu platform cho tenant (chỉ đọc với tenant) và clone dữ liệu cấu hình/nội dung giữa t… | P2 (gán) – P3 (clone) | _suy luận:_ [ADMIN_PORTAL](../04-admin-portal/ADMIN_PORTAL.md) | _suy luận:_ 48 op | _suy luận:_ TC-CMS |

### 3.31. `LMS` LMS

Service: learning-service · Tài liệu sở hữu: [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md)

AC: — · US: US-LMS-001, US-LMS-002

Mở rộng ở tài liệu sở hữu: 3 FR, 14 AC, 9 US — [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md)

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-LMS-001 | Module: Course, Learning Path, Enrollment, Assignment, Schedule, Lesson Delivery, Assessment, Learning Prog… | P3 | [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md) | _suy luận:_ 21 op | _suy luận:_ TC-LMS, TC-DASH |
| FR-LMS-002 | Giáo viên giao bài tập, giao đề ôn tập (target: lớp/nhóm/danh sách HS), đặt open/due, số lần làm, hiển thị… | P3 | [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md) | _suy luận:_ 21 op | _suy luận:_ TC-LMS, TC-DASH |
| FR-LMS-003 | Giáo viên theo dõi kết quả, tiến độ, kỹ năng yếu theo LO | P3 | [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md) | _suy luận:_ 21 op | _suy luận:_ TC-LMS, TC-DASH |
| FR-LMS-004 | Giáo viên tạo nhóm học tập và tổ chức thử thách học tập | P3 (nhóm) – P4 (thử thách) | [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md) | _suy luận:_ 21 op | _suy luận:_ TC-LMS, TC-DASH |
| FR-LMS-005 | Phân quyền theo trường, lớp, môn (TeachingAssignment) | P3 | [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md) | _suy luận:_ 21 op | _suy luận:_ TC-LMS, TC-DASH |
| FR-LMS-006 | Bài được giao xuất hiện trong game như quest/nhiệm vụ đặc biệt | P3 | [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md) | _suy luận:_ 21 op | _suy luận:_ TC-LMS, TC-DASH |
| FR-LMS-007 | Giáo viên chỉ chọn quest/mini-game đã publish và được gán cho tenant để giao bài; thử thách/giải trường tạo… | P2–P4 | [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md) | _suy luận:_ 21 op | _suy luận:_ TC-LMS, TC-DASH |

### 3.32. `AUD` Audit

Service: identity-service · Tài liệu sở hữu: [SECURITY_ARCHITECTURE](../10-security/SECURITY_ARCHITECTURE.md)

Mở rộng ở tài liệu sở hữu: 0 FR, 5 AC, 0 US — [AUDIO_LANGUAGE_LEARNING](../02-learning/AUDIO_LANGUAGE_LEARNING.md)

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-AUD-001 | Audit log cho: đăng nhập admin, thay đổi quyền, truy cập dữ liệu học sinh chéo phạm vi, publish nội dung, k… | P1+ | _suy luận:_ [SECURITY_ARCHITECTURE](../10-security/SECURITY_ARCHITECTURE.md) | _suy luận:_ 27 op | _suy luận:_ TC-AUD |
| FR-AUD-002 | Audit log append-only, partition theo tháng, retention theo OQ-12 | P1 | _suy luận:_ [SECURITY_ARCHITECTURE](../10-security/SECURITY_ARCHITECTURE.md) | _suy luận:_ 27 op | _suy luận:_ TC-AUD |

### 3.33. `OPS` Vận hành

Service: pkg/ (platform kit) · mọi service · Tài liệu sở hữu: [DEPLOYMENT](../11-operations/DEPLOYMENT.md) · [MONITORING](../11-operations/MONITORING.md)

| FR | Yêu cầu | Phase | Tài liệu thiết kế | Operation | TC |
|---|---|---|---|---|---|
| FR-OPS-001 | Health, readiness, liveness endpoint cho mọi binary | P1 | _suy luận:_ [DEPLOYMENT](../11-operations/DEPLOYMENT.md) · [MONITORING](../11-operations/MONITORING.md) | _suy luận:_ 12 op | _suy luận:_ TC-SVC, TC-PERF |
| FR-OPS-002 | Docker + Docker Compose local; environment example; migration; backup script; deployment docs | P1 | _suy luận:_ [DEPLOYMENT](../11-operations/DEPLOYMENT.md) · [MONITORING](../11-operations/MONITORING.md) | _suy luận:_ 12 op | _suy luận:_ TC-SVC, TC-PERF |
| FR-OPS-003 | Metrics: online players, active sessions, matchmaking queue, match duration, API latency, error rate, chat… | P1–P6 | _suy luận:_ [DEPLOYMENT](../11-operations/DEPLOYMENT.md) · [MONITORING](../11-operations/MONITORING.md) | _suy luận:_ 12 op | _suy luận:_ TC-SVC, TC-PERF |
| FR-OPS-004 | Không commit secrets; secret qua secret manager/Docker secrets | P1 | _suy luận:_ [DEPLOYMENT](../11-operations/DEPLOYMENT.md) · [MONITORING](../11-operations/MONITORING.md) | _suy luận:_ 12 op | _suy luận:_ TC-SVC, TC-PERF |
| FR-OPS-005 | CI: lint, test, spec lint/breaking check, build Go/Godot/Web theo path | P1 | _suy luận:_ [DEPLOYMENT](../11-operations/DEPLOYMENT.md) · [MONITORING](../11-operations/MONITORING.md) | _suy luận:_ 12 op | _suy luận:_ TC-SVC, TC-PERF |

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
| AC-GAM-001 | `GAM` | — |
| AC-GAM-002 | `GAM` | — |
| AC-GAM-003 | `GAM` | — |
| AC-GAM-011 | `GAM` | — |
| AC-MOD-020 | `MOD` | — |
| AC-MOD-021 | `MOD` | — |
| AC-MOD-022 | `MOD` | — |
| AC-ORG-001 | `ORG` | — |
| AC-ORG-002 | `ORG` | — |
| AC-ORG-003 | `ORG` | — |
| AC-ORG-004 | `ORG` | — |
| AC-ORG-005 | `ORG` | — |
| AC-ORG-006 | `ORG` | — |
| AC-ORG-007 | `ORG` | — |
| AC-PRG-001 | `PRG` | — |
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

## 5. Báo cáo mồ côi

Tính trên 287 FR thuộc Release 1.0 (phase P1–P6).

| Kiểm tra | Không có liên kết tường minh | Không có cả suy luận (mồ côi thật) |
|---|---|---|
| FR không có tài liệu thiết kế | 234 | 0 |
| FR không có operation OpenAPI | 273 | 0 |
| FR không có TC | 287 | 0 |
| AC không thuộc domain FR nào | — | 0 |
| US không thuộc domain FR nào | — | 0 |
| AC chưa có TC tường minh | 93 | — |
| AC mở rộng (tài liệu sở hữu) chưa có TC tường minh | 356 / 393 | — |
| FR/AC mở rộng mang mã không phải domain FR | — | 82 |
| Mã FR/US/AC được nhắc nhưng không được định nghĩa ở đâu | — | 4 |
| Tiền tố TC không ánh xạ domain FR | — | 2 |

### 5.1. Mồ côi thật

- **FR không có tài liệu thiết kế:** Không có.
- **FR không có operation (kể cả suy luận):** Không có. — có thể hợp lệ nếu yêu cầu thuần client/realtime/hạ tầng; khi đó ghi rõ trong tài liệu sở hữu.
- **FR không có TC (kể cả suy luận):** Không có.
- **AC không thuộc FR nào:** Không có.
- **US không thuộc FR nào:** Không có.
- **FR/AC mở rộng mang mã không phải domain FR** (UX/NFR; cần gắn domain FR hoặc NFR): **PERF** 001, 002, 003, 004, 005, 006, 007, 008, 009, 010, 011 · **UX** 0001, 0002, 0003, 0004, 0005, 0006, 0007, 0008, 0101, 0102, 0103, 0104, 0105, 0106, 0107, 0108, 0109, 0110, 0201, 0202, 0203, 0204, 0301, 0302, 0303, 0304, 0401, 0402, 0403, 0404, 0501, 0502, 0503, 0504, 0601, 0602, 0603, 0604, 0605, 0701, 0702, 0703, 0704, 101, 102, 103, 104, 105, 106, 107, 108, 109, 110, 111, 112, 113, 114, 201, 202, 203, 204, 205, 206, 207, 208, 209, 210, 211, 212, 213, 214
- **Mã được nhắc nhưng không được định nghĩa ở đâu:** AC-PTY-012 (PARTY_SYSTEM) · AC-PTY-013 (PARTY_SYSTEM) · AC-PTY-014 (PARTY_SYSTEM) · AC-PTY-015 (PARTY_SYSTEM)
- **Tiền tố TC chưa ánh xạ domain:** TC-I18N (11), TC-UI (14)

### 5.2. Chỉ có liên kết suy luận (cần bổ sung liên kết tường minh)

- **FR chưa được tài liệu thiết kế nhắc mã:** **ANL** 005, 006, 007, 008, 009, 010, 011, 012, 013, 014 · **AUD** 001, 002 · **AUTH** 001, 002, 003, 004, 005, 007, 008, 009, 010, 011, 012, 013, 014, 016, 017, 018, 019, 021, 022, 023, 024, 027, 028, 029, 030, 031, 032 · **CHAT** 001, 002, 003, 004, 005, 006, 008, 009, 010, 011, 012, 013, 014, 015, 016, 017 · **CHL** 001, 002, 003 · **CMP** 001, 002, 003, 004, 005 · **CMS** 001, 002, 003, 004, 005, 006, 007, 008, 009, 010, 011, 012, 013, 014, 015, 016, 017 · **CNT** 013, 014, 015, 016, 017 · **ECO** 001, 002, 003, 004, 005 · **EVT** 001, 002, 003, 004, 005, 006, 007, 008 · **EXR** 008, 009, 010 · **GAM** 001, 002, 003, 004, 005, 006, 007, 008, 009 · **GAME** 001, 002, 003, 004, 005, 006, 007, 008, 009, 010, 011, 012, 013, 014, 015, 016, 017, 018 · **LDB** 001, 002, 003, 004 · **MG** 001, 002, 003, 004 · **MM** 001, 002, 003, 004, 005, 006 · **MOD** 001, 002, 003, 004, 005, 006, 007, 008, 009, 010, 011, 012, 013, 014, 015 · **MP** 001, 002, 003, 004, 005, 006 · **NTF** 001, 002 · **OPS** 001, 002, 003, 004, 005 · **ORG** 001, 002, 003, 004, 005, 006, 007, 008, 009, 010, 011, 012, 013, 014, 015, 016, 017, 018 · **PRG** 005 · **PTY** 001, 002, 003, 004, 005, 006, 007, 008, 009, 010, 011, 012, 013, 014 · **QST** 001, 002, 003, 004, 005, 006, 007, 008 · **RNK** 001, 002, 003, 004 · **RWD** 001, 002, 003 · **SIS** 001, 002, 003, 004, 005, 006, 007, 008, 009 · **SOC** 001, 002, 003, 004 · **TRN** 001, 002, 003
- **FR chưa có TC nhắc mã:** **ANL** 001, 002, 003, 004, 005, 006, 007, 008, 009, 010, 011, 012, 013, 014 · **AUD** 001, 002 · **AUTH** 001, 002, 003, 004, 005, 007, 008, 009, 010, 011, 012, 013, 014, 016, 017, 018, 019, 021, 022, 023, 024, 027, 028, 029, 030, 031, 032 · **CHAT** 001, 002, 003, 004, 005, 006, 008, 009, 010, 011, 012, 013, 014, 015, 016, 017 · **CHL** 001, 002, 003 · **CMP** 001, 002, 003, 004, 005 · **CMS** 001, 002, 003, 004, 005, 006, 007, 008, 009, 010, 011, 012, 013, 014, 015, 016, 017 · **CNT** 001, 002, 003, 004, 005, 006, 007, 009, 010, 011, 012, 013, 014, 015, 016, 017 · **CUR** 001, 002, 003, 004, 005, 006 · **ECO** 001, 002, 003, 004, 005 · **EVT** 001, 002, 003, 004, 005, 006, 007, 008 · **EXM** 001, 002, 003, 004, 005, 006 · **EXR** 001, 002, 003, 004, 005, 006, 007, 008, 009, 010 · **GAM** 001, 002, 003, 004, 005, 006, 007, 008, 009 · **GAME** 001, 002, 003, 004, 005, 006, 007, 008, 009, 010, 011, 012, 013, 014, 015, 016, 017, 018 · **LDB** 001, 002, 003, 004 · **LMS** 001, 002, 003, 004, 005, 006, 007 · **MG** 001, 002, 003, 004 · **MM** 001, 002, 003, 004, 005, 006 · **MOD** 001, 002, 003, 004, 005, 006, 007, 008, 009, 010, 011, 012, 013, 014, 015 · **MP** 001, 002, 003, 004, 005, 006 · **NTF** 001, 002, 003 · **OPS** 001, 002, 003, 004, 005 · **ORG** 001, 002, 003, 004, 005, 006, 007, 008, 009, 010, 011, 012, 013, 014, 015, 016, 017, 018 · **PRG** 001, 002, 003, 004, 005 · **PTY** 001, 002, 003, 004, 005, 006, 007, 008, 009, 010, 011, 012, 013, 014 · **QB** 001, 002, 003, 004, 005, 006, 007 · **QST** 001, 002, 003, 004, 005, 006, 007, 008 · **RNK** 001, 002, 003, 004 · **RWD** 001, 002, 003 · **SIS** 001, 002, 003, 004, 005, 006, 007, 008, 009 · **SOC** 001, 002, 003, 004 · **TRN** 001, 002, 003
- **AC (PRD) chưa có TC nhắc mã:** **AUTH** 001, 002, 003, 004, 005, 006, 007, 008, 009, 010, 011, 012, 013, 014, 015, 016, 017, 018, 019, 020, 021, 022, 023, 024, 025, 026 · **CHAT** 001, 002, 003, 004, 005, 006, 007, 008, 009, 010, 011, 012, 013, 014 · **CMP** 001, 002, 003, 004, 005, 006 · **CMS** 001, 002, 003, 004, 005 · **CNT** 001, 002, 003, 004, 005, 006, 007, 008 · **EXM** 001, 002 · **EXR** 001, 002, 003, 004, 005, 006 · **GAM** 001, 002, 003, 011 · **MOD** 020, 021, 022 · **ORG** 001, 002, 003, 004, 005, 006, 007 · **PRG** 001 · **PTY** 001, 002, 003, 004, 005, 006, 007, 008, 009, 010, 011
- **AC mở rộng chưa có TC nhắc mã:** **ACT** 001, 002, 003, 004, 005, 006 · **ANL** 102, 109 · **AUD** 001, 002, 003, 004, 005 · **AUTH** 101, 102, 103, 104, 105, 106, 107, 108, 109, 110, 111, 112, 113, 114, 115, 116, 117, 118, 120, 121, 122, 123, 124, 125, 127, 128, 130, 131, 140, 141, 142, 143, 144, 145, 150, 151, 152, 153, 154, 155, 160, 161, 162, 163, 164, 165, 166, 170, 171, 172, 173, 174 · **CHAT** 015, 016, 017, 018, 019, 020, 021, 022, 023, 024, 025, 026, 027, 028, 029, 030, 031, 032, 033, 034, 035, 036, 037, 038, 039, 040, 041, 042, 043 · **CHL** 001, 002, 003, 004, 005, 006, 007, 008 · **CMP** 101, 102, 201, 202, 203, 204, 205, 206, 207, 208, 209, 210 · **DASH** 005, 008 · **ECO** 001, 002, 003, 004, 005, 006, 007, 008 · **EXM** 003, 004, 005 · **GAM** 004, 005, 006, 007, 008, 009, 010 · **GDD** 001, 002, 003, 004, 005 · **LDB** 001, 002, 003, 004, 005, 006 · **LMS** 001, 002, 003, 004, 005, 006, 010, 011, 012, 013, 014, 015, 016, 017 · **MG** 001, 002, 003, 004, 005, 006 · **MM** 001, 002, 003, 004, 005, 006, 007, 008 · **MOD** 001, 002, 003, 004, 005, 006, 007, 008, 009, 010, 011, 012, 013, 014, 015, 016, 017, 018, 019, 023, 024, 025 · **MP** 001, 002, 003, 004, 005, 006, 007, 008, 009, 010, 011 · **ORG** 008, 009, 010, 011, 012, 013, 014, 015, 016, 017, 018, 019, 020, 021, 022, 023, 024 · **PERF** 002, 003, 008 · **QST** 001, 002, 003, 004, 005, 006, 007, 008, 009, 010, 011, 012, 013 · **RNK** 001, 002, 003, 004, 005, 006 · **SIM** 001, 002, 003, 004, 005 · **SOC** 001, 002, 003, 004, 005, 006, 007, 008, 009, 010 · **TRN** 001, 002, 003, 004, 005, 006, 007, 008 · **UX** 0001, 0002, 0003, 0004, 0005, 0006, 0007, 0008, 0101, 0102, 0103, 0104, 0105, 0106, 0107, 0108, 0109, 0110, 0201, 0202, 0203, 0204, 0301, 0302, 0303, 0304, 0401, 0402, 0403, 0404, 0501, 0502, 0503, 0504, 0601, 0602, 0603, 0604, 0605, 0701, 0702, 0703, 0704, 101, 102, 103, 104, 105, 106, 107, 108, 109, 110, 111, 112, 113, 114, 201, 202, 203, 204, 205, 206, 207, 208, 209, 210, 211, 212, 213, 214 · **VID** 001, 002, 003, 004, 005 · **WLD** 001, 002, 003, 004 · **WR** 001, 002, 003, 004, 005, 006, 007, 008
