// Onboarding walkthrough, ported from the commentary project's wide-mode intro
// tour (same visuals and behaviour): a yellow sticky-note bubble with a pointer
// and arrows walks a new visitor through the controls. It runs once per
// browser (a localStorage flag), and can be reopened at any time from the help
// button.
//
// The visual styles are injected here so the two demos share one identical
// implementation; the steps and their target elements are supplied by the
// caller (see app.js in each demo).

const STORAGE_KEY = 'eventfulranges.tour.done.v1';
const TOUR_ID = 'eventfulranges-tour';
const ARROWS_ID = 'eventfulranges-tour-arrows';
const TARGET_CLASS = 'eventfulranges-tour-target';

const CSS = `
.${TARGET_CLASS} {
  outline: none;
}

#${TOUR_ID} {
  --eventfulranges-tour-note-bg: #fff4b3;
  --eventfulranges-tour-note-border: #d4c071;
  --eventfulranges-tour-note-text: #2f2a11;
  --eventfulranges-tour-note-btn-bg: #fff8d7;
  --eventfulranges-tour-note-btn-border: #ccb86a;

  position: fixed;
  z-index: 90;
  width: min(340px, calc(100vw - 16px));
  border-radius: 12px;
  border: 1px solid var(--eventfulranges-tour-note-border);
  background: var(--eventfulranges-tour-note-bg);
  color: var(--eventfulranges-tour-note-text);
  box-shadow: 0 14px 40px rgb(0 0 0 / 20%);
  padding: 12px 12px 10px;
}

#${ARROWS_ID} {
  position: fixed;
  inset: 0;
  z-index: 91;
  pointer-events: none;
}

.eventfulranges-tour-arrow {
  --eventfulranges-tour-arrow-angle: 0rad;
  --eventfulranges-tour-arrow-fill: var(--eventfulranges-tour-note-bg, #fff4b3);
  --eventfulranges-tour-arrow-outline: color-mix(in oklab, CanvasText 34%, transparent);

  position: fixed;
  height: 8px;
  background: var(--eventfulranges-tour-arrow-fill);
  border-radius: 999px;
  transform-origin: 0 50%;
  transform: rotate(var(--eventfulranges-tour-arrow-angle));
  opacity: 0.78;
  box-shadow:
    0 0 0 1px var(--eventfulranges-tour-arrow-outline),
    0 2px 10px rgb(168 137 0 / 12%);
}

.eventfulranges-tour-arrow-head {
  position: absolute;
  right: -10px;
  top: 50%;
  width: 0;
  height: 0;
  border-top: 7px solid transparent;
  border-bottom: 7px solid transparent;
  border-left: 11px solid var(--eventfulranges-tour-arrow-fill);
  transform: translateY(-50%);
}

.eventfulranges-tour-arrow-head::before {
  content: "";
  position: absolute;
  left: -12px;
  top: -8px;
  width: 0;
  height: 0;
  border-top: 8px solid transparent;
  border-bottom: 8px solid transparent;
  border-left: 12px solid var(--eventfulranges-tour-arrow-outline);
  z-index: -1;
}

.eventfulranges-tour-pointer {
  position: absolute;
  width: 0;
  height: 0;
}

#${TOUR_ID}[data-side="below"] .eventfulranges-tour-pointer {
  top: -8px;
  left: var(--eventfulranges-tour-pointer-left, 18px);
  border-left: 8px solid transparent;
  border-right: 8px solid transparent;
  border-bottom: 8px solid var(--eventfulranges-tour-note-bg);
}

#${TOUR_ID}[data-side="below"] .eventfulranges-tour-pointer::before {
  content: "";
  position: absolute;
  left: -9px;
  top: -1px;
  border-left: 9px solid transparent;
  border-right: 9px solid transparent;
  border-bottom: 9px solid color-mix(in oklab, CanvasText 32%, transparent);
  z-index: -1;
}

#${TOUR_ID}[data-side="above"] .eventfulranges-tour-pointer {
  bottom: -8px;
  left: var(--eventfulranges-tour-pointer-left, 18px);
  border-left: 8px solid transparent;
  border-right: 8px solid transparent;
  border-top: 8px solid var(--eventfulranges-tour-note-bg);
}

#${TOUR_ID}[data-side="above"] .eventfulranges-tour-pointer::before {
  content: "";
  position: absolute;
  left: -9px;
  bottom: -1px;
  border-left: 9px solid transparent;
  border-right: 9px solid transparent;
  border-top: 9px solid color-mix(in oklab, CanvasText 32%, transparent);
  z-index: -1;
}

.eventfulranges-tour-title {
  font-weight: 700;
  margin: 0 0 6px;
  line-height: 1.25;
}

.eventfulranges-tour-body {
  margin: 0 0 10px;
  font-size: 13px;
  line-height: 1.45;
}

.eventfulranges-tour-step-action {
  margin: 0 0 10px;
  border: 1px solid var(--eventfulranges-tour-note-btn-border);
  border-radius: 8px;
  background: var(--eventfulranges-tour-note-btn-bg);
  color: var(--eventfulranges-tour-note-text);
  padding: 6px 10px;
  cursor: pointer;
  font-size: 12px;
  font-family: inherit;
}

.eventfulranges-tour-step-action:hover {
  background: color-mix(in oklab, var(--eventfulranges-tour-note-btn-bg) 88%, CanvasText);
}

.eventfulranges-tour-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  font-size: 12px;
}

.eventfulranges-tour-actions {
  display: flex;
  gap: 6px;
  flex-wrap: wrap;
}

.eventfulranges-tour-actions button {
  border: 1px solid var(--eventfulranges-tour-note-btn-border);
  border-radius: 8px;
  background: var(--eventfulranges-tour-note-btn-bg);
  color: var(--eventfulranges-tour-note-text);
  padding: 4px 8px;
  cursor: pointer;
  font-size: 12px;
  font-family: inherit;
}

.eventfulranges-tour-actions button:hover {
  background: color-mix(in oklab, var(--eventfulranges-tour-note-btn-bg) 88%, CanvasText);
}

.eventfulranges-tour-actions button:focus-visible,
.eventfulranges-tour-step-action:focus-visible {
  outline: 2px solid color-mix(in oklab, #5c5200 58%, Canvas);
  outline-offset: 1px;
}

.eventfulranges-help-tour-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  padding: 0;
  border-radius: 8px;
  border: 1px solid transparent;
  background: transparent;
  color: inherit;
  cursor: pointer;
}

.eventfulranges-help-tour-btn:hover {
  background: rgb(255 255 255 / 10%);
}

.eventfulranges-help-tour-btn:focus-visible {
  outline: 2px solid #fff4b3;
  outline-offset: 1px;
}
`;

