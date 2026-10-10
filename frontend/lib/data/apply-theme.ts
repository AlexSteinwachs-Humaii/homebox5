import type { DaisyTheme } from "./themes";

/** Apply preferences without touching unrelated root classes or browser storage. */
export function applyThemeToElement(element: HTMLElement, theme: DaisyTheme) {
  element.setAttribute("data-theme", theme);
  const previous = Array.from(element.classList).filter(name => name.startsWith("theme-") || name === "dark");
  element.classList.remove(...previous);
  element.classList.add(`theme-${theme}`);
}
