// Screen of `menu.school_lms.study_groups` (F055 T035, F055-AC-16): groups of a school (listStudyGroups); with
// lms.study_group.write (catalog action study_groups.edit) create / edit — name and members within StudyGroupInput
// limits, members only from the classes of the school in scope (listTenantSchoolClasses → class students) — and
// delete after a confirmation (soft-delete on the server; assignments keep their target snapshot).
import { Button, Flex, Form, Input, Modal, Select, Space } from 'antd';
import type { ColumnType } from 'antd/es/table';
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useSearchParams } from 'react-router-dom';
import { ActionGate } from '../../../../permissions/ActionGate';
import { useClassStudents, useSchoolClasses } from '../../classes/useClasses';
import { ResponsiveTable } from '../../common/ResponsiveTable';
import { SchoolSelect } from '../../common/SchoolSelect';
import { SingleTenantGate } from '../../common/scope';
import { PageTitle, useFieldRules } from '../../common/ui';
import { useDeleteStudyGroup, useSaveStudyGroup, useStudyGroup, useStudyGroups, type StudyGroup } from '../api/schedules';
import { LMS_FIELDS, LMS_LIMITS } from '../model/fieldSchemas';
import { LmsProblem } from '../LmsUi';

interface GroupForm {
  name: string;
  class_id?: string;
  member_ids: string[];
}

export function StudyGroupForm({ schoolId, groupId, onClose }: { schoolId: string; groupId?: string; onClose: () => void }) {
  const { t } = useTranslation();
  const field = useFieldRules();
  const [form] = Form.useForm<GroupForm>();
  const group = useStudyGroup(schoolId, groupId);
  const classes = useSchoolClasses(schoolId);
  const classId = Form.useWatch('class_id', form) ?? group.data?.class_id;
  const students = useClassStudents(schoolId, classId);
  const save = useSaveStudyGroup(schoolId);
  if (groupId && !group.data && !group.error) return null;
  const lim = LMS_LIMITS.groupMembers;
  return (
    <Modal
      open
      title={groupId ? t('lms.group.editTitle') : t('lms.group.newTitle')}
      okText={t('portal.school_lms.common.save')}
      cancelText={t('portal.school_lms.common.cancel')}
      confirmLoading={save.isPending}
      destroyOnHidden
      onCancel={onClose}
      onOk={async () => {
        const v = await form.validateFields().catch(() => null);
        if (v) save.mutate({ id: groupId, body: { name: v.name.trim(), member_ids: v.member_ids, ...(v.class_id ? { class_id: v.class_id } : {}) } }, { onSuccess: onClose });
      }}
    >
      <LmsProblem error={group.error ?? save.error} />
      <Form form={form} layout="vertical" initialValues={{ name: group.data?.name ?? '', class_id: group.data?.class_id, member_ids: group.data?.member_ids ?? [] }}>
        <Form.Item name="name" label={t('lms.group.name')} rules={field.rules(LMS_FIELDS.studyGroupName, true)}>
          <Input {...field.input(LMS_FIELDS.studyGroupName)} />
        </Form.Item>
        <Form.Item name="class_id" label={t('lms.group.class')}>
          <Select allowClear aria-label={t('lms.group.class')} options={classes.items.map((c) => ({ value: c.class_id, label: c.name }))} onChange={() => form.setFieldValue('member_ids', [])} />
        </Form.Item>
        <Form.Item
          name="member_ids"
          label={t('lms.group.members')}
          rules={[
            {
              validator: (_, ids: string[] = []) =>
                ids.length >= lim.minItems && ids.length <= lim.maxItems ? Promise.resolve() : Promise.reject(new Error(t('lms.form.itemsRange', { min: lim.minItems, max: lim.maxItems }))),
            },
          ]}
        >
          <Select
            mode="multiple"
            showSearch
            optionFilterProp="label"
            aria-label={t('lms.group.members')}
            disabled={!classId}
            options={students.items.map((s) => ({ value: s.student_id, label: s.display_name }))}
          />
        </Form.Item>
      </Form>
    </Modal>
  );
}

function StudyGroupList() {
  const { t } = useTranslation();
  const [params, setParams] = useSearchParams();
  const schoolId = params.get('school') ?? undefined;
  const groups = useStudyGroups(schoolId);
  const del = useDeleteStudyGroup(schoolId ?? '');
  const [editing, setEditing] = useState<string | 'new' | null>(null);
  const columns: ColumnType<StudyGroup>[] = [
    { key: 'name', title: t('lms.group.name'), render: (_, g) => g.name },
    { key: 'members', title: t('lms.group.memberCount'), render: (_, g) => g.member_count ?? g.member_ids.length },
    {
      key: 'actions',
      title: t('portal.school_lms.common.actions'),
      render: (_, g) => (
        <ActionGate actionKey="action.school_lms.study_groups.edit">
          <Space wrap>
            <Button size="small" onClick={() => setEditing(g.study_group_id)}>
              {t('portal.school_lms.common.edit')}
            </Button>
            <Button
              size="small"
              danger
              onClick={() =>
                Modal.confirm({
                  title: t('lms.group.deleteConfirm', { name: g.name }),
                  okText: t('portal.school_lms.common.delete'),
                  cancelText: t('portal.school_lms.common.cancel'),
                  onOk: () => del.mutate(g.study_group_id),
                })
              }
            >
              {t('portal.school_lms.common.delete')}
            </Button>
          </Space>
        </ActionGate>
      ),
    },
  ];
  return (
    <Flex vertical gap="middle">
      <Flex wrap="wrap" gap="middle" justify="space-between">
        <Form.Item label={t('portal.school_lms.common.school')}>
          <SchoolSelect value={schoolId} onChange={(id) => setParams(id ? { school: id } : {}, { replace: true })} />
        </Form.Item>
        {schoolId ? (
          <ActionGate actionKey="action.school_lms.study_groups.edit">
            <Button type="primary" onClick={() => setEditing('new')}>
              {t('lms.group.new')}
            </Button>
          </ActionGate>
        ) : null}
      </Flex>
      <LmsProblem error={groups.error ?? del.error} />
      <ResponsiveTable<StudyGroup>
        rowKey="study_group_id"
        columns={columns}
        data={groups.items}
        loading={groups.isFetching}
        cardTitle={(g) => g.name}
        hasMore={groups.hasMore}
        onLoadMore={groups.loadMore}
      />
      {editing && schoolId ? <StudyGroupForm schoolId={schoolId} groupId={editing === 'new' ? undefined : editing} onClose={() => setEditing(null)} /> : null}
    </Flex>
  );
}

export function StudyGroupsScreen({ title }: { title: string }) {
  return (
    <section aria-labelledby="ew-menu-title">
      <PageTitle title={title} />
      <SingleTenantGate>{() => <StudyGroupList />}</SingleTenantGate>
    </section>
  );
}
