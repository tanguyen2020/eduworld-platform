// DialogueGraphEditor (F079-AC-06, npc-dialogue.schema.json, ADMIN_PORTAL §3.3): an NPC dialogue as a graph — nodes
// (speaker, bilingual text, emotion, actions) linked by `next` or by up to 4 choices (bilingual text, condition,
// actions, target node). The start node and every link are picked among the node ids (no free id); the overview lists
// each node with its outgoing edges and flags nodes no path reaches (display of the graph; the server validates links).
import { effective, SchemaForm, type JsonSchema, type SchemaFieldContext } from '@eduworld/ops-ui/game-console';
import { SearchSelect } from '@eduworld/ops-ui';
import { Alert, Button, Card, Collapse, Flex, Tag, Typography } from 'antd';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { gameOverrides } from '../fields';
import { GenericSchemaBuilder } from '../GenericSchemaBuilder';
import { useFieldLabel } from '../labels';
import type { BuilderProps } from '../types';

type DialogueNode = Record<string, unknown> & { node_id?: string; next?: string; choices?: { next?: string }[] };

/** Outgoing edges of a node (`next` or each choice). */
export function edgesOf(n: DialogueNode): string[] {
  const out = typeof n.next === 'string' ? [n.next] : [];
  for (const c of n.choices ?? []) if (typeof c.next === 'string') out.push(c.next);
  return out;
}

/** Node ids reachable from the start node (graph walk). */
export function reachable(nodes: readonly DialogueNode[], start: string | undefined): Set<string> {
  const byId = new Map(nodes.map((n) => [n.node_id, n]));
  const seen = new Set<string>();
  const stack = start ? [start] : [];
  while (stack.length) {
    const id = stack.pop()!;
    if (seen.has(id) || !byId.has(id)) continue;
    seen.add(id);
    stack.push(...edgesOf(byId.get(id)!));
  }
  return seen;
}

export function nextNodeId(nodes: readonly DialogueNode[]): string {
  const used = new Set(nodes.map((n) => n.node_id));
  let n = nodes.length + 1;
  while (used.has(`n${n}`)) n++;
  return `n${n}`;
}

export function DialogueGraphEditor(props: BuilderProps) {
  const { def, value, onChange, readOnly, errors } = props;
  const { t, i18n } = useTranslation();
  const label = useFieldLabel();
  const lang = i18n.language.startsWith('en') ? 'en' : 'vi';
  const schema = def.schema!;
  const nodeSchema = effective((schema.properties!.nodes as JsonSchema).items, schema);
  const nodes = (Array.isArray(value.nodes) ? value.nodes : []) as DialogueNode[];
  const start = value.start_node_id as string | undefined;
  const ids = nodes.map((n) => n.node_id).filter((x): x is string => !!x);
  const reach = reachable(nodes, start);
  const unreachable = ids.filter((id) => !reach.has(id));
  const setNodes = (next: DialogueNode[]) => onChange({ ...value, nodes: next.length ? next : undefined } as Record<string, unknown>);
  const nodeOptions = ids.map((id) => ({ value: id, label: id }));
  const base = gameOverrides('');
  const override = (ctx: SchemaFieldContext): ReactNode | undefined => {
    // links of the graph: `next` of a node or a choice → one of the node ids
    if (ctx.key === 'next') {
      return <SearchSelect id={ctx.id} aria-label={ctx.label} allowClear disabled={ctx.readOnly} value={ctx.value as string | undefined} onChange={(v) => ctx.onChange(v ?? undefined)} options={nodeOptions} />;
    }
    return base(ctx);
  };

  return (
    <Flex vertical gap="middle">
      <GenericSchemaBuilder {...props} omitKeys={['nodes', 'start_node_id']} />
      <Card size="small" title={t('gameOps.npc.graph')} data-testid="dialogue-graph">
        <Flex vertical gap="small">
          <Flex gap="small" align="center">
            <Typography.Text>{t('gameOps.npc.start')}</Typography.Text>
            <SearchSelect
              aria-label={t('gameOps.npc.start')}
              disabled={readOnly}
              value={start}
              onChange={(v) => onChange({ ...value, start_node_id: v } as Record<string, unknown>)}
              options={nodeOptions}
              status={errors.start_node_id ? 'error' : undefined}
            />
          </Flex>
          <ul aria-label={t('gameOps.npc.edges')}>
            {nodes.map((n, i) => (
              <li key={`${n.node_id ?? i}`} data-testid={`dialogue-edge-${n.node_id}`}>
                <Tag color={n.node_id === start ? 'blue' : undefined}>{n.node_id}</Tag>
                {edgesOf(n).length ? `→ ${edgesOf(n).join(', ')}` : t('gameOps.npc.end')}
              </li>
            ))}
          </ul>
          {unreachable.length > 0 && <Alert type="warning" showIcon message={t('gameOps.npc.unreachable', { ids: unreachable.join(', ') })} />}
        </Flex>
      </Card>
      <Card
        size="small"
        title={t('gameOps.npc.nodes', { count: nodes.length })}
        extra={
          !readOnly && (
            <Button type="primary" onClick={() => setNodes([...nodes, { node_id: nextNodeId(nodes), speaker: 'npc' }])}>
              {t('gameOps.npc.addNode')}
            </Button>
          )
        }
      >
        <Collapse
          size="small"
          items={nodes.map((n, i) => ({
            key: String(i),
            label: (
              <Flex gap="small" align="center">
                <Tag>{n.node_id}</Tag>
                <Typography.Text type="secondary">{String((n.text as Record<string, string> | undefined)?.[lang] ?? '')}</Typography.Text>
              </Flex>
            ),
            extra: readOnly ? null : (
              <Button size="small" danger aria-label={t('gameOps.npc.removeNode', { id: n.node_id })} onClick={(e) => (e.stopPropagation(), setNodes(nodes.filter((_, j) => j !== i)))}>
                {t('gameOps.common.remove')}
              </Button>
            ),
            children: (
              <SchemaForm
                schema={{ ...nodeSchema, $defs: schema.$defs } as JsonSchema}
                value={n}
                basePath={['nodes', i]}
                errors={errors}
                readOnly={readOnly}
                label={label}
                renderOverride={override}
                idPrefix={`dialogue-node-${i}`}
                onChange={(v) => setNodes(nodes.map((x, j) => (j === i ? (v as DialogueNode) : x)))}
              />
            ),
          }))}
        />
      </Card>
    </Flex>
  );
}
