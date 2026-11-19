// School detail (F052 T016, F052-AC-01..03, F052-AC-07): getTenantSchool; out-of-scope school (404/403) → "Không tìm
// thấy" without revealing it exists. Edit and approve are wrapped in ActionGate (action.school_lms.schools.edit).
import { Button, Card, Descriptions, Space, Spin } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link, useParams } from 'react-router-dom';
import { ActionGate } from '../../../permissions/ActionGate';
import { QueryError } from '../common/errors';
import { localizedName } from '../common/format';
import { SourceTag, StatusTag } from '../common/ui';
import { ApproveSchoolDialog } from './ApproveSchoolDialog';
import { SchoolEditForm } from './SchoolEditForm';
import { useSchool } from './useSchools';

export function SchoolDetailPage() {
  const { t, i18n } = useTranslation();
  const { schoolId } = useParams();
  const q = useSchool(schoolId);
  const [editing, setEditing] = useState(false);
  const [approving, setApproving] = useState(false);
  if (q.error) return <QueryError error={q.error} resource />;
  if (!q.data) return <Spin aria-label={t('app.loading')} />;
  const s = q.data;
  return (
    <Space direction="vertical" size="middle" style={{ width: '100%' }}>
      <Link to="..">{t('portal.school_lms.common.back')}</Link>
      <Card
        title={localizedName(s.name, i18n.language)}
        extra={
          <ActionGate actionKey="action.school_lms.schools.edit">
            <Space>
              {s.status === 'pending_review' ? (
                <Button type="primary" onClick={() => setApproving(true)}>
                  {t('portal.school_lms.schools.approve')}
                </Button>
              ) : null}
              <Button onClick={() => setEditing(true)} disabled={editing}>
                {t('portal.school_lms.common.edit')}
              </Button>
            </Space>
          </ActionGate>
        }
      >
        {editing ? (
          <SchoolEditForm school={s} onDone={() => setEditing(false)} />
        ) : (
          <Descriptions column={{ xs: 1, md: 2 }}>
            <Descriptions.Item label={t('portal.school_lms.schools.code')}>{s.code}</Descriptions.Item>
            <Descriptions.Item label={t('portal.school_lms.schools.status')}>
              <StatusTag group="school" value={s.status} />
            </Descriptions.Item>
            <Descriptions.Item label={t('portal.school_lms.schools.province')}>{s.province ? localizedName(s.province.name, i18n.language) : t('portal.school_lms.schools.noProvince')}</Descriptions.Item>
            <Descriptions.Item label={t('portal.school_lms.schools.level')}>{t(`portal.school_lms.schools.levels.${s.school_level}`)}</Descriptions.Item>
            <Descriptions.Item label={t('portal.school_lms.schools.grades')}>{`${s.grade_min}–${s.grade_max}`}</Descriptions.Item>
            <Descriptions.Item label={t('portal.school_lms.schools.address')}>{s.address ?? ''}</Descriptions.Item>
            <Descriptions.Item label={t('portal.school_lms.common.source')}>
              <SourceTag source={s.source} />
            </Descriptions.Item>
          </Descriptions>
        )}
      </Card>
      {approving ? <ApproveSchoolDialog school={s} open={approving} onClose={() => setApproving(false)} /> : null}
    </Space>
  );
}
