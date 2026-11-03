import { expect, it } from 'vitest';
import { layoutPhotos } from './photoLayout';

it.each([390, 768, 1440])(
  'fits mixed orientations into a %ipx container without changing proportions or order',
  (width) => {
    const ratios = [1.5, 2 / 3, 1.5, 2 / 3, 2 / 3, 1.5, 1.5];
    const rows = layoutPhotos(ratios, width, 175, 4);
    expect(
      rows.flatMap((row) =>
        Array.from({ length: row.count }, (_, i) => row.start + i),
      ),
    ).toEqual(ratios.map((_, i) => i));
    for (const [index, row] of rows.entries()) {
      const occupied =
        ratios
          .slice(row.start, row.start + row.count)
          .reduce((total, ratio) => total + ratio * row.height, 0) +
        (row.count - 1) * 4;
      expect(row.height).toBeGreaterThan(0);
      expect(occupied).toBeLessThanOrEqual(width + 0.01);
      if (index < rows.length - 1) expect(occupied).toBeCloseTo(width);
    }
  },
);

it('does not enlarge a sparse final row beyond the target height', () => {
  expect(layoutPhotos([2 / 3], 1440, 175, 4)).toEqual([
    { start: 0, count: 1, height: 175 },
  ]);
});

it('handles an empty library and waits for a measurable container', () => {
  expect(layoutPhotos([], 390, 175, 4)).toEqual([]);
  expect(layoutPhotos([1.5], 0, 175, 4)).toEqual([]);
});
