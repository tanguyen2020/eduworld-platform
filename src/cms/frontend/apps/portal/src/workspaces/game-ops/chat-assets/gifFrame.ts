// Static preview of a GIF (F082-AC-05, Clarifications 2026-11-26): the browser draws the first frame on a canvas and
// exports a PNG uploaded as the second file of the GIF version; module chat reads both files back from the bucket and
// checks them (format, size, pixels, not animated) — nothing here is trusted. Injectable for tests (jsdom has no canvas).

export interface GifFrame {
  preview: File;
  width: number;
  height: number;
}

export type GifFrameExtractor = (file: File) => Promise<GifFrame>;

async function canvasFirstFrame(file: File): Promise<GifFrame> {
  const url = URL.createObjectURL(file);
  try {
    const img = await new Promise<HTMLImageElement>((resolve, reject) => {
      const el = new Image();
      el.onload = () => resolve(el);
      el.onerror = () => reject(new Error('gif decode'));
      el.src = url;
    });
    const canvas = document.createElement('canvas');
    canvas.width = img.naturalWidth;
    canvas.height = img.naturalHeight;
    const ctx = canvas.getContext('2d');
    if (!ctx) throw new Error('canvas 2d unavailable');
    ctx.drawImage(img, 0, 0);
    const blob = await new Promise<Blob>((resolve, reject) => canvas.toBlob((b) => (b ? resolve(b) : reject(new Error('png export'))), 'image/png'));
    const base = file.name.replace(/\.[^.]+$/, '');
    return { preview: new File([blob], `${base}-preview.png`, { type: 'image/png' }), width: img.naturalWidth, height: img.naturalHeight };
  } finally {
    URL.revokeObjectURL(url);
  }
}

let extractor: GifFrameExtractor = canvasFirstFrame;

/** First frame of a GIF as a PNG with the GIF dimensions. */
export const gifFirstFrame: GifFrameExtractor = (file) => extractor(file);

/** Test hook (MOCK canvas): replaces the frame extraction. */
export function setGifFrameExtractor(fn: GifFrameExtractor | null): void {
  extractor = fn ?? canvasFirstFrame;
}