let stylesInstalled = false;

function ensureStyles() {
  if (stylesInstalled) return;
  stylesInstalled = true;
  const style = document.createElement('style');
  style.id = 'eventfulranges-tour-style';
  style.textContent = CSS;
  document.head.appendChild(style);
}

function readFlag() {
  try {
    return localStorage.getItem(STORAGE_KEY) === '1';
  } catch {
    return false;
  }
}

function writeFlag() {
  try {
    localStorage.setItem(STORAGE_KEY, '1');
  } catch {
    // best effort: the tour simply shows again next visit
  }
}

function targetsForStep(step) {
  const selectors =
    Array.isArray(step.targetSelectors) && step.targetSelectors.length > 0
      ? step.targetSelectors
      : step.targetSelector
        ? [step.targetSelector]
        : [];
  const targets = [];
  const seen = new Set();
  for (const selector of selectors) {
    const found = document.querySelector(selector);
    if (!(found instanceof HTMLElement) || seen.has(found)) continue;
    seen.add(found);
    targets.push(found);
  }
  return targets;
}

function isVisible(target) {
  if (target.hidden) return false;
  const style = window.getComputedStyle(target);
  if (style.display === 'none' || style.visibility === 'hidden') return false;
  const rect = target.getBoundingClientRect();
  return rect.width > 0 && rect.height > 0;
}

