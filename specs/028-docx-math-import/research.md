# Research: F028 — Import Word/MathType (`lessons_docx`) qua Pandoc

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · **Ngày spike**: 2026-11-27 (time-box 1 ngày)

Spike theo yêu cầu owner ("spike trước khi mở cho người soạn", [CONTENT_PRODUCTION_PLAN §8](../../docs/02-learning/CONTENT_PRODUCTION_PLAN.md#8-import-wordmathtype-confirmed)). Bộ mẫu do script sinh (`docxfixture`, nội dung toán tiếng Việt giữ chỗ, không lấy từ sách giáo khoa); Pandoc chạy trong image `eduworld/media-tools` dựng từ stage `media-tools` của `deploy/docker/worker.Dockerfile` (Alpine 3.22, `pandoc-cli` 3.6.4).

## R1. Công cụ chuyển đổi và cách chạy

- **Decision**: Pandoc 3.6.4 (gói `pandoc-cli` của Alpine 3.22) là tiến trình ngoài: `pandoc --sandbox -f docx -t json +RTS -M<docx_convert_memory> -RTS`, file `.docx` đưa qua stdin, JSON AST đọc từ stdout (giới hạn byte), `context` có timeout `docx_convert_timeout`, môi trường rỗng, thư mục làm việc tạm rỗng; không ghi file, không `--extract-media`. Ảnh nhúng đọc thẳng từ zip bằng Go (`archive/zip`). Pandoc nằm trong stage `media-tools` (= image `eduworld/media-tools`, cùng ffmpeg); image `worker` dựng trên stage đó nên job `import-jobs` (profile `jobs`) có sẵn `pandoc` trên `PATH`.
- **Rationale**: Owner chốt Pandoc (CONTENT_PRODUCTION_PLAN §8). `--sandbox` chặn mọi IO ngoài input/output (không tải ảnh/link ngoài); `+RTS -M` giới hạn heap — đã thử chạy được trên bản Alpine (Pandoc build cho phép RTS options). Đo: tài liệu 50 câu (124 KB JSON) chuyển trong ~1,5 s gồm khởi động container. Lỗi rõ ràng: zip hỏng → `couldn't unpack docx container`, zip không phải docx → `couldn't parse docx file: DocxError` (exit ≠ 0).
- **Alternatives**: thư viện Go đọc OOXML + tự chuyển OMML (không có thư viện OMML→LaTeX đáng tin; trái quyết định owner); LibreOffice headless (nặng, chạy macro được, rủi ro bảo mật); image riêng chạy như service HTTP (thêm service, trái DEPLOYMENT §2 "image phụ", không cần khi tải nhỏ).

## R2. OMML → LaTeX: cái gì map sạch

Bộ 22 cấu trúc OMML (`docxfixture.Formulas()`), JSON AST ghi ở `src/backend/modules/learning/internal/infrastructure/docximport/testdata/pandoc-3.6.4-formulas.json`:

| Cấu trúc | LaTeX Pandoc 3.6.4 | allowlist server (`mathx.ValidateLatex`) | MathJax 4 portal |
|---|---|---|---|
| phân số, hỗn số, phân số lồng | `\frac{3}{4}`, `2\frac{1}{3}`, `\frac{1}{1 + \frac{1}{x}}` | đạt | đạt |
| căn bậc 2, bậc 3 | `\sqrt{x + 1}`, `\sqrt[3]{8}` | đạt | đạt |
| lũy thừa, chỉ số, hóa học | `x^{2} + y^{3}`, `a_{n} = a_{1} + (n - 1)d`, `H_{2}O` | đạt | đạt |
| ma trận 2×2 | `\begin{pmatrix} 1 & 2 \\ 3 & 4 \end{pmatrix}` (có xuống dòng) | đạt | đạt |
| hệ phương trình (`m:eqArr`) | `\left\{ \begin{array}{r} … \end{array} \right.\ ` | đạt sau chuẩn hóa (bỏ `\ ` cuối) | đạt |
| tổng, tích phân, giới hạn | `\sum_{i = 1}^{n}i`, `\int_{0}^{1}{x^{2}dx}`, `\lim_{x \rightarrow 0}\frac{\sin x}{x}` | đạt | đạt |
| vector, gạch trên, hàm | `\overrightarrow{AB}`, `\overline{abc}`, `\sin x` | đạt | đạt |
| quan hệ, tập số, Hy Lạp, độ | `a \leq b,\ …\ x \in {\mathbb{R}}`, `\alpha + \beta = 90{^\circ}` | đạt | đạt |
| ±, ×, ÷, %, ngoặc co giãn | `\pm 3 \times 4 \div 2`, `25\%`, `\left( \frac{a}{b} \right)^{2}` | đạt | đạt |
| chữ trong công thức (`m:nor`) | `\frac{\text{quãng đường}}{\text{thời gian}}` | đạt | đạt |

Kết quả: **22/22** cấu trúc chuyển đúng ngữ nghĩa, qua allowlist server và render MathJax 4 (test `pandocLatex.test.ts` của portal, `TestPandocLatexAllowlist` của domain). Quan sát: Pandoc dùng `\left\{ … \right.` cho ngoặc một phía, thêm `\ ` (control space) cuối công thức hệ, xuống dòng trong môi trường ma trận, `{^\circ}` cho độ — server chuẩn hóa: xuống dòng → khoảng trắng, bỏ `\ ` và khoảng trắng đầu/cuối.

- **Decision**: công thức sau Pandoc chỉ qua **allowlist chung** `mathx.ValidateLatex` (một nguồn quy tắc — R33, ADR-0055 §7), không thêm allowlist riêng cho import; công thức bị từ chối → câu lỗi `MATH_NOT_ALLOWED` (không tạo Draft). SVG render ở portal như mọi công thức (ADR-0055 §4).

## R3. Cái phải từ chối

| Đầu vào | Pandoc làm gì | Quyết định |
|---|---|---|
| MathType OLE (`w:object` + `o:OLEObject ProgID="Equation.DSMT4"`, ảnh xem trước WMF) | `Image` trỏ `media/<x>.wmf`, không có công thức | Câu lỗi `FORMULA_NEEDS_REWRITE` ("cần soạn lại công thức": dùng *Convert Equations* của MathType/Word sang Office Math rồi import lại). OOXML không mang MathML của OLE nên không có đường "MathML nhúng" để dùng. Nhận diện bằng quét `document.xml` + `document.xml.rels` trong Go |
| Ảnh GIF/WMF/EMF khác | `Image` | Câu lỗi `IMAGE_TYPE_NOT_ALLOWED` (media `image` chỉ nhận PNG/JPEG/WebP/SVG — CONTENT_MODEL §7) |
| Ảnh không có alt text (`wp:docPr/@descr` rỗng) | `Image` alt rỗng | Câu lỗi `IMAGE_ALT_MISSING` (media ảnh bắt buộc alt — F027-AC-02) |
| Liên kết ngoài (`w:hyperlink` external) | `Link` | Giữ chữ, bỏ link, cảnh báo `EXTERNAL_LINK_REMOVED` |
| Macro (`word/vbaProject.bin`, content type `macroEnabled`) | bỏ qua | **Từ chối cả file** `MACRO_NOT_ALLOWED` (file `.docx` hợp lệ không chứa macro; file có macro là `.docm` đổi đuôi) |
| Bảng | `Table` | Câu lỗi `UNSUPPORTED_CONTENT` (không đoán cấu trúc bảng; soạn trong editor) |
| Danh sách tự đánh số của Word | `OrderedList`/`BulletList` (mất chữ "Câu n."/"A.") | Làm phẳng thành đoạn, cảnh báo `LIST_FLATTENED`; mẫu yêu cầu gõ tay "Câu n.", "A." |
| Chú thích chân trang, raw OOXML | `Note`, `RawInline` | Bỏ, cảnh báo `FOOTNOTE_REMOVED` |
| Chỉ số trên/dưới định dạng chữ (x², H₂ gõ bằng định dạng) | `Superscript`/`Subscript` | Đổi thành công thức inline `\(^{…}\)`/`\(_{…}\)`, cảnh báo `TEXT_SCRIPT_CONVERTED` |
| Zip hỏng, không phải docx, zip bomb | lỗi/treo | Kiểm trước bằng Go: kích thước ≤ `csv_import_max`, số entry ≤ `docx_zip_entries_max`, tổng giải nén ≤ `docx_uncompressed_max`, có `word/document.xml`; lỗi → job `failed` `INVALID_FORMAT`/`DOCX_TOO_COMPLEX` |

## R4. Mẫu tài liệu (template) và cách tách câu

- **Decision**: Mẫu theo CONTENT_PRODUCTION_PLAN §8 bước 1, đọc trên JSON AST (không phụ thuộc style Word):
  - Khối đầu file (trước câu đầu tiên): `Môn:` (mã môn), `Khối:` (một hoặc nhiều khối, phân tách dấu phẩy), `Chương trình:` (mã curriculum), `Phiên bản chương trình:` (tùy chọn), `Chủ đề:` (mã topic), `Ngôn ngữ:` (`vi`/`en`, mặc định `vi`).
  - Mỗi câu bắt đầu bằng đoạn `Câu <n>.` hoặc `Câu <n>:`; các đoạn sau đến câu kế thuộc câu đó.
  - Phương án `A.`…`H.` (hoặc `A)`), đáp án đúng đánh dấu bằng **chữ cái in đậm** hoặc dòng `Đáp án: B` / `Đáp án: A, C` (cả hai mà khác nhau → `ANSWER_CONFLICT`). Một đáp án → `single_choice`; nhiều → `multiple_choice` (≥ 3 phương án).
  - `Gợi ý <k>:` (≤ 5), `Giải thích:`, `LO:` (mã LO, phân tách dấu phẩy), `Độ khó:` (1–5), `Chủ đề:` trong câu ghi đè khối đầu.
  - Đoạn không có nhãn nối vào phần đang mở (đề, phương án, gợi ý, giải thích).
- **Rationale**: Hai loại câu chọn đáp án là định dạng giáo viên dùng nhiều nhất trong Word; loại câu khác (điền khuyết, ghép cặp…) cần cấu trúc mà văn bản tự do không biểu diễn chắc chắn — báo `UNSUPPORTED_QUESTION_FORMAT`, soạn trong editor F053.
- **Alternatives**: dựa style Word ("Heading 2" = câu) — giáo viên ít dùng style; bảng Word một câu một bảng — khó soạn công thức trong ô, Pandoc làm phẳng bảng phức tạp.

## R5. Hai giai đoạn: phân tích trên server, render công thức trên portal

- **Decision**: (1) Job `lessons_docx` chỉ **phân tích**: chuyển Pandoc, tách câu, kiểm allowlist, phân giải mã môn/curriculum/topic/LO, lưu từng câu với trạng thái và vấn đề vào `content.import_items`; không tạo câu hỏi (cờ `dry_run` không đổi hành vi). (2) Portal lấy danh sách câu, render mỗi công thức bằng MathJax 4 (SVG + speech) và gọi `createDraftFromImportItem` cho từng câu nhập được; server ghép SVG vào nội dung đã phân tích (client không gửi lại nội dung câu), chạy đúng validator của `createQuestion`, tạo version 1 `draft`, ghi trạng thái câu `created` hoặc `draft_failed`.
- **Rationale**: ADR-0055 §4 — SVG chỉ render trên trình duyệt, không có math worker phía server; "Import hàng loạt đi qua màn Import của portal". Mọi công thức của câu bắt buộc có SVG khi lưu (registry F022/F027), nên Draft không thể tạo ngay trong job. Nội dung câu nằm phía server từ lúc phân tích → client không sửa được đề/đáp án (R5).
- **Alternatives**: Node/MathJax trong worker (bị ADR-0055 loại); portal tự dựng file `content_json` rồi chạy import F024 (client gửi lại toàn bộ nội dung, mất liên kết báo cáo từng câu với file Word).

## R6. Ảnh nhúng

- **Decision**: Ảnh PNG/JPEG/WebP có alt được đọc từ zip khi tạo Draft của câu (giai đoạn 2) và đưa vào media pipeline F027 như một upload server-side: object vào `ew-media-quarantine`, asset `processing` + event `media.uploaded` (quét virus, biến thể như mọi ảnh), trùng sha256 trong cùng phạm vi dùng lại asset có sẵn; giấy phép (`license`) lấy từ file Word đã upload. Nội dung câu dùng `media_ref` giữ chỗ, thay bằng id asset thật trước khi validate.
- **Rationale**: CONTENT_PRODUCTION_PLAN §8 bước 2 ("ảnh nhúng đưa vào media pipeline"); không tạo asset cho câu không được nhập.
- **Alternatives**: tạo asset ngay khi phân tích (ghi dữ liệu cho câu có thể bị bỏ); bắt người soạn tải lại ảnh (mất nội dung).

## R7. Tiêu chí nghiệm thu trên bộ mẫu thật

- **Decision**: Bộ mẫu tổng hợp đạt 22/22 cấu trúc; tiêu chí "tỉ lệ chuyển đổi đúng trên bộ mẫu thật Toán/Lý/Hóa" (CONTENT_PRODUCTION_PLAN §8) chốt **≥ 95% công thức OMML chuyển đúng ngữ nghĩa và render được, 0 câu bị nhập sai đáp án** [owner chốt 2026-11-27 theo lệnh "chốt hết", mức an toàn]; đo bằng quickstart §2.6 trên file thật do EdTech cung cấp (không commit vào repo — bản quyền) trước khi mở cho người soạn. Đây là việc cần môi trường/dữ liệu thật nên để mở (F028-AC-16).
