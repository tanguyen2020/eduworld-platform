// F053 T066 (OQ F053-9): the topic pickers of DocMetaFields and QuestionActivityEditor infer the current curriculum
// version for the subject × grade (khối) from listCmsCurricula (current_version) + getCurriculumVersion (subject_grades),
// then call listCmsTopics with curriculum_version_id as the contract requires; without such a version they explain
// why the list is empty. Whole portal + MOCK backend (AGENTS R17).
import { screen, waitFor } from '@testing-library/react';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { MockActivities } from '../../../test/mock-activities';
import { P, renderPublishing, stopPortals, TENANTS, tenantUser, type MockPublishing } from '../../../test/publishing-harness';
import { pickCurrentVersion, type Curriculum, type CurriculumVersion } from './useCurriculum';

afterEach(() => {
  stopPortals();
  vi.unstubAllGlobals();
});

const MATH = P(10);
const OTHER = P(11);
const TENANT = TENANTS.thDemoA.tenant_id;

const curriculum = (id: string, owner: 'platform' | 'tenant', current?: { id: string; version: number }): Curriculum => ({
  curriculum_id: id,
  code: `C-${id.slice(-3)}`,
  name: { vi: `Chương trình ${id.slice(-3)}` },
  owner_scope: owner,
  latest_version_no: current?.version ?? 1,
  current_version: current ? { curriculum_version_id: current.id, version: current.version, version_label: `${current.version}.0`, status: 'published' } : null,
});
const version = (id: string, curriculumId: string, n: number, grades: { subject_id: string; grade_level: number }[], status: CurriculumVersion['status'] = 'published'): CurriculumVersion => ({
  curriculum_version_id: id,
  curriculum_id: curriculumId,
  version: n,
  version_label: `${n}.0`,
  status,
  subject_grades: grades,
});

describe('pickCurrentVersion (T066)', () => {
  const cPlat = curriculum(P(700), 'platform', { id: P(711), version: 2 });
  const cTen = curriculum(P(701), 'tenant', { id: P(721), version: 1 });
  const cDraft = curriculum(P(702), 'tenant');
  const vPlat = version(P(711), P(700), 2, [{ subject_id: MATH, grade_level: 3 }, { subject_id: MATH, grade_level: 4 }]);
  const vPlatOld = version(P(710), P(700), 1, [{ subject_id: MATH, grade_level: 5 }], 'archived');
  const vTen = version(P(721), P(701), 1, [{ subject_id: MATH, grade_level: 4 }]);

  it('takes the current version (not an older one) covering subject × grade', () => {
    expect(pickCurrentVersion([cPlat], [vPlatOld, vPlat], MATH, 3)?.version.curriculum_version_id).toBe(P(711));
    expect(pickCurrentVersion([cPlat], [vPlatOld, vPlat], MATH, 5)).toBeUndefined();
    expect(pickCurrentVersion([cPlat], [vPlat], OTHER, 3)).toBeUndefined();
  });

  it("prefers the tenant's own curriculum over a platform one; a curriculum without a current version never counts", () => {
    expect(pickCurrentVersion([cPlat, cTen, cDraft], [vPlat, vTen], MATH, 4)?.version.curriculum_version_id).toBe(P(721));
    expect(pickCurrentVersion([cPlat, cTen, cDraft], [vPlat, vTen], MATH, 3)?.curriculum.curriculum_id).toBe(P(700));
  });

  it('without one grade (several or none chosen) matches the subject only', () => {
    expect(pickCurrentVersion([cPlat], [vPlat], MATH)?.version.curriculum_version_id).toBe(P(711));
    expect(pickCurrentVersion([], [], MATH)).toBeUndefined();
  });
});

const meta = { title: { vi: 'Luyện chọn câu' }, grade_levels: [3], subject_id: MATH, objective_ids: [], difficulty: 2, content_language: 'vi' };
const selectionQuiz = {
  content: { question_source: { kind: 'selection', selection: { count: 10 } }, allowed_question_types: ['single_choice'] },
  evaluation: { strategies: ['auto'], max_score: 10 },
  feedback: { timing: 'immediate' },
  completion_rule: { mode: 'submit' },
};

function setupActivity(withCurriculum: boolean) {
  const ID = P(801);
  return {
    ID,
    setup: (m: MockPublishing) => {
      const acts = new MockActivities(m);
      acts.add(ID, [{ status: 'draft', content: { activity_type: 'quiz', ...meta, config: selectionQuiz } }], { tenant_id: TENANT });
      if (withCurriculum) {
        m.curricula = [curriculum(P(700), 'platform', { id: P(711), version: 2 })];
        m.curriculumVersions = [version(P(711), P(700), 2, [{ subject_id: MATH, grade_level: 3 }])];
      }
    },
  };
}

describe('topic pickers (T066, whole portal)', () => {
  it('DocMetaFields and the selection rule call listCmsTopics with the inferred curriculum_version_id', async () => {
    const { ID, setup } = setupActivity(true);
    const { backend } = await renderPublishing(`/content-studio/activities/${ID}`, { account: tenantUser(), setup });
    // both pickers name the inferred version; they share one listCmsTopics query (same key, react-query dedupes)
    expect((await screen.findAllByText('Chủ đề theo chương trình Chương trình 700, phiên bản 2.0', {}, { timeout: 60_000 })).length).toBeGreaterThanOrEqual(2);
    await waitFor(() => expect(backend.calls('GET', '/cms/learning/topics').length).toBeGreaterThan(0));
    for (const r of backend.requests.filter((x) => x.method === 'GET' && x.path === '/cms/learning/topics')) {
      const q = new URLSearchParams(r.search);
      expect(q.get('curriculum_version_id'), r.search).toBe(P(711));
      expect(q.get('subject_id'), r.search).toBe(MATH);
      expect(q.get('grade_level'), r.search).toBe('3');
    }
    expect(backend.calls('GET', `/cms/learning/curricula/${P(700)}/versions/2`).length).toBeGreaterThan(0);
  });

  it('no current curriculum version for subject × grade → no listCmsTopics, an explanation instead', async () => {
    const { ID, setup } = setupActivity(false);
    const { backend } = await renderPublishing(`/content-studio/activities/${ID}`, { account: tenantUser(), setup });
    expect((await screen.findAllByText(/Chưa có chương trình học đã xuất bản cho môn và khối này/, {}, { timeout: 60_000 })).length).toBeGreaterThanOrEqual(2);
    await waitFor(() => expect(backend.calls('GET', '/cms/learning/curricula').length).toBeGreaterThan(0));
    expect(backend.calls('GET', '/cms/learning/topics')).toHaveLength(0);
  });
});