function visibleTargetsFor(steps, current) {
  const step = steps[current];
  if (!step) return [];
  return targetsForStep(step).filter(isVisible);
}

function clearTourUi() {
  for (const el of document.querySelectorAll(`.${TARGET_CLASS}`)) {
    if (el instanceof HTMLElement) el.classList.remove(TARGET_CLASS);
  }
  for (const id of [TOUR_ID, ARROWS_ID]) {
    const el = document.getElementById(id);
    if (el) el.remove();
  }
}

function createTourElements() {
  const arrowLayer = document.createElement('div');
  arrowLayer.id = ARROWS_ID;
  arrowLayer.setAttribute('aria-hidden', 'true');
  document.body.appendChild(arrowLayer);

  const bubble = document.createElement('section');
  bubble.id = TOUR_ID;
  bubble.setAttribute('role', 'dialog');
  bubble.setAttribute('aria-live', 'polite');
  bubble.innerHTML = `
    <span class="eventfulranges-tour-pointer" aria-hidden="true"></span>
    <p class="eventfulranges-tour-title"></p>
    <p class="eventfulranges-tour-body"></p>
    <button type="button" class="eventfulranges-tour-step-action" hidden></button>
    <div class="eventfulranges-tour-footer">
      <span class="eventfulranges-tour-progress"></span>
      <div class="eventfulranges-tour-actions">
        <button type="button" data-tour="back">Back</button>
        <button type="button" data-tour="next">Next</button>
        <button type="button" data-tour="skip">Skip</button>
      </div>
    </div>
  `;
  document.body.appendChild(bubble);

  const titleEl = bubble.querySelector('.eventfulranges-tour-title');
  const bodyEl = bubble.querySelector('.eventfulranges-tour-body');
  const stepActionBtn = bubble.querySelector('.eventfulranges-tour-step-action');
  const progressEl = bubble.querySelector('.eventfulranges-tour-progress');
  const backBtn = bubble.querySelector('button[data-tour="back"]');
  const nextBtn = bubble.querySelector('button[data-tour="next"]');
  const skipBtn = bubble.querySelector('button[data-tour="skip"]');
  if (
    !(titleEl instanceof HTMLElement) ||
    !(bodyEl instanceof HTMLElement) ||
    !(stepActionBtn instanceof HTMLButtonElement) ||
    !(progressEl instanceof HTMLElement) ||
    !(backBtn instanceof HTMLButtonElement) ||
    !(nextBtn instanceof HTMLButtonElement) ||
    !(skipBtn instanceof HTMLButtonElement)
  ) {
    arrowLayer.remove();
    bubble.remove();
    return null;
  }
  return { bubble, arrowLayer, titleEl, bodyEl, stepActionBtn, progressEl, backBtn, nextBtn, skipBtn };
}

function clamp(n, lo, hi) {
  return Math.max(lo, Math.min(hi, n));
}

function repositionBubble(bubble, target) {
  const rect = target.getBoundingClientRect();
  const vw = window.innerWidth;
  const vh = window.innerHeight;
  const bubbleRect = bubble.getBoundingClientRect();
  const bubbleWidth = bubbleRect.width > 0 ? bubbleRect.width : 340;
  const bubbleHeight = bubbleRect.height > 0 ? bubbleRect.height : 160;
  const margin = 8;
  const canPlaceBelow = rect.bottom + 12 + bubbleHeight <= vh - margin;
  const top = canPlaceBelow
    ? Math.max(margin, rect.bottom + 12)
    : Math.max(margin, rect.top - bubbleHeight - 12);
  const left = clamp(rect.left, margin, vw - bubbleWidth - margin);
  bubble.style.top = `${String(Math.round(top))}px`;
  bubble.style.left = `${String(Math.round(left))}px`;
  bubble.dataset.side = canPlaceBelow ? 'below' : 'above';
  const pointerLeft = clamp(rect.left + rect.width / 2 - left - 8, 10, bubbleWidth - 22);
  bubble.style.setProperty('--eventfulranges-tour-pointer-left', `${String(Math.round(pointerLeft))}px`);
}

