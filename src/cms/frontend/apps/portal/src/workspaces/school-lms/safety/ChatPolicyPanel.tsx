// School chat policy (F122 US8; F122-AC-21, AC-22, AC-28; FR-MOD-004, ADR-0020, ADR-0078, ADR-0095): getChatPolicy (scope
// school) + getEffectiveChatPolicy per channel type. Controls per channel type only — `direct`, `world`, `area`: on/off,
// narrower hours, slow mode, stricter `strictness`; `match`/`team`/`party` are locked "Luôn bật" (only slow mode ≤ 10 s and
// strictness); no class/study_group channel, no grade or age control, no real-name option. The values of the levels above
// are the limits shown next to each control; a channel without a school rule says "Kế thừa". The school preset
// (`standard` / `quick_message_emote_only`) applies to the whole school. Save = putChatPolicy with If-Match and a mandatory
// reason; CHAT_POLICY_RELAX_NOT_ALLOWED / CHAT_POLICY_ALWAYS_ON / VALIDATION_FAILED land on the channel card they name.
// Only the shape of the request is built here: the server validates every rule (AGENTS R1).
import { Alert, Button, Card, Flex, Form, Input, InputNumber, Radio, Space, Switch, Tag, TimePicker, Typography } from 'antd';
import { AlwaysOnLock, openapiInputProps, openapiRules, OPS_UI_NS, SearchSelect } from '@eduworld/ops-ui';
import dayjs, { type Dayjs } from 'dayjs';
import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { actionStatus } from '../../../permissions/menuRegistry';
import { usePermissions } from '../../../permissions/usePermissions';
import { ApiError } from '../../../shared/api/middleware/problem';
import { LIMITS, SafetyError } from '../../../shared/chatSafety/ui';
import { SchoolSelect } from '../common/SchoolSelect';
import { ALWAYS_ON, POLICY_CHANNELS, useEffectivePolicies, usePutSchoolPolicy, useSchoolPolicy, type ChannelType, type ChatPolicyPut, type EffectivePolicy } from './api';

const STRICTNESS = ['standard', 'strict', 'very_strict'] as const;
type Strictness = (typeof STRICTNESS)[number];
const PRESETS = ['standard', 'quick_message_emote_only'] as const;
/** Slow mode cap of the always-on gameplay chat (chat-policy.schema.json, ADR-0020 §6). */
const ALWAYS_ON_SLOW_MODE_MAX = 10;
const SLOW_MODE_MAX = 300;

type Rule = Record<string, unknown> & { channel_type: ChannelType };

interface CardState {
  /** Whether the school sets a rule for this channel (false = inherited). */
  own: boolean;
  enabled: boolean;
  from?: string;
  to?: string;
  slowMode: number;
  strictness: Strictness;
}

function initialCard(ch: ChannelType, rule: Rule | undefined, eff: EffectivePolicy | null): CardState {
  const hours = (rule?.allowed_hours as { from: string; to: string }[] | undefined)?.[0];
  const validation = rule?.validation as { strictness?: Strictness } | undefined;
  return {
    own: !!rule,
    enabled: ALWAYS_ON.includes(ch) ? true : rule ? rule.enabled !== false : !!eff?.enabled,
    from: hours?.from,
    to: hours?.to,
    slowMode: Number(rule?.slow_mode_seconds ?? eff?.slow_mode_seconds ?? 0),
    strictness: validation?.strictness ?? 'standard',
  };
}

/** Rule of the school level for one channel (schema v5 shape; the limits of the levels above come from `eff`). */
export function buildRule(ch: ChannelType, s: CardState, eff: EffectivePolicy | null): Rule {
  if (!s.enabled && !ALWAYS_ON.includes(ch)) return { channel_type: ch, enabled: false, allowed_kinds: [] };
  const kinds = eff?.allowed_kinds ?? ['emoji', 'quick'];
  const rule: Rule = { channel_type: ch, enabled: true, allowed_kinds: kinds, slow_mode_seconds: s.slowMode, validation: { strictness: s.strictness } };
  if (kinds.includes('text')) {
    rule.max_text_length = eff?.max_text_length ?? 1;
    rule.rate = eff?.rate ?? { messages_per_minute: 1 };
  }
  if (!ALWAYS_ON.includes(ch) && s.from && s.to) rule.allowed_hours = [{ days: 'mon-sun', from: s.from, to: s.to }];
  if (ALWAYS_ON.includes(ch) && eff?.features) rule.features = eff.features;
  return rule;
}

const hhmm = (v?: string) => (v ? dayjs(v, 'HH:mm') : null);

