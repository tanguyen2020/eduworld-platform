# Bộ conformance test EduWorld SIS Contract v1

Dành cho trường / nhà cung cấp SIS dựng facade theo [EduWorld SIS Contract v1](../../sis/eduworld-sis-contract.v1.yaml)
(FR-SIS-009, [SIS_INTEGRATION §2.6](../../../docs/09-identity/SIS_INTEGRATION.md)). Chạy bộ test vào facade trên môi
trường test tới khi xanh, rồi gửi kết quả cho EduWorld để đăng ký connection `contract_v1`.

| File | Nội dung |
|---|---|
| `sis-conformance.postman_collection.json` | Collection **sinh từ contract** (không sửa tay) — `node specs/postman/sis-conformance/generate.mjs` |
| `env.template.postman_environment.json` | Môi trường mẫu cho facade của trường |
| `env.sis-mock.postman_environment.json` | Môi trường của `sis-mock` (Prism, local/CI) |
| `generate.mjs` | Bộ sinh collection; `--check` báo lỗi khi collection lệch contract (CI) |

## Biến môi trường

| Biến | Ý nghĩa |
|---|---|
| `sis_base_url` | Base URL facade (đến `/eduworld/v1`, ví dụ `https://sis-test.truong.example/eduworld/v1`) |
| `auth_type` | `oauth2_client_credentials` · `api_key` · `mtls` |
| `api_key` / `access_token` | Credential (đánh dấu secret, không commit). OAuth2: lấy token client credentials trước rồi điền `access_token`; mTLS: truyền `--ssl-client-cert`/`--ssl-client-key` cho Postman CLI |
| `school_external_id`, `academic_year` | Một trường và năm học hiện hành có dữ liệu thử |
| `page_limit` | `limit` gửi khi lấy danh sách (≤ 1000) |
| `lookup_username` | Username học sinh có thật trong facade (capability `student_lookup`) |
| `expired_watermark` | Một watermark đã hết hạn mà facade trả `410` (capability `delta`) |
| `eduworld_webhook_url` | Để trống, hoặc URL webhook EduWorld do kỹ sư EduWorld cấp khi thử nghiệm push |
| `mock_prefer` | `true` chỉ với `sis-mock`: thêm header `Prefer: code=...` để Prism trả 401/404/410 |

## Chạy

```bash
postman collection run specs/postman/sis-conformance/sis-conformance.postman_collection.json \
  -e specs/postman/sis-conformance/env.template.postman_environment.json \
  --env-var "sis_base_url=https://sis-test.truong.example/eduworld/v1" --env-var "api_key=<khóa>"
```

## Bộ test kiểm gì

- **Bắt buộc** (`full_sync`): `getHealth` (JSON Schema, `contract_version` major 1, ghi lại `capabilities`), `401`
  `application/problem+json` khi thiếu credential, `listSchools`, `listAcademicYears` (đúng một năm `is_current`),
  `listClasses`, `listStudents` (đi tiếp trang theo `next_cursor`), `listEnrollments`: status, JSON Schema của
  response, phân trang cursor (`has_more` ⇒ `next_cursor` mới, số bản ghi ≤ `limit`), `limit > 1000` → `400` problem+json.
- **Tùy chọn theo capability** khai báo trong `getHealth`: `delta` → `listChanges` (thứ tự `sequence`, `record` khi
  upsert, `410` với watermark hết hạn); `student_lookup` → `lookupStudent` (200 trong 3 giây, 404 problem+json);
  `webhook` → kiểm thuật toán chữ ký `X-EduWorld-Signature = sha256=hex(HMAC_SHA256(secret, timestamp + "." + raw_body))`
  với vector của EduWorld. Capability không khai báo → request được **bỏ qua** (skipped), không fail.

Facade thiếu `/health` hoặc không trả lời: mọi request bắt buộc fail, các request tùy chọn bị bỏ qua.
