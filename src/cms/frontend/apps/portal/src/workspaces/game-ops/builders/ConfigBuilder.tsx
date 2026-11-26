// ConfigBuilder (F079 T017, T021, T026, T031): the builder of a config type — quest steps, NPC dialogue graph, Level & XP,
// event wizard (steps 1–5 when editing), season track, event template, sticker pack / GIF of module chat (F082), or the
// generic schema form.
import { EventWizard } from './event/EventWizard';
import { EventTemplateEditor } from './event/EventTemplateEditor';
import { GifBuilder } from '../chat-assets/GifBuilder';
import { StickerPackBuilder } from '../chat-assets/StickerPackBuilder';
import { GenericSchemaBuilder } from './GenericSchemaBuilder';
import { DialogueGraphEditor } from './npc/DialogueGraphEditor';
import { ProgressionCurveEditor } from './progression/ProgressionCurveEditor';
import { QuestStepEditor } from './quest/QuestStepEditor';
import { SeasonTrackEditor } from './season/SeasonTrackEditor';
import type { BuilderProps } from './types';

export function ConfigBuilder(props: BuilderProps) {
  switch (props.def.builder) {
    case 'quest':
      return <QuestStepEditor {...props} />;
    case 'npc':
      return <DialogueGraphEditor {...props} />;
    case 'progression':
      return <ProgressionCurveEditor {...props} />;
    case 'event':
      return <EventWizard {...props} />;
    case 'season':
      return <SeasonTrackEditor {...props} />;
    case 'event-template':
      return <EventTemplateEditor {...props} />;
    case 'sticker-pack':
      return <StickerPackBuilder {...props} />;
    case 'gif':
      return <GifBuilder {...props} />;
    default:
      return <GenericSchemaBuilder {...props} />;
  }
}