function renderArrows(bubble, arrowLayer, targets) {
  arrowLayer.replaceChildren();
  if (targets.length === 0) return;

  const bubbleRect = bubble.getBoundingClientRect();
  const bubbleCenterX = bubbleRect.left + bubbleRect.width / 2;
  const bubbleCenterY = bubbleRect.top + bubbleRect.height / 2;
  const edgePadding = 12;
  const sideInset = 8;
  const spread = 12;
  const side = bubble.dataset.side;
  const pointerLeftRaw = Number.parseFloat(
    bubble.style.getPropertyValue('--eventfulranges-tour-pointer-left'),
  );
  const pointerCenterX = Number.isFinite(pointerLeftRaw)
    ? bubbleRect.left + pointerLeftRaw + 8
    : bubbleCenterX;
  const pointerTipY =
    side === 'below'
      ? bubbleRect.top - sideInset
      : side === 'above'
        ? bubbleRect.bottom + sideInset
        : bubbleCenterY;

  for (const [index, target] of targets.entries()) {
    const rect = target.getBoundingClientRect();
    const endX = rect.left + rect.width / 2;
    const endY = rect.top + rect.height / 2;
    const toTargetX = endX - bubbleCenterX;
    const toTargetY = endY - bubbleCenterY;
    const horizontalDominant = Math.abs(toTargetX) >= Math.abs(toTargetY);
    const spreadOffset = index - (targets.length - 1) / 2;

    let startX;
    let startY;
    if (targets.length === 1 && (side === 'below' || side === 'above')) {
      startX = pointerCenterX;
      startY = pointerTipY;
    } else if (horizontalDominant) {
      startX = toTargetX >= 0 ? bubbleRect.right + sideInset : bubbleRect.left - sideInset;
      startY = clamp(
        endY + spreadOffset * spread,
        bubbleRect.top + edgePadding,
        bubbleRect.bottom - edgePadding,
      );
    } else {
      startY = toTargetY >= 0 ? bubbleRect.bottom + sideInset : bubbleRect.top - sideInset;
      startX = clamp(
        endX + spreadOffset * spread,
        bubbleRect.left + edgePadding,
        bubbleRect.right - edgePadding,
      );
    }

    const dx = endX - startX;
    const dy = endY - startY;
    const length = Math.hypot(dx, dy);
    if (!Number.isFinite(length) || length < 12) continue;

    const arrow = document.createElement('span');
    arrow.className = 'eventfulranges-tour-arrow';
    arrow.style.left = `${String(Math.round(startX))}px`;
    arrow.style.top = `${String(Math.round(startY))}px`;
    arrow.style.width = `${String(Math.round(length))}px`;
    arrow.style.setProperty('--eventfulranges-tour-arrow-angle', `${String(Math.atan2(dy, dx))}rad`);
    const head = document.createElement('span');
    head.className = 'eventfulranges-tour-arrow-head';
    arrow.appendChild(head);
    arrowLayer.appendChild(arrow);
  }
}

/**
 * Starts the onboarding tour. With force=false it runs only once per browser;
 * the help button reopens it with force=true.
 * @param {Array<{targetSelector?: string, targetSelectors?: string[], title: string, body: string, fallbackActionLabel?: string, fallbackAction?: () => void}>} steps
 * @param {{force?: boolean}} [options]
 */
