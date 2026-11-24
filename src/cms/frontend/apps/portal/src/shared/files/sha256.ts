// sha256 of a file computed in the browser (ADR-0044 media keys media/<sha256>/…, presign requests of F027 and F078).

function bytesOf(file: Blob): Promise<ArrayBuffer> {
  if (typeof file.arrayBuffer === 'function') return file.arrayBuffer();
  return new Promise((resolve, reject) => {
    const r = new FileReader();
    r.onload = () => resolve(r.result as ArrayBuffer);
    r.onerror = () => reject(r.error);
    r.readAsArrayBuffer(file);
  });
}

export async function sha256OfFile(file: Blob): Promise<string> {
  // copy into this realm's typed array (a test DOM may hand out a foreign ArrayBuffer)
  const bytes = Uint8Array.from(new Uint8Array(await bytesOf(file)));
  const digest = await crypto.subtle.digest('SHA-256', bytes);
  return Array.from(new Uint8Array(digest), (b) => b.toString(16).padStart(2, '0')).join('');
}
