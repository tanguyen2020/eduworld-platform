# Content Model

> Trạng thái: Draft · Phiên bản: 0.4 · Owner: Backend Architect + EdTech Specialist · Cập nhật: 2026-11-16
> Liên quan: [CURRICULUM_MODEL](CURRICULUM_MODEL.md) · [QUESTION_BANK](QUESTION_BANK.md) · [ADMIN_PORTAL](../04-admin-portal/ADMIN_PORTAL.md) · [ADR-0015](../adr/ADR-0015-content-versioning-bat-bien-sau-publish.md) · [ADR-0016](../adr/ADR-0016-server-authoritative-scoring-answer-key-khong-bao-gio-roi-se.md) · [ADR-0043](../adr/ADR-0043-phat-hanh-noi-dung-draft-publish-con-tro-phien-ban-giu-phien.md) · [ADR-0044](../adr/ADR-0044-luu-tru-hinh-anh-va-media-object-storage-s3-compatible-cdn.md) · [ADR-0072](../adr/ADR-0072-bo-vai-tro-giao-vien-hoc-sinh-tu-hoc-kiem-duyet-tap-trung.md) · [CONTENT_PRODUCTION_PLAN](CONTENT_PRODUCTION_PLAN.md)

## 1. Phạm vi

module `learning` (database `eduworld_learning`) sở hữu mọi nội dung học tập phiên bản hóa. Nội dung game (QuestDefinition, MiniGameConfig, NPCDialogue, level package) dùng **cùng mô hình version/release/pinning** nhưng thuộc module `game` — xem [QUEST_SYSTEM](../01-game-design/QUEST_SYSTEM.md), [MINIGAME_ENGINE](../08-game-client/MINIGAME_ENGINE.md).

## 2. Các loại nội dung

| Loại | Entity | Mô tả | Dùng ở |
|---|---|---|---|
| Lesson | `lesson` | Bài học gồm các khối nội dung (kể cả `worked_example`, §9.2) + activity | Learning Zone, LMS |
| Learning Material | `learning_material` | Tài liệu đọc/xem: rich text, PDF, slide, ví dụ mẫu `worked_example` | Lesson |
| Video / Image / Audio | `media_asset` | Media dùng chung | mọi nơi |
| Interactive Content | `interactive_content` | Mô phỏng, H5P-like widget, scene Godot nhỏ (`godot_scene_ref`) | Lesson, Science Simulation |
| Question | `question` | Câu hỏi đơn ([QUESTION_BANK](QUESTION_BANK.md)) | Exercise, Exam, Mini-game, Battle |
| Exercise | `question_set` (purpose=`exercise`) | Bài tập gồm nhiều câu | Practice Zone, Assignment |
| Question Set | `question_set` | Tập câu tĩnh hoặc theo rule chọn | mọi nơi |
| Assessment | `assessment_blueprint` | Ma trận đề | Exam Zone |
| Learning Activity | `learning_activity` | Đơn vị học làm được, có `activity_type` (`quiz`, `interactive_exercise`, `video_lesson`, `audio_lesson`, `simulation`, `minigame`), xử lý qua [Activity Registry](ACTIVITY_ENGINE.md) (ADR-0048) | Lesson, Quest step, Assignment, Challenge |
| Game Activity | `minigame_config` | Cấu hình mini-game (schema `game`) | Quest, Learning Zone |

