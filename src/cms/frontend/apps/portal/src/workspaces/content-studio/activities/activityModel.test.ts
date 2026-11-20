// F058-AC-01, AC-02, AC-03, AC-04, AC-16…AC-20 (T012, T014, T015, T035, T037, T038; pure rules of the Activity
// Builder): offered types, schema check of the config by type (JSON pointers like a server 422), metadata rules,
// form ↔ ActivityDefinitionInput, question source rules, video rules, recommended length by grade.
import { describe, expect, it } from 'vitest';
import { allowedTypesOf, emptyActivity, fromDefinition, metaErrors, toInput, validateConfig, type ActivityTypeInfo } from './activityModel';
import { offeredTypes } from './editorRegistry';
import './editors';
import { questionConfig } from './editors/QuestionActivityEditor';
import { videoConfig } from './editors/VideoLessonEditor';
import { canAddQuestion, sourceIssues } from './validators/questionSourceRules';
import { recommendedMinutes, videoIssues } from './validators/videoRules';

const info = (activity_type: string, status: string): ActivityTypeInfo => ({ activity_type, status, schema_url: 'x', evaluation_strategies: ['auto'], min_client_version: '0.1.0' }) as ActivityTypeInfo;
const QV = '0193a4d2-5000-7000-8000-000000000101';

