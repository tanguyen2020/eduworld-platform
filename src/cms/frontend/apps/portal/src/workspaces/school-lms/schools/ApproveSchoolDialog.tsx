// Approve a `pending_review` school (F052 T016, F052-AC-03, FR-SIS-005): shows why it waits, requires a province
// (active provinces only) before approveTenantSchool (If-Match). When the school has no province yet the chosen one
// is saved first with updateTenantSchool; the server still checks both steps.
import { Alert, App, Form, Modal } from 'antd';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { writeErrorKey } from '../common/errors';
import { localizedName } from '../common/format';
import { useActiveProvinces, useApproveSchool, useUpdateSchool, type School } from './useSchools';
import { SearchSelect } from '@eduworld/ops-ui';

/** Reasons a school waits for review, as i18n keys (pure, F052-AC-03). */
export function pendingReasons(school: Pick<School, 'status' | 'province'>): string[] {
  if (school.status !== 'pending_review') return [];
  const reasons = ['portal.school_lms.schools.pending.new'];
  if (!school.province) reasons.unshift('portal.school_lms.schools.pending.noProvince');
  return reasons;
}

export function ApproveSchoolDialog({ school, open, onClose }: { school: School; open: boolean; onClose: () => void }) {
  const { t, i18n } = useTranslation();
  const { message } = App.useApp();
  const provinces = useActiveProvinces(open);
  const update = useUpdateSchool(school.school_id);
  const approve = useApproveSchool(school.school_id);
  const [province, setProvince] = useState<string | undefined>(school.province?.code);
  const busy = update.isPending || approve.isPending;
  const ok = async () => {
    if (!province) return;
    try {
      if (province !== school.province?.code) await update.mutateAsync({ province_code: province });
      await approve.mutateAsync();
      message.success(t('portal.school_lms.schools.approved'));
      onClose();
    } catch (e) {
      message.error(t(writeErrorKey(e)));
    }
  };
  return (
    <Modal
      open={open}
      title={t('portal.school_lms.schools.approveTitle', { name: localizedName(school.name, i18n.language) })}
      okText={t('portal.school_lms.schools.approve')}
      cancelText={t('portal.school_lms.common.cancel')}
      okButtonProps={{ disabled: !province, 'data-testid': 'approve-ok' } as never}
      confirmLoading={busy}
      onOk={ok}
      onCancel={onClose}
      destroyOnHidden
    >
      <Alert type="info" showIcon message={t('portal.school_lms.schools.pending.title')} description={pendingReasons(school).map((k) => t(k)).join(' ')} />
      <Form layout="vertical">
        <Form.Item label={t('portal.school_lms.schools.province')} required help={province ? undefined : t('portal.school_lms.schools.provinceRequired')}>
          <SearchSelect
            aria-label={t('portal.school_lms.schools.province')}
            value={province}
            onChange={setProvince}
            showSearch
            optionFilterProp="label"
            loading={provinces.isFetching}
            options={(provinces.data ?? []).filter((p) => p.status === 'active').map((p) => ({ value: p.code, label: localizedName(p.name, i18n.language) }))}
          />
        </Form.Item>
      </Form>
    </Modal>
  );
}
