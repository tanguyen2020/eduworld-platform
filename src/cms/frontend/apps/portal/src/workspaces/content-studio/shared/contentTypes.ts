// Content types of the release screens (F054, ADR-0090): for each LearningContentType, the catalog actions of its
// menu (view / edit / workflow / publish / rollback), the authoring REST base (create version, delete, draft) when the
// contract has one, and the trash resource type. Names only — the backend authorizes every call (AGENTS R20).
import type { ActionKey } from '../../../generated/menu-catalog';
import type { components } from '../../../shared/api/schema.gen';

export type ContentType = components['schemas']['LearningContentType'];

export interface ContentTypeDef {
  /** Catalog action of each operation group (`x-portal-action` of the operation for this content_type). */
  actions: { view: ActionKey; edit?: ActionKey; workflow: ActionKey; publish: ActionKey; rollback: ActionKey };
  /** REST base of the authoring API (`/cms/learning/question-sets`…); absent = no create-version/delete operation. */
  base?: string;
}

export const CONTENT_TYPES: Record<ContentType, ContentTypeDef> = {
  question: {
    actions: {
      view: 'action.content_studio.question_bank.view',
      edit: 'action.content_studio.question_bank.edit',
      workflow: 'action.content_studio.question_bank.workflow',
      publish: 'action.content_studio.question_bank.publish',
      rollback: 'action.content_studio.question_bank.rollback',
    },
    base: '/cms/learning/questions',
  },
  question_set: {
    actions: {
      view: 'action.content_studio.question_sets.view',
      edit: 'action.content_studio.question_sets.edit',
      workflow: 'action.content_studio.question_sets.workflow',
      publish: 'action.content_studio.question_sets.publish',
      rollback: 'action.content_studio.question_sets.rollback',
    },
    base: '/cms/learning/question-sets',
  },
  assessment_blueprint: {
    actions: {
      view: 'action.content_studio.assessments.view',
      edit: 'action.content_studio.assessments.edit',
      workflow: 'action.content_studio.assessments.workflow',
      publish: 'action.content_studio.assessments.publish',
      rollback: 'action.content_studio.assessments.rollback',
    },
    base: '/cms/learning/assessment-blueprints',
  },
  lesson: {
    actions: {
      view: 'action.content_studio.lessons.view',
      edit: 'action.content_studio.lessons.edit',
      workflow: 'action.content_studio.lessons.workflow',
      publish: 'action.content_studio.lessons.publish',
      rollback: 'action.content_studio.lessons.rollback',
    },
    base: '/cms/learning/lessons',
  },
  learning_material: {
    actions: {
      view: 'action.content_studio.lessons.view',
      edit: 'action.content_studio.lessons.edit',
      workflow: 'action.content_studio.lessons.workflow',
      publish: 'action.content_studio.lessons.publish',
      rollback: 'action.content_studio.lessons.rollback',
    },
    base: '/cms/learning/learning-materials',
  },
  activity: {
    actions: {
      view: 'action.content_studio.activities.view',
      edit: 'action.content_studio.activities.edit',
      workflow: 'action.content_studio.activities.workflow',
      publish: 'action.content_studio.activities.publish',
      rollback: 'action.content_studio.activities.rollback',
    },
    base: '/cms/learning/activities',
  },
  learning_path: {
    actions: {
      view: 'action.content_studio.learning_paths.view',
      workflow: 'action.content_studio.learning_paths.workflow',
      publish: 'action.content_studio.learning_paths.publish',
      rollback: 'action.content_studio.learning_paths.rollback',
    },
  },
};

export const ALL_CONTENT_TYPES = Object.keys(CONTENT_TYPES) as ContentType[];

/** Statuses of ContentStatus (filters, DomainTag `content_status`). */
export const CONTENT_STATUSES = ['draft', 'in_review', 'approved', 'published', 'archived', 'rejected'] as const;
export type ContentStatus = (typeof CONTENT_STATUSES)[number];
