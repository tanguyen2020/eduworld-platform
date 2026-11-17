# Research: F022 — Question Bank 8 loại cơ bản: validator, scorer server-side, render payload không đáp án

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## R1. Số thập phân khi chấm `numeric`

- **Decision**: Dùng `github.com/cockroachdb/apd/v3` (decimal tùy ý độ chính xác) trong `internal/domain/question/numeric`; parse chuỗi học sinh thành `apd.Decimal`, so sánh dung sai bằng phép trừ decimal.
- **Rationale**: QUESTION_BANK §3.6 yêu cầu "parse về decimal, không float"; apd hỗ trợ context làm tròn tường minh, không phụ thuộc cgo. Domain được phép import thư viện thuần không I/O (cần ghi ngoại lệ trong depguard cho `domain`).
- **Alternatives**: `shopspring/decimal` (phổ biến nhưng phép chia mặc định 16 chữ số ngầm); `math/big.Rat` (đủ chính xác nhưng parse dấu thập phân/ngăn nghìn phải tự viết nhiều hơn).

## R2. Chuẩn hóa văn bản tiếng Việt

- **Decision**: `golang.org/x/text/unicode/norm` (NFC/NFD), `golang.org/x/text/cases` với `language.Vietnamese` cho chữ thường; bảng ánh xạ dấu thanh kiểu cũ → mới (`oà→òa`, `uỷ→ủy`…) khai báo hằng trong `internal/domain/question/normalize/tone_table.go`; bỏ dấu bằng NFD + loại combining mark (giữ `đ→d` tường minh).
- **Rationale**: Đủ 8 bước của QUESTION_BANK §4; bảng dấu thanh là dữ liệu chuẩn chính tả, không phải nội dung học tập (không vi phạm R2).
- **Alternatives**: Regex tự viết (dễ sót tổ hợp); thư viện bỏ dấu bên ngoài (không chuẩn hóa vị trí dấu).

## R3. Trộn thứ tự tái lập

- **Decision**: `math/rand/v2` với nguồn `rand.NewPCG(seed, hash(question_version_id))`; Fisher–Yates; với `ordering` lặp lại tối đa 10 lần đến khi khác thứ tự đúng, hết lần thì xoay vòng một vị trí.
- **Rationale**: Tái lập theo `attempt.seed` (QUESTION_BANK §6 bước 3) và mỗi câu một luồng độc lập; PCG ổn định giữa phiên bản Go.
- **Alternatives**: `crypto/rand` (không tái lập); `math/rand` v1 (nguồn toàn cục, khó cô lập).

## R4. Tách phần chỉ server khi lưu

- **Decision**: Validator nhận dạng soạn theo `question.schema.json`; mapper `authoring → storage` chuyển `render_payload.options[].feedback|misconception_tag` sang cột `option_feedback` (map `option_id → {feedback, misconception_tag}`); mapper ngược khi `getCmsQuestion`.
- **Rationale**: Giữ contract soạn thảo hiện có, đồng thời cột `render_payload` đúng nghĩa "không có đáp án" (DATABASE_DESIGN §3.3) để view delivery an toàn mặc định.
- **Alternatives**: Lưu nguyên và lọc khi đọc (một lần quên lọc là lộ đáp án — trái ADR-0016).
