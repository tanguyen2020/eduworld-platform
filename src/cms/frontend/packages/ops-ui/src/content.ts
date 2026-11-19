// @eduworld/ops-ui/content — content editing components (F053). A separate entry so the portal shell bundle never
// pulls MathJax / MathLive (they load with the Content Studio workspace chunk). No API calls here (ADMIN_PORTAL §1.1).
export { ContentEditorShell } from './editor/ContentEditorShell';
export type { ContentEditorShellProps } from './editor/ContentEditorShell';
export { ItemStatusBar, actionsForStatus, visibleActions } from './editor/ItemStatusBar';
export type { ItemStatusBarProps, WorkflowAction, EditorBlocker, ContentStatusValue } from './editor/ItemStatusBar';
export { ConflictBanner } from './editor/ConflictBanner';
export { AutosaveController, useAutosave } from './editor/useAutosave';
export type { SaveState, AutosaveSnapshot, AutosaveOptions, UseAutosaveResult } from './editor/useAutosave';
export { RichContentEditor } from './rich-content/RichContentEditor';
export type { RichContentEditorProps, MediaSlotArgs } from './rich-content/RichContentEditor';
export { WorkedExampleBlock, newWorkedExample, newWorkedExampleId, validateWorkedExample, WORKED_EXAMPLE_ID } from './rich-content/WorkedExampleBlock';
export type { WorkedExampleIssue } from './rich-content/WorkedExampleBlock';
export { collectMath, buildMathAssets, pathsForMathPointer } from './rich-content/collectMath';
export type { FoundMath, MathAssetInput, MathBuildResult } from './rich-content/collectMath';
export { emptyRichContent, BASIC_BLOCKS } from './rich-content/types';
export type { RichBlock, RichBlockType, RichContent, WorkedExample, WorkedExampleStep } from './rich-content/types';
export { MathInput, loadMathlive } from './math/MathInput';
export type { MathInputProps } from './math/MathInput';
export { MathText } from './math/MathText';
export { MathInsertButton } from './math/MathInsertButton';
export { renderMath, checkLatex, MathRenderError, mathId } from './math/renderMath';
export type { RenderedMath, MathErrorCode } from './math/renderMath';
export { splitMath, wrapLatex, hasMath } from './math/delimiters';
export { keyboardLayouts, KEYBOARD_TABS } from './math/keyboardLayouts';
export { LATEX_MAX_LENGTH, LATEX_MAX_DEPTH, MATH_SVG_MAX } from './math/mathjaxConfig';
export { BilingualField, TranslationBadge } from './i18n-field/BilingualField';
export type { BilingualFieldProps } from './i18n-field/BilingualField';
export { fieldStatus, toLangMap, fromLangMap, textIn, otherLang } from './i18n-field/translationStatus';
export type { ContentLang, I18nTextValue, FieldTranslationStatus } from './i18n-field/translationStatus';
export { QuestionEditor } from './question-editor/QuestionEditor';
export type { QuestionEditorProps } from './question-editor/QuestionEditor';
export { QuestionPreview } from './question-editor/QuestionPreview';
export { OptionFeedbackFields } from './question-editor/OptionFeedbackFields';
export { HintsField } from './question-editor/HintsField';
export { itemId, isItemId, ITEM_ID_PATTERN } from './question-editor/itemId';
export {
  QUESTION_TYPES,
  ADVANCED_TYPES,
  LIMITS as QUESTION_LIMITS,
  defaultsFor,
  newQuestionDraft,
  changeType,
  needsTypeChangeConfirm,
  answerIssues,
  toStudentView,
  scoringMethods,
  expressionInputWarning,
} from './question-editor/mappers';
export type { QuestionDraft, QuestionType, OptionItem, ScoringRule, AnswerIssue, AnswerIssueCode, TextMatch } from './question-editor/mappers';
