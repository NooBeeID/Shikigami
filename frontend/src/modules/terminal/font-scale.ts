export const BASE_FONT_SIZE = 13;

interface Instance {
  term: { options: { fontSize?: number } };
  refit: () => void;
}

const instances = new Set<Instance>();
let scalePercent = 100;

export function currentTerminalFontSize(): number {
  return Math.round((BASE_FONT_SIZE * scalePercent) / 100);
}

export function registerTerminalInstance(
  term: { options: { fontSize?: number } },
  refit: () => void,
): () => void {
  const inst: Instance = { term, refit };
  instances.add(inst);
  return () => {
    instances.delete(inst);
  };
}

export function setTerminalFontScale(percent: number): void {
  scalePercent = percent;
  const size = currentTerminalFontSize();
  for (const inst of instances) {
    inst.term.options.fontSize = size;
    try {
      inst.refit();
    } catch {
      /* host tersembunyi */
    }
  }
}
