// Props shared by the builders of workspace A (F079 US2–US4): a controlled payload editor. `item` is the saved version
// (absent while creating) — needed by the server simulation and the preview.
import type { ConfigTypeDef } from '../registry/configTypes';

export interface SavedVersion {
  configId: string;
  version: number;
  /** Bumped when the server copy changed (autosave): server views (simulation) refresh. */
  revision: string | null;
}

export interface BuilderProps {
  def: ConfigTypeDef;
  value: Record<string, unknown>;
  onChange: (value: Record<string, unknown>) => void;
  readOnly: boolean;
  /** Server field errors by field path. */
  errors: Record<string, string>;
  item?: SavedVersion;
}
