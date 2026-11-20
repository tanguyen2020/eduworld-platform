// Built-in editors of the Activity Builder (F058): quiz, interactive_exercise, video_lesson; F066 adds audio_lesson and
// simulation the same way (registerActivityEditor). Imported once by the activities screen.
import type { EditorBlocker } from '@eduworld/ops-ui/content';
import type { Config } from '../activityModel';
import { registerActivityEditor, type BlockerContext } from '../editorRegistry';
import { allowedTypesOf } from '../activityModel';
import { sourceIssues, type QuestionSource } from '../validators/questionSourceRules';
import { QuestionActivityEditor, questionConfig } from './QuestionActivityEditor';
import { VideoLessonEditor, videoBlockers, videoConfig } from './VideoLessonEditor';
import { AudioLessonEditor, audioBlockers, audioConfig } from './audio_lesson/AudioLessonEditor';
import { SimulationEditor, simulationBlockers, simulationConfig } from './simulation/SimulationEditor';

const questionBlockers =
  (type: 'quiz' | 'interactive_exercise') =>
  (config: Config, ctx: BlockerContext): EditorBlocker[] => {
    const content = (config.content ?? {}) as { question_source?: QuestionSource; allowed_question_types?: string[] };
    const allowed = content.allowed_question_types ?? allowedTypesOf(type).defaults;
    return sourceIssues(content.question_source, allowed).map((i) => ({ field: i.pointer, message: ctx.t(`content_studio.activity.${i.key}`) }));
  };

registerActivityEditor({ type: 'quiz', labelKey: 'content_studio.activity.types.quiz', empty: () => questionConfig('quiz'), Editor: QuestionActivityEditor, blockers: questionBlockers('quiz') });
registerActivityEditor({
  type: 'interactive_exercise',
  labelKey: 'content_studio.activity.types.interactive_exercise',
  empty: () => questionConfig('interactive_exercise'),
  Editor: QuestionActivityEditor,
  blockers: questionBlockers('interactive_exercise'),
});
registerActivityEditor({ type: 'video_lesson', labelKey: 'content_studio.activity.types.video_lesson', empty: videoConfig, Editor: VideoLessonEditor, blockers: videoBlockers });
registerActivityEditor({ type: 'audio_lesson', labelKey: 'content_studio.activity.types.audio_lesson', empty: audioConfig, Editor: AudioLessonEditor, blockers: audioBlockers });
registerActivityEditor({ type: 'simulation', labelKey: 'content_studio.activity.types.simulation', empty: simulationConfig, Editor: SimulationEditor, blockers: simulationBlockers });