describe('Activity Builder rules (F058)', () => {
  it('AC-01: only registered, enabled types with an editor; never speaking / written_response / project', () => {
    const offered = offeredTypes([info('quiz', 'enabled'), info('video_lesson', 'preview'), info('speaking', 'enabled'), info('project', 'enabled'), info('minigame', 'enabled'), info('interactive_exercise', 'disabled')]);
    expect(offered.map((x) => x.activity_type)).toEqual(['quiz', 'video_lesson']);
  });

  it('AC-02, AC-04: the config of each type is checked with its schema; errors are /config pointers', () => {
    expect(validateConfig('quiz', { ...questionConfig('quiz'), content: { question_source: { kind: 'questions', questions: [{ question_version_id: QV }] } } })).toEqual([]);
    const bad = validateConfig('quiz', { content: { question_source: { kind: 'questions', questions: [] }, timed: { total_seconds: 5 } }, evaluation: { strategies: ['auto'], max_score: 10 }, feedback: { timing: 'soon' }, completion_rule: { mode: 'submit' } });
    const pointers = bad.map((e) => e.pointer);
    expect(pointers).toContain('/config/content/question_source/questions');
    expect(pointers).toContain('/config/content/timed/total_seconds');
    expect(pointers).toContain('/config/feedback/timing');
    expect(validateConfig('video_lesson', { ...videoConfig(), content: { ...(videoConfig().content as object), script: 'x' } }).some((e) => e.pointer.startsWith('/config/content'))).toBe(true);
    const s = emptyActivity('quiz', questionConfig('quiz'));
    expect(metaErrors(s)).toEqual(['/title', '/grade_levels', '/subject_id', '/objective_ids']);
    expect(allowedTypesOf('interactive_exercise').defaults).toContain('fill_in_blank');
    expect(allowedTypesOf('quiz').defaults).not.toContain('fill_in_blank');
  });

  it('form ↔ ActivityDefinitionInput keeps description, instructions, topic and timer (full PATCH body)', () => {
    const d = { activity_id: QV, activity_type: 'quiz', title: { vi: 'Luyện nhân' }, status: 'draft', version: 2, description: { vi: 'Ôn' }, instructions: { vi: 'Chọn' }, topic_id: QV, time_limit_seconds: 600, grade_levels: [3], objective_ids: [QV], subject_id: QV, difficulty: 2, content_language: 'vi', config: { content: {} } } as never;
    const body = toInput(fromDefinition(d)) as Record<string, unknown>;
    expect(body).toMatchObject({ activity_type: 'quiz', title: { vi: 'Luyện nhân' }, description: { vi: 'Ôn' }, instructions: { vi: 'Chọn' }, topic_id: QV, time_limit_seconds: 600, difficulty: 2, content_language: 'vi' });
  });

  it('AC-03: question source — type outside allowed_question_types refused, duplicates, three kinds', () => {
    const src = { kind: 'questions' as const, questions: [{ question_version_id: QV, type: 'single_choice' }] };
    expect(canAddQuestion(src, { question_version_id: 'other', type: 'numeric' }, allowedTypesOf('quiz').defaults)).toBe('type');
    expect(canAddQuestion(src, { question_version_id: QV, type: 'single_choice' }, [])).toBe('duplicate');
    expect(canAddQuestion(src, { question_version_id: 'other', type: 'true_false' }, allowedTypesOf('quiz').defaults)).toBe('ok');
    expect(sourceIssues({ kind: 'questions', questions: [] }, []).map((i) => i.key)).toEqual(['source.noQuestion']);
    expect(sourceIssues({ kind: 'question_set' }, []).map((i) => i.key)).toEqual(['source.noSet']);
    expect(sourceIssues({ kind: 'selection', selection: { count: 0, difficulty_min: 4, difficulty_max: 2, question_types: ['numeric'] } }, ['single_choice']).map((i) => i.key)).toEqual(['source.count', 'source.difficulty', 'source.typeNotAllowed']);
  });

  it('AC-16…AC-19: chapters, cues, subtitles, branches; AC-20 recommended length by grade', () => {
    const issues = videoIssues(
      {
        video_asset_id: QV,
        duration_ms: 240_000,
        chapters: [
          { chapter_id: 'a', start_ms: 0, end_ms: 120_000 },
          { chapter_id: 'b', start_ms: 100_000, end_ms: 300_000 },
        ],
        embedded_questions: [
          { cue_id: 'q1', at_ms: 90_000, question_version_id: QV },
          { cue_id: 'q1', at_ms: 300_000, question_version_id: QV },
        ],
        subtitles: [{ language: 'en', subtitle_asset_id: QV }],
        segments: [{ segment_id: 'water', video_asset_id: QV }],
        branches: [{ branch_point_id: 'bp1', at_ms: 1000, options: [{ option_id: 'o1', target_segment_id: 'land' }] }],
      },
      { speech: true },
    );
    const keys = issues.map((i) => `${i.key}@${i.pointer}`);
    expect(keys).toContain('video.afterEnd@/config/content/chapters/1/end_ms');
    expect(keys).toContain('video.chapterOverlap@/config/content/chapters/1/start_ms');
    expect(keys).toContain('video.cueDuplicate@/config/content/embedded_questions/1/cue_id');
    expect(keys).toContain('video.cueOutside@/config/content/embedded_questions/1/at_ms');
    expect(keys).toContain('video.subtitleDefaultIssue@/config/content/subtitles');
    expect(keys).toContain('video.subtitleViRequired@/config/content/subtitles');
    expect(keys).toContain('video.branchOptions@/config/content/branches/0/options');
    expect(keys).toContain('video.branchTarget@/config/content/branches/0/options/0/target_segment_id');
    expect(keys).toContain('video.noEnding@/config/content/segments');
    expect(videoIssues({ video_asset_id: QV, duration_ms: 240_000, chapters: [{ chapter_id: 'a', start_ms: 0, end_ms: 60_000 }], embedded_questions: [{ cue_id: 'q1', at_ms: 90_000, question_version_id: QV }], subtitles: [{ language: 'vi', subtitle_asset_id: QV, is_default: true }] }, { speech: true })).toEqual([]);
    expect(recommendedMinutes([3])).toBe(8);
    expect(recommendedMinutes([7])).toBe(15);
    expect(recommendedMinutes([])).toBeNull();
    expect((videoConfig().completion_rule as Record<string, unknown>).min_watched_ratio).toBe(0.9);
    expect((videoConfig().interaction as Record<string, unknown>).allow_seek).toBe('watched_only');
  });
});
