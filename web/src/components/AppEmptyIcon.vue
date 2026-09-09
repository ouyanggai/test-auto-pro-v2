<template>
  <svg class="app-empty-icon" viewBox="0 0 120 88" aria-hidden="true" focusable="false">
    <circle class="app-empty-icon__halo" cx="60" cy="44" r="38" />
    <path class="app-empty-icon__paper app-empty-icon__paper--back" d="M39 22h42a5 5 0 0 1 5 5v34H34V27a5 5 0 0 1 5-5Z" />
    <path class="app-empty-icon__paper" d="M34 29h52a5 5 0 0 1 5 5v31H29V34a5 5 0 0 1 5-5Z" />
    <path class="app-empty-icon__line" d="M42 42h36M42 50h25" />
    <path class="app-empty-icon__tray" d="m24 58 8 14h56l8-14H73l-4 6H51l-4-6H24Z" />
    <circle class="app-empty-icon__dot" cx="94" cy="27" r="3" />
    <circle class="app-empty-icon__dot app-empty-icon__dot--small" cx="25" cy="35" r="2" />
  </svg>
</template>

<style scoped>
.app-empty-icon {
  display: block;
  width: 100%;
  height: 100%;
  color: var(--primary-color);
}

.app-empty-icon__halo {
  fill: color-mix(in srgb, var(--primary-color) 8%, transparent);
}

.app-empty-icon__paper {
  fill: var(--card-color);
  stroke: color-mix(in srgb, var(--primary-color) 44%, var(--divider-color));
  stroke-width: 1.5;
}

.app-empty-icon__paper--back {
  fill: color-mix(in srgb, var(--primary-color) 7%, var(--card-color));
  opacity: 0.72;
}

.app-empty-icon__line {
  fill: none;
  stroke: color-mix(in srgb, var(--primary-color) 34%, var(--text-color-3));
  stroke-linecap: round;
  stroke-width: 2;
}

.app-empty-icon__tray {
  fill: color-mix(in srgb, var(--primary-color) 12%, var(--card-color));
  stroke: var(--primary-color);
  stroke-linejoin: round;
  stroke-width: 1.8;
}

.app-empty-icon__dot {
  fill: color-mix(in srgb, var(--primary-color) 48%, transparent);
}

.app-empty-icon__dot--small {
  opacity: 0.56;
}
</style>
