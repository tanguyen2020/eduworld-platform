// F066-AC-25 (pure): early checks of an audio_lesson config — audio and transcript required, segments in order, inside
// the audio, unique and without overlap, question times inside the audio.
import { describe, expect, it } from 'vitest';
import { audioIssues } from './audioRules';

const keys = (c: Parameters<typeof audioIssues>[0], d?: number) => audioIssues(c, d).map((i) => `${i.pointer} ${i.key}`);

describe('audioRules (F066-AC-25)', () => {
  it('a complete config has no issue', () => {
    expect(keys({ audio_asset_id: 'a', transcript_asset_id: 't', segments: [{ segment_id: 's1', start_ms: 0, end_ms: 20_000 }, { segment_id: 's2', start_ms: 20_000, end_ms: 40_000 }], questions: [{ question_version_id: 'q', at_ms: 10_000 }, { question_version_id: 'r' }] }, 60_000)).toEqual([]);
  });

  it('audio and transcript are required', () => {
    expect(keys({})).toEqual(['/config/content/audio_asset_id audio.noAudio', '/config/content/transcript_asset_id audio.noTranscript']);
  });

  it('segments: start before end, inside the audio, unique id, no overlap', () => {
    const out = keys(
      {
        audio_asset_id: 'a',
        transcript_asset_id: 't',
        segments: [
          { segment_id: 's1', start_ms: 0, end_ms: 20_000 },
          { segment_id: 's1', start_ms: 15_000, end_ms: 30_000 },
          { segment_id: 's3', start_ms: 50_000, end_ms: 40_000 },
          { segment_id: 's4', start_ms: 55_000, end_ms: 70_000 },
        ],
      },
      60_000,
    );
    expect(out).toContain('/config/content/segments/1/segment_id audio.segmentDuplicate');
    expect(out).toContain('/config/content/segments/1/start_ms audio.segmentOverlap');
    expect(out).toContain('/config/content/segments/2/end_ms audio.segmentOrder');
    expect(out).toContain('/config/content/segments/3/end_ms audio.afterEnd');
  });

  it('question times must be inside the audio; without a duration only negatives are refused', () => {
    expect(keys({ audio_asset_id: 'a', transcript_asset_id: 't', questions: [{ question_version_id: 'q', at_ms: 60_000 }] }, 60_000)).toEqual(['/config/content/questions/0/at_ms audio.questionOutside']);
    expect(keys({ audio_asset_id: 'a', transcript_asset_id: 't', questions: [{ question_version_id: 'q', at_ms: 90_000 }] })).toEqual([]);
    expect(keys({ audio_asset_id: 'a', transcript_asset_id: 't', questions: [{ question_version_id: 'q', at_ms: -1 }] })).toEqual(['/config/content/questions/0/at_ms audio.questionOutside']);
  });
});
