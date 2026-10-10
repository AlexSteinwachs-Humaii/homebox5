import { expect, type Locator } from "@playwright/test";

// WCAG sRGB contrast of actual computed text against its nearest opaque surface.
// Deliberately reject transparency/gradients rather than claiming an inaccurate audit.
export async function contrast(locator: Locator) {
  const result = await locator.evaluate(el => {
    const luminance = (color: string) => {
      const c = color
        .match(/[\d.]+/g)!
        .slice(0, 3)
        .map(Number)
        .map(value => {
          const s = value / 255;
          return s <= 0.04045 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
        });
      return c[0]! * 0.2126 + c[1]! * 0.7152 + c[2]! * 0.0722;
    };
    let surface: Element | null = el;
    while (surface) {
      const style = getComputedStyle(surface);
      if (style.backgroundColor.startsWith("rgb(")) {
        const fg = luminance(getComputedStyle(el).color);
        const bg = luminance(style.backgroundColor);
        return {
          foreground: getComputedStyle(el).color,
          background: style.backgroundColor,
          ratio: (Math.max(fg, bg) + 0.05) / (Math.min(fg, bg) + 0.05),
          gradient: style.backgroundImage,
        };
      }
      surface = surface.parentElement;
    }
    throw new Error("No opaque background found");
  });
  expect(result.gradient).toBe("none");
  expect(result.ratio).toBeGreaterThanOrEqual(4.5);
  return result;
}
