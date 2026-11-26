// @eduworld/ops-ui/game-console — Game Operations Console pieces (F079, workspace A): the versioned editor frame shared with
// Content Studio (ContentEditorShell, ItemStatusBar, ConflictBanner, useAutosave), the JSON-Schema form, the bilingual
// badge, thumbnail / turntable of a version, reference picker and the Godot Web preview frame. A separate entry without
// MathJax / MathLive so the game-ops chunk stays small. No API calls here (ADMIN_PORTAL §1.1).
export { ContentEditorShell } from './editor/ContentEditorShell';
export type { ContentEditorShellProps } from './editor/ContentEditorShell';
export { ItemStatusBar, actionsForStatus, visibleActions } from './editor/ItemStatusBar';
export type { ItemStatusBarProps, WorkflowAction, EditorBlocker, ContentStatusValue } from './editor/ItemStatusBar';
export { ConflictBanner } from './editor/ConflictBanner';
export { AutosaveController, useAutosave } from './editor/useAutosave';
export type { SaveState, AutosaveSnapshot, AutosaveOptions, UseAutosaveResult } from './editor/useAutosave';
export { TranslationBadge } from './i18n-field/BilingualField';
export { SchemaForm } from './game-console/SchemaForm';
export type { SchemaFormProps, SchemaFieldContext } from './game-console/SchemaForm';
export { canonicalPath, effective, getIn, initialValue, isLocalized, pathKey, resolveRef, setIn, typeOf, withConsts } from './game-console/schemaModel';
export type { JsonSchema, PathSegment } from './game-console/schemaModel';
export { ThumbnailImage } from './game-console/ThumbnailImage';
export type { ThumbnailImageProps, RenderStatus } from './game-console/ThumbnailImage';
export { TurntableViewer, framePosition, TURNTABLE_FRAMES } from './game-console/TurntableViewer';
export { GamePreviewFrame, PREVIEW_MESSAGE_TYPE } from './game-console/GamePreviewFrame';
export { ManifestRefPicker } from './game-console/ManifestRefPicker';
export type { ManifestRefPickerProps, RefOption } from './game-console/ManifestRefPicker';
export { textKey } from './game-console/SchemaForm';
// F082: checksum upload queue (chat assets, translation files) and the presentational pieces of the game translations.
export { useChecksumUpload, sha256Hex, safeFileName } from './upload/useChecksumUpload';
export type { UploadItem, UploadError, UploadStatus, PresignInput, PresignOutput, UseChecksumUploadOptions } from './upload/useChecksumUpload';
export { UploadQueue } from './upload/UploadQueue';
export type { UploadQueueProps } from './upload/UploadQueue';
export { TranslationStatusBadge } from './translations/TranslationStatusBadge';
export type { TranslationStatusValue, TranslationStatusBadgeProps } from './translations/TranslationStatusBadge';
export { TranslationJobProgress } from './translations/TranslationJobProgress';
export type { TranslationJobView, TranslationReportError, TranslationJobProgressProps } from './translations/TranslationJobProgress';
export { TranslationExportDialog } from './translations/TranslationExportDialog';
export type { TranslationExportValues, TranslationExportDialogProps } from './translations/TranslationExportDialog';
export { TranslationImportDialog } from './translations/TranslationImportDialog';
export type { TranslationImportDialogProps, ImportUploadState } from './translations/TranslationImportDialog';
// Bilingual vi/en text (no MathJax: F053 BilingualField is presentational) — sticker pack names and alt texts (F082-AC-02).
export { BilingualField } from './i18n-field/BilingualField';
export type { BilingualFieldProps } from './i18n-field/BilingualField';
