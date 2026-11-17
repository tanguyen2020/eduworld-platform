# Research: F053 — Workspace B — cây curriculum, editor nội dung, công thức, song ngữ, trình soạn câu hỏi

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## R1. Render công thức trên trình duyệt

- **Decision**: Nạp MathJax 4 (component TeX input + SVG output, `fontCache: 'none'` để SVG tự chứa, extension `ams`, `mhchem`; tắt `require`, `html`, `href`, `unicode` macro tùy ý). Với mỗi công thức: `tex2svgPromise(latex, { display })` → lấy `outerHTML` của `<svg>`, thay mọi màu bằng `currentColor`; speech text từ speech-rule-engine của MathJax 4 (locale theo `content_language`). `math_id` = sha256 hex của `display + latex` (≤ 64). Kiểm `latex_max_length`, `latex_max_depth`, `math_svg_max` trước khi gửi.
- **Rationale**: [CONTENT_MODEL §9.1](../../docs/02-learning/CONTENT_MODEL.md), ADR-0055 (render trên trình duyệt, không worker thường trực); game chỉ hiển thị SVG.
- **Alternatives**: KaTeX — loại (ADR-0055 chọn MathJax 4, cần `mhchem` + speech). Render phía server — loại theo ADR-0055.

## R2. Validation client bằng `question.schema.json`

- **Decision**: Script `pnpm gen:schemas` sao chép `specs/schemas/question.schema.json` (và `$defs`) vào `web/apps/portal/src/api/schemas/` (file sinh, không sửa tay, CI kiểm khớp). `schemaValidator.ts` dùng ajv 2020-12 kiểm payload trước khi gửi; lỗi map về field path giống `422 VALIDATION_FAILED`.
- **Rationale**: Một nguồn sự thật (R33); báo lỗi sớm trên form; server vẫn kiểm (F022).
- **Alternatives**: Tự viết Zod cho từng dạng — loại vì lặp quy tắc và lệch schema.

## R3. Autosave và khóa lạc quan

- **Decision**: `useAutosave` debounce sau khi ngừng gõ và ép lưu theo chu kỳ của [ADMIN_PORTAL §1.4](../../docs/04-admin-portal/ADMIN_PORTAL.md); một request lưu tại một thời điểm; gửi `If-Match` = `ETag` gần nhất; `412`/`409 CONTENT_VERSION_CONFLICT` → dừng autosave, giữ bản cục bộ, hiện `ConflictBanner` (Tải bản mới / Sao chép thay đổi của tôi). Render công thức chạy trước khi gửi lưu (chỉ công thức thay đổi, cache theo `math_id`).
- **Rationale**: F053-AC-06, F053-AC-07; TC-CNT-003.
- **Alternatives**: Khóa bi quan (lock item) — loại vì contract dùng ETag.

## R4. Sinh `itemId`

- **Decision**: `itemId(prefix)` = `prefix + '_' + base36(crypto.getRandomValues(4 byte))`, kiểm pattern `^[a-z]_[a-z0-9_]{1,24}$` và không trùng trong version; prefix theo loại (`o` option, `b` blank, `z` zone, `p` pair, `s` step).
- **Rationale**: [QUESTION_BANK §2](../../docs/02-learning/QUESTION_BANK.md) — ID không tiết lộ thứ tự.
- **Alternatives**: UUID — quá dài so với pattern; chỉ số — lộ thứ tự.
