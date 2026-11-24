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