export function startTour(steps, options = {}) {
  const force = options.force === true;
  if (!force && readFlag()) return;
  ensureStyles();
  clearTourUi();

  const elements = createTourElements();
  if (!elements) return;

  const runtime = { steps, current: 0, highlighted: [], elements };

  function reposition() {
    const targets = visibleTargetsFor(runtime.steps, runtime.current);
    const primary = targets[0];
    if (!primary) {
      elements.bubble.dataset.side = 'none';
      elements.bubble.style.top = '12px';
      elements.bubble.style.left = '12px';
      elements.arrowLayer.replaceChildren();
      return;
    }
    repositionBubble(elements.bubble, primary);
    renderArrows(elements.bubble, elements.arrowLayer, targets);
  }

  function advanceToRenderableStep() {
    while (runtime.current < runtime.steps.length) {
      const step = runtime.steps[runtime.current];
      if (!step) break;
      const visibleTargets = visibleTargetsFor(runtime.steps, runtime.current);
      const hasFallbackAction = typeof step.fallbackAction === 'function';
      if (visibleTargets.length > 0 || hasFallbackAction) break;
      runtime.current++;
    }
  }

  function retreatToRenderableStep() {
    while (runtime.current > 0) {
      const step = runtime.steps[runtime.current];
      if (!step) break;
      const visibleTargets = visibleTargetsFor(runtime.steps, runtime.current);
      const hasFallbackAction = typeof step.fallbackAction === 'function';
      if (visibleTargets.length > 0 || hasFallbackAction) break;
      runtime.current--;
    }
  }

  function syncStepActionUi(step, targets) {
    if (
      targets.length === 0 &&
      typeof step.fallbackAction === 'function' &&
      step.fallbackActionLabel
    ) {
      elements.stepActionBtn.hidden = false;
      elements.stepActionBtn.textContent = step.fallbackActionLabel;
      elements.stepActionBtn.disabled = false;
      return;
    }
    elements.stepActionBtn.hidden = true;
    elements.stepActionBtn.textContent = '';
    elements.stepActionBtn.disabled = true;
  }

  function render() {
    advanceToRenderableStep();
    if (runtime.current >= runtime.steps.length) {
      closeTour();
      return;
    }
    const step = runtime.steps[runtime.current];
    if (!step) return;
    const targets = visibleTargetsFor(runtime.steps, runtime.current);
    for (const el of runtime.highlighted) el.classList.remove(TARGET_CLASS);
    runtime.highlighted = targets;
    for (const el of runtime.highlighted) el.classList.add(TARGET_CLASS);
    elements.titleEl.textContent = step.title;
    elements.bodyEl.textContent = step.body;
    syncStepActionUi(step, targets);
    elements.progressEl.textContent = `${String(runtime.current + 1)} / ${String(runtime.steps.length)}`;
    elements.backBtn.disabled = runtime.current === 0;
    elements.nextBtn.textContent = runtime.current === runtime.steps.length - 1 ? 'Done' : 'Next';
    // Scroll the primary target into view before placing the bubble, so the
    // arrow never points at an off-screen (and therefore wrong) control.
    if (targets[0]) targets[0].scrollIntoView({ block: 'nearest', inline: 'nearest' });
    reposition();
  }

  function closeTour() {
    for (const el of runtime.highlighted) el.classList.remove(TARGET_CLASS);
    runtime.highlighted = [];
    elements.arrowLayer.remove();
    elements.bubble.remove();
    window.removeEventListener('resize', onResize);
    window.removeEventListener('scroll', reposition, true);
    document.removeEventListener('keydown', onKeyDown, true);
    writeFlag();
  }

  function onResize() {
    render();
  }

  function onKeyDown(ev) {
    if (ev.key !== 'Escape') return;
    ev.preventDefault();
    closeTour();
  }

  elements.backBtn.addEventListener('click', () => {
    if (runtime.current > 0) {
      runtime.current--;
      retreatToRenderableStep();
    }
    render();
  });
  elements.stepActionBtn.addEventListener('click', () => {
    const step = runtime.steps[runtime.current];
    if (!step || typeof step.fallbackAction !== 'function') return;
    step.fallbackAction();
    render();
  });
  elements.nextBtn.addEventListener('click', () => {
    if (runtime.current >= runtime.steps.length - 1) {
      closeTour();
      return;
    }
    runtime.current++;
    render();
  });
  elements.skipBtn.addEventListener('click', closeTour);
  window.addEventListener('resize', onResize);
  window.addEventListener('scroll', reposition, true);
  document.addEventListener('keydown', onKeyDown, true);
  render();
}