function ChannelCard({ ch, state, eff, error, onChange, readOnly }: {
  ch: ChannelType;
  state: CardState;
  eff: EffectivePolicy | null;
  error?: string;
  onChange: (s: CardState) => void;
  readOnly: boolean;
}) {
  const { t } = useTranslation();
  const alwaysOn = ALWAYS_ON.includes(ch);
  const limitHours = (eff?.allowed_hours ?? []).map((h) => `${h.from}–${h.to}`).join(', ');
  const set = (patch: Partial<CardState>) => onChange({ ...state, ...patch, own: true });
  return (
    <Card
      size="small"
      title={t(`chatSafety.channel.${ch}`)}
      extra={state.own ? <Tag color="processing">{t('chatSafety.policy.own')}</Tag> : <Tag>{t('chatSafety.policy.inherited')}</Tag>}
      data-testid={`policy-card-${ch}`}
    >
      <Flex vertical gap="small">
        {alwaysOn ? (
          <AlwaysOnLock reason={t('chatSafety.policy.alwaysOnReason')} />
        ) : (
          <Flex align="center" gap="small">
            <Switch checked={state.enabled} disabled={readOnly || !eff?.enabled} onChange={(v) => set({ enabled: v })} aria-label={t('chatSafety.policy.enabledOf', { channel: t(`chatSafety.channel.${ch}`) })} />
            <Typography.Text>{state.enabled ? t('chatSafety.common.on') : t('chatSafety.common.off')}</Typography.Text>
            {!eff?.enabled ? <Typography.Text type="secondary">{t('chatSafety.policy.offAbove')}</Typography.Text> : null}
          </Flex>
        )}
        {!alwaysOn && state.enabled ? (
          <Space wrap>
            <Typography.Text>{t('chatSafety.policy.hours')}</Typography.Text>
            <TimePicker
              format="HH:mm"
              aria-label={t('chatSafety.policy.from', { channel: t(`chatSafety.channel.${ch}`) })}
              value={hhmm(state.from)}
              disabled={readOnly}
              onChange={(v: Dayjs | null) => set({ from: v ? v.format('HH:mm') : undefined })}
            />
            <TimePicker
              format="HH:mm"
              aria-label={t('chatSafety.policy.to', { channel: t(`chatSafety.channel.${ch}`) })}
              value={hhmm(state.to)}
              disabled={readOnly}
              onChange={(v: Dayjs | null) => set({ to: v ? v.format('HH:mm') : undefined })}
            />
            <Typography.Text type="secondary">{limitHours ? t('chatSafety.policy.hoursLimit', { hours: limitHours }) : t('chatSafety.policy.hoursCurfew')}</Typography.Text>
          </Space>
        ) : null}
        {state.enabled ? (
          <Space wrap>
            <Typography.Text>{t('chatSafety.policy.slowMode')}</Typography.Text>
            <InputNumber
              min={eff?.slow_mode_seconds ?? 0}
              max={alwaysOn ? ALWAYS_ON_SLOW_MODE_MAX : SLOW_MODE_MAX}
              value={state.slowMode}
              disabled={readOnly}
              aria-label={t('chatSafety.policy.slowModeOf', { channel: t(`chatSafety.channel.${ch}`) })}
              onChange={(v) => set({ slowMode: Number(v ?? 0) })}
            />
            <Typography.Text type="secondary">{t('chatSafety.policy.slowModeLimit', { min: eff?.slow_mode_seconds ?? 0, max: alwaysOn ? ALWAYS_ON_SLOW_MODE_MAX : SLOW_MODE_MAX })}</Typography.Text>
            <Typography.Text>{t('chatSafety.policy.strictness')}</Typography.Text>
            <SearchSelect
              aria-label={t('chatSafety.policy.strictnessOf', { channel: t(`chatSafety.channel.${ch}`) })}
              value={state.strictness}
              disabled={readOnly}
              onChange={(v: Strictness) => set({ strictness: v })}
              options={STRICTNESS.map((v) => ({ value: v, label: t(`chatSafety.strictness.${v}`) }))}
              style={{ minWidth: '9rem' }}
            />
          </Space>
        ) : null}
        {error ? <Alert type="error" showIcon role="alert" message={error} /> : null}
      </Flex>
    </Card>
  );
}

