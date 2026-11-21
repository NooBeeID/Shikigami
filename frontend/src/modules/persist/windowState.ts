let timer: ReturnType<typeof setTimeout> | null = null;

export async function saveWindowNow(): Promise<void> {
  if (timer) {
    clearTimeout(timer);
    timer = null;
  }
  try {
    const b = (window as any).go?.main?.Bridge;
    if (b?.WindowSaveState) await b.WindowSaveState();
  } catch {
    // diabaikan
  }
}

export function scheduleWindowSave(): void {
  if (timer) clearTimeout(timer);
  timer = setTimeout(() => {
    timer = null;
    void saveWindowNow();
  }, 500);
}

export function startWindowWatch(): () => void {
  window.addEventListener('resize', scheduleWindowSave);
  return () => window.removeEventListener('resize', scheduleWindowSave);
}
