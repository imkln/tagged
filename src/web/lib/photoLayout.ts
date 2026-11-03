export type PhotoRow = { start: number; count: number; height: number };

export function layoutPhotos(
  ratios: number[],
  width: number,
  targetHeight: number,
  gap: number,
): PhotoRow[] {
  if (width <= 0) return [];
  const rows: PhotoRow[] = [];
  let start = 0;
  let totalRatio = 0;
  for (let index = 0; index < ratios.length; index++) {
    totalRatio += ratios[index];
    const count = index - start + 1;
    if (totalRatio * targetHeight + gap * (count - 1) >= width) {
      rows.push({
        start,
        count,
        height: (width - gap * (count - 1)) / totalRatio,
      });
      start = index + 1;
      totalRatio = 0;
    }
  }
  if (start < ratios.length) {
    const count = ratios.length - start;
    rows.push({
      start,
      count,
      height: Math.min(targetHeight, (width - gap * (count - 1)) / totalRatio),
    });
  }
  return rows;
}
