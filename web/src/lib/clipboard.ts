// copyToClipboard receives a text value and returns whether it was copied to the system clipboard.
export async function copyToClipboard(value: string): Promise<boolean> {
  if (typeof navigator === 'undefined' || !navigator.clipboard?.writeText) {
    return false;
  }

  try {
    await navigator.clipboard.writeText(value);
    return true;
  } catch {
    // Clipboard writes can be rejected by the browser (e.g. denied permission); fail silently.
    return false;
  }
}
