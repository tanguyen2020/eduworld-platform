# Research: F027 — Media pipeline, công thức SVG, preview session

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## R1. Công cụ xử lý media trong worker

- **Decision**: Gói dùng chung `src/backend/pkg/media` (ADR-0044 mục 3) với interface `Scanner`, `MagicSniffer`, `ExifStripper`, `Transcoder`. Hiện thực: ClamAV qua giao thức `clamd` TCP (`INSTREAM`); magic bytes bằng `net/http.DetectContentType` + bảng chữ ký bổ sung (WebP, OGG, MP4 `ftyp`, WebM EBML, GLB); ảnh bằng libvips (binding Go) — xuất WebP + PNG 256/1024/2048 px, không giữ metadata; âm thanh/video bằng `ffmpeg` (process con, timeout) → Ogg Vorbis + MP3; Ogg Theora `.ogv` + HLS 360p/720p + MP4.
- **Rationale**: libvips và ffmpeg là chuẩn thực tế, hỗ trợ đủ định dạng Godot cần (Vorbis, Theora); `clamd` có sẵn image `clamav/clamav` trong compose.
- **Alternatives**: thư viện Go thuần (`imaging`) — chậm, không có WebP encode; dịch vụ transcode SaaS — vi phạm ADR-0071 (floci, tự vận hành).

## R2. Sanitize SVG công thức

- **Decision**: Parse XML bằng `encoding/xml` streaming, allowlist phần tử (`svg`, `g`, `path`, `rect`, `line`, `use` nội bộ, `defs`, `title`, `desc`) và thuộc tính hình học/`fill`/`stroke`/`transform`/`viewBox`/`href` chỉ dạng `#id`; loại `script`, `foreignObject`, mọi `on*`, `style` chứa `url(`, tham chiếu ngoài; tính `sha256` trên bản đã sanitize.
- **Rationale**: SVG MathJax 4 chỉ dùng `path`/`use`/`g`; allowlist nhỏ dễ kiểm và test (TC-MATH-002).
- **Alternatives**: sanitize HTML chung (bluemonday) — không hiểu namespace SVG; render lại phía server — trái ADR-0055 (không worker render).

## R3. Kiểm tra LaTeX

- **Decision**: Tokenizer LaTeX tối giản trong `domain/mathx`: đếm độ sâu `{}`/`\left…\right`, chặn lệnh trong danh sách cấm ADR-0055 mục 7 và `\def`, `\newcommand`, `\renewcommand`, `\let`; chỉ cho phép môi trường trong allowlist (`ams`, `mhchem` `\ce`/`\pu`, `color` giới hạn).
- **Rationale**: Không cần render phía server; kiểm tra cú pháp an toàn là đủ, đúng ranh giới domain thuần (R13).
- **Alternatives**: chạy MathJax trong Node phía server — thêm runtime, trái ADR-0055 mục 4.
