// Pure camera-framing helpers. Kept free of DOM and three.js so the fit math
// can be unit-tested directly, like slice.js.

// orthoHalf returns the orthographic half-height that frames the given
// axis-aligned bounds (or null) so both extents fit without clipping. The
// horizontal extent is divided by the canvas aspect, because the frustum
// width is expressed in units of the half-height.
export function orthoHalf(bounds, aspect) {
  if (!bounds) return 4;
  const halfY = (bounds.max[1] - bounds.min[1]) / 2;
  const halfX = (bounds.max[0] - bounds.min[0]) / 2;
  return Math.max(1, halfY, halfX / aspect);
}

// orthoFrustum returns the orthographic frustum planes for a camera looking
// straight down at material centred on its own local origin. The planes are
// symmetric around the origin because the camera is positioned above the
// material's centre; folding the centre into the planes as well would shift
// the frame twice and push the material out of view.
export function orthoFrustum(half, aspect) {
  return {
    left: -half * aspect,
    right: half * aspect,
    top: half,
    bottom: -half,
  };
}

// perspDistance returns the distance from which a bounding sphere of the given
// radius fits the narrower of the two fields of view, with a little breathing
// room.
export function perspDistance(radius, fovDeg, aspect) {
  const fov = (fovDeg * Math.PI) / 180;
  const hFov = 2 * Math.atan(Math.tan(fov / 2) * aspect);
  const halfFov = Math.min(fov, hFov) / 2;
  return (radius / Math.sin(halfFov)) * 1.15;
}