function SchoolPolicyEditor({ schoolId }: { schoolId: string }) {
  const { t } = useTranslation();
  const ops = useTranslation(OPS_UI_NS);
  const doc = useSchoolPolicy(schoolId);
  const eff = useEffectivePolicies(schoolId);
  const put = usePutSchoolPolicy(schoolId);
  const [form] = Form.useForm<{ reason: string }>();
  const [edits, setEdits] = useState<Partial<Record<ChannelType, CardState>>>({});
  const [preset, setPreset] = useState<(typeof PRESETS)[number] | undefined>();
  const rules = useMemo(() => new Map(((doc.data?.rules ?? []) as Rule[]).map((r) => [r.channel_type, r])), [doc.data]);
  const [saved, setSaved] = useState(false);
  const { view } = usePermissions();
  // without the edit action (permission or feature toggle) the policy is shown read only (ADR-0090; the server refuses anyway)
  const readOnly = !view || actionStatus('action.school_lms.chat_safety.edit', view) !== 'enabled';
  if (doc.error) return <SafetyError error={doc.error} />;
  if (eff.error) return <SafetyError error={eff.error} />;
  if (doc.isPending || eff.loading) return <Card loading />;
  const stateOf = (ch: ChannelType) => edits[ch] ?? initialCard(ch, rules.get(ch), eff.byChannel[ch]);
  const currentPreset = preset ?? doc.data?.chat_policy_preset ?? 'standard';
  const err = put.error instanceof ApiError ? put.error : undefined;
  const cardError = (ch: ChannelType): string | undefined => {
    if (!err) return undefined;
    if ((err.code === 'CHAT_POLICY_RELAX_NOT_ALLOWED' || err.code === 'CHAT_POLICY_ALWAYS_ON') && err.extensions.channel_type === ch) {
      return t(`error.${err.code}`) + (err.extensions.field ? ` (${t(`chatSafety.policy.field.${String(err.extensions.field).split('.')[0]}`, { defaultValue: String(err.extensions.field) })})` : '');
    }
    if (err.code === 'VALIDATION_FAILED') {
      const sent = POLICY_CHANNELS.filter((c) => edits[c]?.own || rules.has(c));
      const idx = sent.indexOf(ch);
      return Object.keys(err.fieldErrors).some((f) => f.startsWith(`rules/${idx}`) || f.startsWith(`rules[${idx}]`) || f.startsWith(`/rules/${idx}`)) ? t('error.VALIDATION_FAILED') : undefined;
    }
    return undefined;
  };
  const cardCodes = ['CHAT_POLICY_RELAX_NOT_ALLOWED', 'CHAT_POLICY_ALWAYS_ON'];
  const save = (v: { reason: string }) => {
    setSaved(false);
    const channels = POLICY_CHANNELS.filter((c) => edits[c]?.own || rules.has(c));
    const body: ChatPolicyPut = {
      rules: channels.map((c) => buildRule(c, stateOf(c), eff.byChannel[c])) as ChatPolicyPut['rules'],
      chat_policy_preset: currentPreset,
      reason: v.reason.trim(),
    };
    if (!body.rules.length) body.rules = [buildRule('direct', stateOf('direct'), eff.byChannel.direct)] as ChatPolicyPut['rules'];
    put.mutate({ body, version: doc.data?.version ?? 0 }, { onSuccess: () => { setEdits({}); setPreset(undefined); form.resetFields(); setSaved(true); } });
  };
  return (
    <Flex vertical gap="middle">
      {!doc.data ? <Alert type="info" showIcon message={t('chatSafety.policy.noSchoolPolicy')} /> : null}
      <Card size="small" title={t('chatSafety.policy.preset')} data-testid="preset-card">
        <Radio.Group value={currentPreset} disabled={readOnly} onChange={(e) => setPreset(e.target.value as (typeof PRESETS)[number])}>
          <Flex vertical gap="small">
            {PRESETS.map((p) => (
              <Radio key={p} value={p}>
                <Typography.Text strong>{t(`chatSafety.preset.${p}`)}</Typography.Text> — <Typography.Text type="secondary">{t(`chatSafety.preset.${p}Help`)}</Typography.Text>
              </Radio>
            ))}
          </Flex>
        </Radio.Group>
      </Card>
      {POLICY_CHANNELS.map((ch) => (
        <ChannelCard key={ch} ch={ch} state={stateOf(ch)} eff={eff.byChannel[ch]} error={cardError(ch)} readOnly={readOnly} onChange={(s) => setEdits((e) => ({ ...e, [ch]: s }))} />
      ))}
      <Typography.Text type="secondary">{t('chatSafety.policy.cacheNote')}</Typography.Text>
      {!readOnly ? (
        <Form form={form} layout="vertical" name="school_chat_policy" onFinish={save}>
          <Form.Item name="reason" label={t('chatSafety.common.reason')} rules={openapiRules(LIMITS.policyReason, { required: true, t: ops.t })}>
            <Input.TextArea autoSize={{ minRows: 2, maxRows: 5 }} {...openapiInputProps(LIMITS.policyReason)} />
          </Form.Item>
          {put.error && !(err && (cardCodes.includes(err.code) || (err.code === 'VALIDATION_FAILED' && POLICY_CHANNELS.some((c) => cardError(c))))) ? <SafetyError error={put.error} /> : null}
          {err && cardCodes.includes(err.code) ? <Alert type="error" showIcon message={t(`error.${err.code}`)} /> : null}
          {saved ? <Alert type="success" showIcon message={t('chatSafety.policy.saved')} /> : null}
          <Button type="primary" htmlType="submit" loading={put.isPending}>
            {t('chatSafety.common.save')}
          </Button>
        </Form>
      ) : null}
    </Flex>
  );
}

export function ChatPolicyPanel() {
  const { t } = useTranslation();
  const [schoolId, setSchoolId] = useState<string | undefined>();
  return (
    <Flex vertical gap="middle">
      <Form layout="inline" name="chat_policy_school">
        <Form.Item label={t('chatSafety.common.school')}>
          <SchoolSelect value={schoolId} onChange={setSchoolId} />
        </Form.Item>
      </Form>
      {schoolId ? <SchoolPolicyEditor key={schoolId} schoolId={schoolId} /> : null}
    </Flex>
  );
}
