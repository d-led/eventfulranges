// Pure helpers for slicing an n-dimensional box into 3D. Kept free of DOM and
// three.js so they can be unit-tested directly.
export const FADE_FRACTION = 0.3; // share of a box's w-span used for the fade
export const FADE_FLOOR = 0.05; // absolute fade width, so thin boxes still fade

// fadeOpacity is 1 well inside a box's w-interval and ramps to 0 just outside
// each edge, so a swept cross-section reads as a blurred slab rather than a
// hard binary cut. Boxes are half-open in w: [minW, maxW).
export function fadeOpacity(minW, maxW, w) {
  const fade = Math.max((maxW - minW) * FADE_FRACTION, FADE_FLOOR);
  return Math.max(0, Math.min(1, (w - minW) / fade, (maxW - w) / fade));
}