`LearningContentType` (loại nội dung học tập có version và phát hành, ADR-0043): `curriculum` (đơn vị version là CurriculumVersion — [CURRICULUM_MODEL §2](CURRICULUM_MODEL.md#2-cấu-trúc-phân-cấp)), `question`, `question_set`, `assessment_blueprint`, `lesson`, `learning_material`, `activity`. Enum chuẩn nằm ở [`enums.schema.json`](../../specs/schemas/common/enums.schema.json); tài liệu khác link về đó. Learning path của LMS **không** phải nội dung có version ([SCHOOL_LMS §3](../04-admin-portal/SCHOOL_LMS.md#3-course-và-learning-path)). Rubric và các activity viết/nói (`written_response`, `speaking`, `project`) ngoài version này ([ADR-0082](../adr/ADR-0082-spec-driven-development-va-noi-dung-deferred.md)).

## 3. Metadata bắt buộc [Confirmed — Master Prompt §4.2]

Mọi *version* của nội dung đều có các field chung (`content_meta`):

| Field | Kiểu | Bắt buộc | Mô tả |
|---|---|---|---|
| `grade_levels` | int[] (1–9) | ✔ | khối áp dụng |
| `subject_id` | uuid | ✔ | |
| `topic_id` | uuid | ✔ (trừ media) | Cột `NOT NULL` ở mọi bảng version nội dung có metadata này |
| `objective_ids` | uuid[] | ✔ với question/question_set/activity | LO đánh giá/giảng dạy |
| `difficulty` | int 1–5 | ✔ với question | 1 rất dễ → 5 rất khó |
| `version` | int | ✔ | tăng dần trong một `*_id` |
| `status` | `content_status` | ✔ | |
| `author_id` | uuid | ✔ | |
| `reviewer_id` | uuid | khi ≥ approved | |
| `published_at` | timestamptz | khi published | |
| `owner_scope` / `tenant_id` | enum (`platform`/`tenant`) / uuid (NULL khi platform) | ✔ | §4 |
| `deleted_at`, `deleted_by` | timestamptz / uuid | | Soft-delete (ADR-0028) — chỉ cho draft/rejected; nội dung đã publish dùng `archived` |
| `cloned_from_id`, `cloned_from_tenant_id`, `cloned_at`, `cloned_by` | uuid / timestamptz | | Nguồn gốc khi System Admin clone giữa tenant (ADR-0029) |
| `content_language` | enum `vi` \| `en` | ✔ | Ngôn ngữ gốc của nội dung (ADR-0056); mặc định `vi`, môn Tiếng Anh là `en`. Xem §8 |
| `tags` | string[] | | tự do, có chuẩn hóa |
| `license` | string ≤ 255 | ✔ với media | Mã giấy phép (`owned`, `licensed`, `cc_by`, `cc_by_sa`, `public_domain`) kèm ghi nguồn/tác giả khi giấy phép yêu cầu |
| `estimated_duration_sec` | int | | |
| `curriculum_version_id` | uuid | ✔ | version mà `objective_ids` thuộc về |

## 4. Phạm vi sở hữu (`owner_scope`) — ADR-0005, ADR-0029

| owner_scope | `tenant_id` | Ai tạo | Ai thấy/dùng | Ai duyệt |
|---|---|---|---|---|
| `platform` | NULL | Content Admin / System Admin | Chỉ các tenant **được gán** (`infra.tenant_resource_assignments` trong database của module sở hữu nội dung — [DATABASE_DESIGN](../07-backend/DATABASE_DESIGN.md)), chỉ đọc | Content Reviewer platform |
| `tenant` | tenant sở hữu | `tenant_content_editor`, `academic_manager`, `school_admin`, `tenant_admin` của tenant (theo permission) | Chỉ tenant đó (với tenant `unit`: mọi trường trong đơn vị, có thể lọc theo trường) | Role có `learning_content.review` của tenant: `tenant_content_reviewer`, `school_admin` (phạm vi trường), `tenant_admin`, `academic_manager` — luôn ≠ tác giả ([ADMIN_PORTAL §2.3](../04-admin-portal/ADMIN_PORTAL.md#23-role--permission)) |

Tenant muốn sửa nội dung platform → System Admin clone sang tenant (bản sao `draft`, `owner_scope = tenant`). `owner_scope` chỉ có hai giá trị `platform`/`tenant` (tenant `unit` đại diện cho đơn vị nhiều trường).

- Nội dung cấp dưới **có thể** tham chiếu nội dung cấp trên (question set trường dùng câu hỏi platform), **không** ngược lại.
- Repository luôn lọc: `deleted_at IS NULL AND (tenant_id = principal.tenant OR (owner_scope='platform' AND <có assignment active cho principal.tenant>))` (chi tiết [MULTITENANCY §4.2](../06-architecture/MULTITENANCY.md)).

## 5. Version, Release pointer và Draft/Publish (ADR-0015, ADR-0043)

Mọi nội dung có version (curriculum version, lesson, learning material, question, question set, assessment blueprint, learning activity ở module learning; quest, mini-game config, NPC dialogue, level package, reward rule, item ở module game; sticker/GIF pack ở module chat) dùng **cùng một mô hình hai lớp**:

```mermaid
erDiagram
  CONTENT_ITEM ||--o{ CONTENT_VERSION : has
  CONTENT_ITEM ||--o{ CONTENT_RELEASE : "lịch sử con trỏ"
  CONTENT_ITEM {
    uuid item_id
    string item_type
    string owner_scope
    uuid tenant_id
    uuid current_version_id "release pointer, NULL = chưa/không publish"
    int latest_version_no
  }
  CONTENT_VERSION {
    uuid version_id
    uuid item_id
    int version_no
    string status "draft|in_review|approved|published|rejected|archived"
    jsonb body "render_payload, answer_key... theo loại"
    string change_note
    timestamptz published_at
    string snapshot_key "khóa snapshot đã dựng"
  }
  CONTENT_RELEASE {
    uuid release_id
    uuid item_id
    uuid from_version_id
    uuid to_version_id
    string action "publish|rollback|unpublish"
    string reason
    uuid released_by
    timestamptz released_at
  }
```

Quy tắc:

1. **Version** là bản nội dung. **Draft** sửa tự do (optimistic concurrency theo field `version` của ent, ADR-0039). Khi đã `published` thì **bất biến**: hook ent + trigger DB chặn UPDATE cột nội dung.
2. Sửa nội dung đã publish = tạo Draft mới từ version đó (`POST .../versions` với `based_on_version`). Chỉ **một** Draft/In-review mở cho mỗi item tại một thời điểm [Confirmed].
3. **Release pointer** `content_items.current_version_id` quyết định version học sinh nhận khi **bắt đầu** một lượt chơi/học mới. Mỗi lần đổi con trỏ ghi một dòng `content_releases` + audit + outbox event.
4. **Publish**: version `approved` → `published`, con trỏ trỏ sang version đó (ngay hoặc hẹn giờ `publish_at`). Lịch hẹn lưu ở `content.scheduled_publications` (trạng thái `scheduled`/`applied`/`cancelled`/`failed`, hủy được trước giờ chạy); `content_releases` là bảng append-only và chỉ ghi khi con trỏ thực sự đổi. Đến giờ, worker chạy lại toàn bộ điều kiện publish; lỗi → lịch `failed` kèm lý do, con trỏ giữ nguyên. Version published trước đó **vẫn giữ** trạng thái `published` (xem và rollback được), chỉ không còn là con trỏ.
5. **Rollback**: trỏ con trỏ về một version `published` cũ hơn (hoặc mới hơn = roll forward). **Không** sao chép, **không** tạo version mới. Bắt buộc `reason`, permission `content.release.rollback`, audit (thay quy tắc "clone rồi publish" cũ của ADR-0015).
6. **Unpublish**: con trỏ về `NULL` → học sinh không bắt đầu lượt mới với nội dung này; lượt đang chơi vẫn tiếp tục với version đã ghim.
7. **Archive** (`published → archived`): ẩn khỏi lựa chọn mới, không thể rollback tới; chỉ khi version không phải con trỏ hiện hành.
8. Mọi attempt, learning event, quest progress, level session, match tham chiếu `version_id`, không phải `item_id`.
9. Sửa lỗi làm sai đáp án sau khi đã có attempt → xem [EXAM_ENGINE §9 Regrade](EXAM_ENGINE.md#9-chấm-lại-regrade).
10. Xóa: Draft/Rejected soft-delete (ADR-0028); version đã publish không bao giờ xóa.

### 5.1. Xem lịch sử và so sánh version

CMS hiển thị danh sách version (số, trạng thái, tác giả, người duyệt, người publish, `change_note`, thời gian), dòng thời gian `content_releases` (ai publish/rollback version nào, lý do), xem nội dung từng version ở chế độ **chỉ đọc**, **so sánh hai version** (diff theo field/khối) và **preview trong game** (Godot Web export dùng snapshot của version đó, không tạo game session thật). Chi tiết màn hình: [ADMIN_PORTAL §3.11](../04-admin-portal/ADMIN_PORTAL.md#311-phát-hành-nội-dung-draftpublish-lịch-sử-rollback-adr-0043).

### 5.2. Version pinning — học sinh đang chơi giữ version cũ

Khi học sinh **bắt đầu** một đơn vị chơi/học, server đọc con trỏ hiện tại và **ghim** version vào đơn vị đó:

| Đơn vị | Nơi ghi version | Thời điểm ghim | Hết ghim khi |
|---|---|---|---|
| Màn/level | `game.level_sessions.content_versions` (jsonb: item → version) | `POST /game/levels/{level_id}:start` | Hoàn thành/rời màn |
| Quest | `game.quest_progress.quest_version_id` | Nhận quest | Hoàn thành/bỏ quest (quest nhiều ngày giữ đến khi xong) [Confirmed] |
| Mini-game | `assessment.attempts.source_version_id` (mode `minigame`) | Bắt đầu mini-game | Nộp/hết giờ |
| Bài học | `learning.lesson_sessions.lesson_version_id` | Mở bài | Hoàn thành/đóng |
| Attempt bài tập | `attempts.source_version_id` + `attempt_items.question_version_id` | Bắt đầu attempt | Submit/expire |
| Đề kiểm tra | `exam_sessions.blueprint_version_id` + `exam_papers` | Nhà trường (`school_admin`/`academic_manager`) tạo exam session | Đóng exam session ([EXAM_ENGINE](EXAM_ENGINE.md)) |
| Bài giao | `lms.assignments.pinned_version_id` (mặc định) hoặc `follow_latest = true` | Lúc giao | Hết hạn bài giao ([SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md)) |
| Trận đấu | `competition.matches.question_plan_version` | Trận bắt đầu | Trận kết thúc |

Trong đơn vị đã ghim, mọi lần tải nội dung dùng version đã ghi, kể cả khi đã có bản mới, đã rollback hoặc đã unpublish. Lượt tiếp theo nhận version mới nhất theo con trỏ.

### 5.3. Content manifest cho game

- Mỗi tenant có một **content manifest**: danh sách `item_type, item_id → version_id, snapshot_url/hash` của mọi nội dung game và học tập mà tenant được dùng (sở hữu hoặc được gán, ADR-0029), kèm `manifest_version` tăng dần.
- Client tải `GET /api/v1/game/content-manifest` (ETag, `If-None-Match` → `304`) **khi khởi động** và **khi vào màn mới**. Trong một màn, client không đổi nội dung.
- Khi publish/rollback, server phát `content.manifest.updated` qua realtime; client chỉ ghi nhận, áp dụng ở màn sau.

### 5.4. Đẩy cache khi publish (ADR-0043 mục 6)

```mermaid
sequenceDiagram
  participant CMS
  participant SVC as learning/module game
  participant W as snapshot worker
  participant VK as Valkey
  participant OS as Object storage + CDN
  CMS->>SVC: POST .../versions/{v}:publish (Idempotency-Key)
  SVC->>SVC: kiểm tra approved, tham chiếu con đã published
  SVC->>W: dựng snapshot (render payload KHÔNG answer key, gói level, media refs)
  W->>VK: SET ew:<module>:cv:<type>:<id>:<v> (bất biến, TTL trượt content_snapshot_cache_ttl)
  W->>OS: PUT content/<type>/<id>/<v>/<hash>.json (chỉ nội dung game không nhạy cảm)
  W-->>SVC: snapshot_key
  SVC->>SVC: TX: current_version_id = v, content_releases, outbox content.published
  SVC-->>CMS: 200 (release_id)
  Note over SVC,VK: consumer content.published
  SVC->>VK: SET ew:<module>:current:<tenant>:<type>:<id> = v
  SVC->>VK: dựng lại manifest tenant, manifest_version++
  SVC-->>Client: content.manifest.updated (realtime)
```

- Snapshot bất biến → không bao giờ cần invalidate; chỉ con trỏ và manifest đổi. TTL trượt của snapshot trong Valkey `content_snapshot_cache_ttl` = 7 ngày [Confirmed]. Cache miss → đọc DB, nạp lại (ADR-0004).
- **Không đưa lên CDN:** render payload câu hỏi của đề kiểm tra, mọi `answer_key`/`scoring_rule`, nội dung bài giao chưa mở. Những thứ này chỉ phục vụ qua API có kiểm quyền và cache phía server.
- Nội dung riêng tenant trên CDN dùng signed URL ngắn hạn (ADR-0044).
- Mục tiêu từ khi bấm Publish đến khi học sinh mới nhận được: ≤ 30 giây [Confirmed].

## 6. Workflow trạng thái

```mermaid
stateDiagram-v2
  [*] --> draft: create / new version (based_on)
  draft --> in_review: submit_review
  in_review --> approved: approve (reviewer ≠ author)
  in_review --> rejected: reject (kèm lý do)
  rejected --> draft: revise
  approved --> published: publish (ngay / theo lịch publish_at)
  approved --> draft: revise trước khi publish
  published --> published: rollback / roll forward (chỉ đổi con trỏ)
  published --> archived: archive (không phải con trỏ hiện hành)
  archived --> [*]
```

| Thao tác | Điều kiện | Event |
|---|---|---|
| `submit_review` | validate schema đầy đủ, mọi media `ready`; nội dung nhóm `g1_3` có đủ audio bắt buộc (§8.1); bản `content_language` đủ (§8) — bộ kiểm `PublishReadinessGuard` (F025) dùng lại khi `publish` | `content.version_submitted` |
| `approve` | reviewer có quyền ở scope; **reviewer ≠ author** (`403 PERMISSION_DENIED` + audit `authz.denied`); miễn trừ audio cần `audio_exemption_acknowledged = true` | `content.version_approved` |
| `reject` | bắt buộc `note` (1–2.000 ký tự) | `content.version_rejected` |
| `revise` | version `rejected` hoặc `approved` chưa publish → `draft` | — (chỉ `content_transitions` + audit `content.revised`) |
| `publish` (hẹn giờ) | như `publish`; `publish_at` ở tương lai | `content.publish_scheduled` |
| `publish` | đã approved; mọi tham chiếu con đã published; audio bắt buộc cho `g1_3` đủ (§8.1); snapshot dựng xong | `content.published` |
| `rollback` | version đích `published`, cùng item, ≠ con trỏ hiện tại; `reason` bắt buộc | `content.rolled_back` |
| `unpublish` | `reason` bắt buộc | `content.unpublished` |
| `archive` | không phải con trỏ hiện hành | `content.version_archived` |
| (sau mọi thao tác đổi con trỏ) | manifest tenant dựng lại | `content.manifest_updated` |

Cấu hình tenant nhỏ có thể bật `self_approve_allowed` (người biên tập nội dung của tenant tự duyệt câu hỏi riêng của tenant, có audit) [Confirmed].

## 7. Media asset pipeline (ADR-0044)

File media **không** lưu trong PostgreSQL. File ở **object storage S3-compatible** (floci ở mọi môi trường, ADR-0044, ADR-0071), phát qua **CDN**; DB chỉ lưu metadata `media_assets` trong database của module sở hữu nội dung (module learning cho nội dung học tập, module game cho nội dung game, module chat cho sticker/GIF).

```mermaid
flowchart LR
  U[CMS] -->|POST /cms/media:presign-upload| API[module sở hữu]
  API -->|presigned PUT, giới hạn loại + dung lượng| U
  U -->|PUT| Q[(ew-media-quarantine)]
  U -->|POST /cms/media/asset_id:complete-upload| API
  API -->|kiểm object, outbox media.uploaded| W[media worker]
  Q --> W
  W --> AV[ClamAV]
  AV --> MB[Kiểm tra magic bytes]
  MB --> EX[Xóa EXIF/GPS]
  EX --> T[Chuyển đổi nhiều biến thể]
  T --> PUB[(ew-media-public: platform)]
  T --> PRI[(ew-media-private: riêng tenant / đề kiểm tra)]
  W --> DB[(media_assets status=ready)]
  PUB & PRI --> CDN[CDN]
```

| Loại | Đầu vào | Đầu ra | Giới hạn `media_max_bytes_by_kind` [Confirmed] |
|---|---|---|---|
| Ảnh | PNG, JPG, WebP, SVG (sanitize) | WebP + PNG, 3 kích thước 256 / 1024 / 2048 px | ≤ 10 MB |
| Âm thanh | MP3, WAV, OGG | **Ogg Vorbis** (Godot) + MP3 (web) | ≤ 20 MB, ≤ 10 phút |
| Video | MP4 (H.264), WebM | **Ogg Theora `.ogv`** (Godot 4 chỉ phát Theora) + HLS 360p/720p + MP4 (CMS/web) | ≤ 300 MB, ≤ 20 phút |
| Mô hình 3D / scene | `.glb`, `.pck` | lưu nguyên + checksum, theo pipeline [WORLD_RENDERING](../08-game-client/WORLD_RENDERING.md) | ≤ 50 MB, chỉ content admin platform |

- **Khóa content-addressed:** `media/<sha256>/<variant>.<ext>` → bất biến, CDN và client cache vĩnh viễn (`Cache-Control: immutable`).
- **Bất biến theo version:** thay ảnh = upload asset mới; version nội dung cũ vẫn trỏ asset cũ, nên học sinh đang chơi version cũ thấy đúng media cũ. Asset còn được version nào tham chiếu thì không bị xóa (soft-delete, ADR-0028).
- **Xin upload và báo xong:** `presignMediaUpload` tạo asset `quarantined` và trả presigned PUT tới `ew-media-quarantine`, hạn `media_presign_ttl` = 15 phút [Confirmed]; trùng `sha256` trong cùng phạm vi → trả `duplicate_of`, không cấp URL mới. Sau khi PUT, portal gọi `completeMediaUpload` (`POST /api/v1/cms/media/{asset_id}:complete-upload`, `Idempotency-Key`): server kiểm object tồn tại, kích thước và `sha256` khớp rồi chuyển `processing` và ghi outbox `media.uploaded`; không khớp → `rejected` (`type_mismatch`/`too_large`). Gọi lại trả trạng thái hiện tại, không phát lại event.
- Trạng thái: `quarantined → processing → ready | rejected`. Version không được `submit_review` nếu media chưa `ready`.
- Media nội dung riêng tenant và media của đề kiểm tra nằm ở `ew-media-private`, phát bằng **signed URL** hạn `media_signed_url_ttl` = 10 phút [Confirmed] (ADR-0044 cho phép 5–15 phút). Media nội dung platform ở `ew-media-public`.
- Một version tham chiếu tối đa 50 asset [Confirmed]. Media không chứa dữ liệu học sinh (học sinh không upload).

Metadata `media_assets`: `asset_id`, `tenant_id`/`owner_scope`, `kind`, `original_filename` varchar(255), `mime_type` varchar(127), `size_bytes`, `width`, `height`, `duration_ms`, `sha256` varchar(64), `variants` jsonb (≤ 16 KB), `alt_text` varchar(500), `license` varchar(255), `status`, cột chuẩn ADR-0039.

## 8. Ngôn ngữ nội dung và bản dịch (ADR-0056)

Game song ngữ `vi-VN` (mặc định) và `en-US` [Confirmed — ADR-0056]. Tài liệu này sở hữu **quy tắc ngôn ngữ của dữ liệu nội dung**; chuỗi UI của client: [GODOT_ARCHITECTURE §10.1](../08-game-client/GODOT_ARCHITECTURE.md#101-quốc-tế-hóa-i18n); biên tập: [ADMIN_PORTAL §1.5](../04-admin-portal/ADMIN_PORTAL.md#15-biên-tập-song-ngữ-adr-0056).

**Hai kiểu văn bản trong dữ liệu**

| Kiểu | Dùng cho | Quy tắc | Schema |
|---|---|---|---|
| **Văn bản game** (`localized_text`) | Tên/mô tả vùng, khu, NPC, hội thoại NPC, quest, challenge, boss, achievement, danh hiệu, item, sự kiện, chương truyện | `{ "vi": "...", "en": "..." }` **bắt buộc đủ cả hai** trước khi publish [Confirmed] | `quest`, `boss`, `challenge`, `achievement`, `minigame-config`, `npc-dialogue`, `progression-curve` (`$defs/localized_text`) |
| **Văn bản học tập** (`i18nText`, `i18nShort/Medium/Long`) | Đề bài, đáp án, giải thích, bài học, hướng dẫn hoạt động, phụ đề | Bắt buộc bản theo `content_language`; bản dịch còn lại **tùy chọn** [Confirmed] | `question`, `activities/*` |

Khóa ngôn ngữ trong dữ liệu luôn là `vi`/`en` (không dùng `vi-VN`/`en-US` trong JSON nội dung); locale đầy đủ chỉ dùng ở client/HTTP (`Accept-Language: vi-VN`).

**`content_language` (bắt buộc ở mọi version nội dung học tập)**

- Môn Tiếng Anh: `en`; đề và đáp án bằng tiếng Anh, hướng dẫn có thể kèm bản `vi` cho lớp nhỏ.
- Các môn khác: `vi`; bản `en` chỉ cần khi trường song ngữ dùng [Confirmed].
- Câu hỏi tham số hóa, công thức (§9.1), media dùng chung giữa hai bản; alt text và `speech_text` theo từng ngôn ngữ.

**Kiểm tra đủ bản dịch trước khi publish**

| Loại nội dung | Bắt buộc | Nếu thiếu |
|---|---|---|
| Nội dung game (bảng trên, dòng 1) | `vi` + `en` | Chặn publish `422 TRANSLATION_INCOMPLETE` (liệt kê field thiếu) |
| Nội dung học tập | Bản `content_language` | Chặn publish; bản còn lại thiếu chỉ cảnh báo, gắn `translation_status = partial` |
| Bản dịch đã có nhưng cũ hơn bản gốc (sửa bản gốc sau khi dịch) | — | Đánh dấu `translation_status = outdated`, cảnh báo khi publish; vẫn publish được với nội dung học tập |

`translation_status` (`complete` \| `partial` \| `outdated`) được tính khi lưu, lưu cùng version để portal lọc nội dung cần dịch.

**Phục vụ và fallback**

- API nhận `Accept-Language`; với snapshot nội dung game và content manifest (cache dài, §5.3–5.4) **trả cả hai ngôn ngữ** để client đổi ngôn ngữ không phải tải lại; với API học tập trả bản theo ngôn ngữ yêu cầu, thiếu thì bản `content_language`.
- Client hiển thị nhãn "Chưa có bản tiếng Anh" / "No Vietnamese version yet" khi rơi về ngôn ngữ gốc (chỉ nội dung học tập; nội dung game không thể thiếu vì đã chặn publish).
- Attempt ghi `display_language` (ngôn ngữ thực tế đã hiển thị) — [ACTIVITY_ENGINE](ACTIVITY_ENGINE.md).
- Âm thanh đọc đề và hướng dẫn là `media_ref` gắn với `stem.audio_ref` / `instructions_audio_ref`, mỗi ngôn ngữ một bản; bắt buộc với nhóm `g1_3` (§8.1).
- Chuẩn hóa Unicode NFC khi lưu mọi text tiếng Việt.

### 8.1. Audio đọc đề và hướng dẫn cho nhóm `g1_3` [Confirmed]

Học sinh lớp 1–3 chưa đọc thạo, nên **lồng tiếng là điều kiện publish** chứ không phải tùy chọn:

| Nội dung có `grade_levels` giao {1, 2, 3} | Bắt buộc | Ghi chú |
|---|---|---|
| Đề câu hỏi (`stem`) | `stem.audio_ref` bản `vi` (môn Tiếng Anh: bản `en` cho phần tiếng Anh + bản `vi` cho lời dẫn) | Câu hỏi nghe hiểu đã có audio là nội dung bài |
| Hướng dẫn activity (`instructions`) | `instructions_audio_ref` bản `vi` | |
| Hướng dẫn bước quest, thoại NPC giao nhiệm vụ (nội dung game, module game) | Audio `vi` theo cùng quy tắc publish | Trường dữ liệu do [QUEST_SYSTEM](../01-game-design/QUEST_SYSTEM.md) định nghĩa |
| Lựa chọn dạng chữ (`options[].content` không có hình) | Khuyến nghị (cảnh báo, không chặn) | |

- **Nguồn audio:** **ghi âm giọng người** [Confirmed — owner 2026-11-20, OQ-23]; EduWorld không tích hợp engine TTS phía server trong Release 1.0. File do người soạn tự tạo ngoài hệ thống (kể cả bằng công cụ TTS) chỉ được dùng khi người duyệt nghe lại trước khi approve. Audio là media thường (Ogg Vorbis + MP3, §7), bất biến theo version. Đọc to bằng TTS của hệ điều hành trên thiết bị chỉ là dự phòng khi nội dung cũ thiếu file, không thay điều kiện publish dưới đây.
- **Miễn trừ:** câu mà đọc to làm lộ đáp án (chính tả, nhận biết chữ, đánh vần) khai báo `audio_exempt_reason` (≤ 255 ký tự); reviewer phải duyệt rõ miễn trừ này.
- **Kiểm tra:** thiếu audio bắt buộc → chặn `submit_review` và `publish` với `422 AUDIO_REQUIRED` (liệt kê field thiếu).
- **Phụ đề và icon:** văn bản đề luôn hiển thị cùng audio (phụ đề); hướng dẫn có thể kèm `instruction_icons[]` (biểu tượng hành động: chạm, kéo, nghe) để học sinh làm được khi phòng máy không có tai nghe hoặc tắt tiếng. Cách hiển thị: [LEARNING_EXPERIENCE §5.1](LEARNING_EXPERIENCE.md#51-hỗ-trợ-đọc-cho-lớp-13-confirmed).
- **Ngân sách sản xuất audio:** [CONTENT_PRODUCTION_PLAN](CONTENT_PRODUCTION_PLAN.md).

## 9. Định dạng rich content

Dùng cấu trúc khối JSON (không lưu HTML tự do) để Godot và web cùng render được:

```json
{
  "blocks": [
    { "type": "paragraph", "text": "Bạn Lan có {{a}} quả táo..." },
    { "type": "math", "latex": "\\frac{3}{4}" },
    { "type": "image", "media_ref": "01J...", "alt": "Rổ táo" },
    { "type": "audio", "media_ref": "01J..." }
  ]
}
```

Khối hỗ trợ: `paragraph` (inline bold/italic/underline, công thức inline `\( ... \)`), `heading`, `list`, `math` (công thức block), `image`, `audio`, `video`, `table`, `callout`, `worked_example` (§9.2, chỉ trong lesson/learning material). Biến `{{x}}` chỉ dùng cho câu hỏi tham số hóa ([QUESTION_BANK §5](QUESTION_BANK.md#5-câu-hỏi-tham-số-hóa-confirmed)).

### 9.1. Công thức Toán, Lý, Hóa (ADR-0055)

| Khía cạnh | Quy định |
|---|---|
| Định dạng lưu | **LaTeX** (mỗi công thức ≤ 2.000 ký tự), hóa học **mhchem** (`\ce{}`, `\pu{}`); inline `\( ... \)`, block `\[ ... \]` hoặc khối `math` |
| Soạn (chỉ Admin Portal) | **MathLive** `<math-field>` trong Lesson/Question/Activity Builder, đáp án, giải thích, gợi ý, phản hồi theo phương án, ví dụ mẫu; preview bằng **MathJax 4** |
| Render SVG | Khi lưu/publish, **portal render trên trình duyệt** bằng MathJax 4 mỗi công thức thành **SVG + speech text**, gửi kèm nội dung. Không có worker render chạy thường trực |
| Kiểm tra phía server | module learning kiểm tra LaTeX theo allowlist, **sanitize SVG** (chỉ phần tử/thuộc tính an toàn, ≤ 256 KB [Confirmed]), LaTeX ≤ 2.000 ký tự và độ sâu lồng ≤ 20, lưu `math/<sha256>.svg` trên object storage/CDN (ADR-0044). Thiếu SVG, SVG không hợp lệ, LaTeX ngoài allowlist → `422 MATH_RENDER_FAILED` |
| Import hàng loạt | Qua màn Import của portal (render trên trình duyệt) hoặc công cụ dòng lệnh dùng cùng MathJax 4 chạy trong CI/vận hành [Confirmed] |
| Hiển thị game | Render payload thay LaTeX bằng khối `{ "math_ref": "math/<sha256>.svg", "speech_text": "...", "latex": "..." }`; Godot hiển thị SVG inline ([GODOT_ARCHITECTURE](../08-game-client/GODOT_ARCHITECTURE.md)). **Game chỉ hiển thị, không nhập công thức** |
| Trả lời của học sinh | Không gõ biểu thức; dùng chọn đáp án, đúng/sai, nhập số, điền số/từ, ghép, sắp xếp, kéo thả ([QUESTION_BANK §3.6.1](QUESTION_BANK.md#361-câu-hỏi-có-công-thức-adr-0055)) |
| Trợ năng | Portal: đọc công thức của MathJax 4; game: `speech_text` cho TTS/mô tả thay thế |
| Phiên bản | Ghim MathJax 4.x và MathLive trong `package.json` của portal; nâng cấp phải qua visual regression bộ công thức mẫu Toán/Lý/Hóa. SVG đã publish không render lại (bất biến theo version, ADR-0043) |

### 9.2. Khối `worked_example` [Confirmed]

Ví dụ mẫu có lời giải từng bước, dùng trước bài mới (NPC trình bày) và sau khi học sinh trả lời sai (nút "Xem ví dụ mẫu" từ `worked_example_ref` của câu hỏi, [QUESTION_BANK §2](QUESTION_BANK.md#2-cấu-trúc-một-question-version)):

```json
{
  "type": "worked_example",
  "id": "we_mul7",
  "objective_ids": ["<MATH.G3.NUM.MUL_TABLE.2>"],
  "problem": { "blocks": [{ "type": "paragraph", "text": "7 × 6 = ?" }] },
  "steps": [
    { "content": { "blocks": [{ "type": "paragraph", "text": "Em biết 7 × 5 = 35." }] }, "audio_ref": null },
    { "content": { "blocks": [{ "type": "paragraph", "text": "7 × 6 = 7 × 5 + 7 = 35 + 7 = 42." }] }, "audio_ref": null }
  ],
  "self_explain_prompt": { "blocks": [{ "type": "paragraph", "text": "Vì sao lại cộng thêm 7?" }] }
}
```

- `steps[]` 1–8 bước, hiển thị lần lượt (học sinh bấm "Tiếp"); mỗi bước có thể kèm `audio_ref` (bắt buộc cho `g1_3` theo §8.1).
- `self_explain_prompt` tùy chọn: câu hỏi tự giải thích không chấm điểm, chỉ để học sinh suy nghĩ.
- Không chứa câu hỏi đang dùng trong đề; không phải evidence mastery. `id` duy nhất trong version của lesson/material, để câu hỏi tham chiếu `{content_item_id, block_id}`.
- Trong danh sách khối của lesson/learning material (API và [lesson.schema.json](../../specs/schemas/lesson.schema.json)), khối có `"kind": "worked_example"`, `id` theo mẫu `^we_[a-z0-9_]{1,40}$`, `objective_ids` 1–20 thuộc curriculum version của nội dung; không có câu hỏi/activity lồng bên trong (F024).
- `worked_example_ref` của câu hỏi là `{content_item_id, block_id}`: `content_item_id` là id lesson hoặc learning material cùng phạm vi; khối phải có trong version hiện hành (con trỏ release) hoặc bản nháp mới nhất và có ít nhất một LO chung với câu hỏi, nếu không `422 VALIDATION_FAILED` (F024).

## 10. Yêu cầu chức năng

Mã FR, nhãn và phase do [PRODUCT_REQUIREMENTS §2.5](../00-overview/PRODUCT_REQUIREMENTS.md#25-learning-content--cnt-42) cấp (ADR-0082); bảng dưới chỉ chỉ ra phần chi tiết trong tài liệu này.

| Yêu cầu (mã PRD) | Chi tiết trong tài liệu này |
|---|---|
| FR-CNT-001 | Các loại nội dung (§2) |
| FR-CNT-002 | Metadata bắt buộc (§3) |
| FR-CNT-003 | Workflow Draft → Review → Approved → Published → Archived (§6) |
| FR-CNT-004 | Version bất biến sau publish; attempt tham chiếu version (§5) |
| FR-CNT-005 | `owner_scope` platform/tenant, gán/clone (§4) |
| FR-CNT-006 | Media asset pipeline: object storage + CDN, quét malware, xóa EXIF, chuyển đổi biến thể (§7) |
| FR-CNT-008 | Báo cáo "nội dung đang được dùng ở đâu" trước khi chuyển `archived` (vòng đời ở §6) |
| FR-CNT-009 | Release pointer, publish ngay hoặc theo lịch `publish_at`, rollback có lý do, unpublish, lịch sử release (§5) |
| FR-CNT-010 | Xem version đã publish, diff, preview (§5.1) |
| FR-CNT-011 | Version pinning (§5.2) |
| FR-CNT-012 | Snapshot bất biến lên Valkey/CDN, content manifest theo tenant (§5.3, §5.4) |
| FR-CNT-017 | Công thức Toán, Lý, Hóa (§9.1) |
| FR-CNT-018 | Ngôn ngữ nội dung, bản dịch, `TRANSLATION_INCOMPLETE`, `translation_status` (§8) |
| FR-CNT-019 | Audio đọc đề và hướng dẫn cho nhóm `g1_3` (`AUDIO_REQUIRED`, miễn trừ được duyệt) (§8.1) |
| FR-CNT-020 | Khối `worked_example` (§9.2) |
| FR-CNT-021 | Tìm kiếm nội dung theo metadata (§3) và full-text tiếng Việt (`unaccent`) |

## 11. Kiểm thử bắt buộc

- Không thể UPDATE version `published` (test trigger DB + use case).
- Reviewer = author bị từ chối approve.
- Học sinh/nhân sự trường B không đọc được nội dung `school`-scope của trường A (API + RLS).
- Câu hỏi `grade_levels = [2]` thiếu `stem.audio_ref` và không có `audio_exempt_reason` → `422 AUDIO_REQUIRED` khi submit review/publish; có miễn trừ được duyệt → publish được.
- Attempt cũ vẫn render đúng version sau khi version mới được publish.
- Học sinh đang trong màn/quest giữ version cũ khi có publish mới hoặc rollback; vào màn mới nhận version theo con trỏ.
- Rollback chỉ đổi con trỏ, không tạo version, ghi `content_releases` + audit; rollback tới version `archived`/khác item bị từ chối (`ROLLBACK_TARGET_INVALID`).
- Publish hai lần cùng `Idempotency-Key` chỉ tạo một release; publish Draft đã bị sửa đồng thời → `CONTENT_VERSION_CONFLICT`.
- Snapshot câu hỏi đề kiểm tra và answer key không xuất hiện trên CDN.
- Manifest trả `304` khi không đổi; đổi sau publish trong ≤ 30 giây.
- Media: file sai magic bytes/quá dung lượng bị từ chối; EXIF bị xóa; version cũ vẫn hiển thị media cũ sau khi thay ảnh.
