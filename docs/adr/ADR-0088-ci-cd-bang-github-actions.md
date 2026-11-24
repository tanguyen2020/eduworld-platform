# ADR-0088 — CI/CD bằng GitHub Actions

> **Trạng thái:** Accepted · Superseded một phần bởi [ADR-0101](ADR-0101-bo-windows-khoi-nen-tang-client-release-1-0.md) (export Windows) · **Ngày:** 2026-11-17 · **OQ:** — · **Tài liệu:** [DEPLOYMENT](../11-operations/DEPLOYMENT.md) · [LOCAL_DEVELOPMENT](../11-operations/LOCAL_DEVELOPMENT.md) · [TEST_STRATEGY](../12-project/TEST_STRATEGY.md) · [CODE_PLAN](../12-project/CODE_PLAN_CMS_LMS_FIRST.md) · [Chỉ mục ADR](README.md)

> [!WARNING]
> **Superseded một phần bởi [ADR-0101](ADR-0101-bo-windows-khoi-nen-tang-client-release-1-0.md) (bỏ Windows, 2026-11-24).** Không còn hiệu lực: export Windows trong `godot.yml` (mục 1) và "giữ đủ 4 nền tảng" ở mục 3. **Còn hiệu lực:** phần còn lại; `godot.yml` export Android/Web trên runner Linux, export và ký iOS trên runner macOS.

## Bối cảnh

Repository nằm trên GitHub, nhưng nhiều tài liệu và feature spec (F001, F004, F005, ROADMAP, DEPLOYMENT, GODOT_ARCHITECTURE) mô tả pipeline GitLab CI (`.gitlab-ci.yml`, GitLab Runner, GitLab Container Registry). Review kiến trúc ngày 2026-11-17 ghi nhận mâu thuẫn này. Technical Architect chọn GitHub Actions.

## Quyết định

1. **Nền tảng CI/CD:** GitHub Actions. Workflow nằm trong `.github/workflows/`; mỗi lane là một workflow riêng, chạy theo `paths` filter:
   - `go.yml`: ma trận theo service/module (`gen-check`, `lint`, `test`, `test-integration` với testcontainers, `migrate-lint`).
   - `specs.yml`: `make specs-lint`, `make specs-breaking`, `node scripts/specs/check-features.mjs`, script kiểm tra liên feature.
   - `docs.yml`: `make docs-check` (link, mermaid, RTM).
   - `web.yml`: lint, test, Storybook, visual regression (Playwright).
   - `design.yml`: `make design-tokens-check`.
   - `godot.yml`: GdUnit4 headless, export Windows/Android/Web trên runner Linux; export và ký iOS trên runner macOS.
   - `release.yml`: build image, quét lỗ hổng, ký, đẩy registry, triển khai staging theo tag.
2. **Registry:** GitHub Container Registry (`ghcr.io`) cho image backend và web.
3. **Runner:** GitHub-hosted cho lane thường; macOS runner cho iOS (ADR-0013 giữ đủ 4 nền tảng). Self-hosted runner chỉ cho bước triển khai vào hạ tầng nội bộ (Docker Swarm), đặt trong mạng quản trị, không nhận job từ fork.
4. **Secret:** GitHub Environments (`staging`, `production`) có required reviewers; secret ký Android/iOS và token triển khai chỉ cấp cho environment tương ứng. Không có secret trong repo (R4). Truy cập cloud dùng OIDC khi nhà cung cấp hỗ trợ.
5. **Cổng merge:** branch protection trên `main` bắt buộc các lane liên quan pass, có review CODEOWNERS; `make check` là tổng hợp chạy cục bộ tương đương.
6. **Chuỗi cung ứng:** pin action theo commit SHA, Dependabot cho action/Go/npm, SBOM và ký image (cosign) trong `release.yml`.

## Hệ quả

(+) Cùng nền tảng với repository; có runner macOS cho iOS; secret theo environment. (−) Phải sửa F001, F004, F005 và các tài liệu còn nhắc GitLab; build iOS trên runner macOS tốn phút tính phí; triển khai vào Swarm nội bộ cần self-hosted runner được bảo vệ.

## Phương án đã loại

- **GitLab CI:** phải mirror repository hoặc chuyển nơi lưu code.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-17 | Tạo ADR (Accepted) | Technical Architect |
| 2026-11-24 | Superseded một phần bởi ADR-0101 (bỏ export Windows) | Owner dự án |
