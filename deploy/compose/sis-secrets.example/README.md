# Secret SIS mẫu (chỉ local, F017 T003)

Mẫu cho connection `contract_v1` demo của TH Demo A trỏ vào `sis-mock` (Prism không kiểm giá trị khóa).
Sao chép sang thư mục secret thật (đã nằm trong `.gitignore`, không commit) rồi đặt `EW_SIS_SECRETS_DIR`
của worker trỏ tới thư mục đó:

```bash
mkdir -p deploy/compose/secrets && cp -r deploy/compose/sis-secrets.example/sis deploy/compose/secrets/
```

| `secret_ref` | File | Nội dung |
|---|---|---|
| `sis/th-demo-a` | `secrets/sis/th-demo-a` | API key (`auth_type = api_key`); OAuth2: JSON `{"client_id","client_secret"}`; mTLS: PEM chứng chỉ + khóa |
| `sis/th-demo-a-webhook` | `secrets/sis/th-demo-a-webhook` | Secret HMAC của webhook (`webhook_secret_ref`) |

Staging/production dùng Docker Swarm secret mount `/run/secrets/<secret_ref>`; API không bao giờ trả các giá trị này (F017-AC-04).
