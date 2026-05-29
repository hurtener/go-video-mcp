/**
 * theme.ts — Frameline Studio theming.
 *
 * The bridge propagates the host's theme via `hostContext.styles.variables`
 * (the `--dy-*` CSS custom properties). We apply them to the App root; the
 * cinematic palette in App.svelte's CSS is the *fallback* (`var(--dy-…, #hex)`)
 * so Frameline looks right whether or not the host supplies variables.
 */

import type { StyleVariables } from 'dockyard-bridge';

/** Applies the host's style variables to the App root as CSS custom properties. */
export function applyHostVariables(
  root: HTMLElement,
  vars: StyleVariables | undefined,
): void {
  if (!vars) return;
  for (const [name, value] of Object.entries(vars)) {
    if (typeof value === 'string') root.style.setProperty(name, value);
  }
}

/** Reads the host's preferred theme hint, defaulting to dark (Frameline's home). */
export function hostThemeHint(vars: StyleVariables | undefined): 'light' | 'dark' {
  const hint = vars ? (vars as Record<string, string>)['--dy-host-theme'] : undefined;
  if (hint === 'light' || hint === 'dark') return hint;
  if (
    typeof window !== 'undefined' &&
    typeof window.matchMedia === 'function' &&
    window.matchMedia('(prefers-color-scheme: light)').matches
  ) {
    return 'light';
  }
  return 'dark';
}
