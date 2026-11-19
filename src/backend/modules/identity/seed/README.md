# Seed — module identity

Dữ liệu giả cho local ([LOCAL_DEVELOPMENT §5](../../../../../docs/11-operations/LOCAL_DEVELOPMENT.md)), nạp bằng `make seed-identity` (`go -C src/backend run ./modules/identity/cmd/seed`, chỉ chạy khi `EW_ENV=local`).

- `dev.yaml` — tenant, trường, nhân sự platform (mục đầu là System Admin bootstrap `sa.admin`), nhân sự tenant + role, học sinh bị khóa. Username phải là user của realm import `deploy/keycloak/realm-eduworld.dev.json` (test `internal/presentation/devseed` kiểm tra khớp realm role và Organization).
- `sis/<mã trường>/{classes,students,enrollments}.csv` — file import CSV của SIS (cột = tên trường DTO của EduWorld SIS Contract v1, `login_username` = username trong realm import); seed chạy qua đúng pipeline `SisRun` của job import CSV.

Mọi thao tác đi qua use case của module (không SQL tay); tài khoản Keycloak được gắn với user sẵn có của realm import (`keycloak.DevAdopter`), không đặt mật khẩu. Chạy lại không thay đổi gì. Không seed vào schema `ref` (projection nạp qua event, kiểm bằng `make seed-check`).
