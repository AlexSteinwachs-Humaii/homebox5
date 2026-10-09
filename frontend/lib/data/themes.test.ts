import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import { applyThemeToElement } from "./apply-theme";
import { darkThemes, themes } from "./themes";

const css = readFileSync(new URL("../../assets/css/main.css", import.meta.url), "utf8");
const cinematic = css.match(/\.theme-cinematic\s*\{([^}]+)\}/)![1]!;
const tokens = Object.fromEntries(
  [...cinematic.matchAll(/--([\w-]+):\s*([\d.]+) ([\d.]+)% ([\d.]+)%;/g)].map(([, name, h, s, l]) => [
    name,
    [Number(h), Number(s) / 100, Number(l) / 100],
  ])
);

function luminance(token: string) {
  const [h, s, l] = tokens[token];
  const a = s * Math.min(l, 1 - l);
  const channel = (n: number) => {
    const k = (n + h / 30) % 12;
    const value = l - a * Math.max(-1, Math.min(k - 3, 9 - k, 1));
    return value <= 0.04045 ? value / 12.92 : ((value + 0.055) / 1.055) ** 2.4;
  };
  return 0.2126 * channel(0) + 0.7152 * channel(8) + 0.0722 * channel(4);
}

function contrast(a: string, b: string) {
  const first = luminance(a);
  const second = luminance(b);
  return (Math.max(first, second) + 0.05) / (Math.min(first, second) + 0.05);
}

describe("shared themes", () => {
  it("adds cinematic without removing any existing theme choices", () => {
    expect(themes.map(theme => theme.value)).toEqual([
      "cinematic",
      "homebox",
      "garden",
      "light",
      "cupcake",
      "bumblebee",
      "emerald",
      "corporate",
      "synthwave",
      "retro",
      "cyberpunk",
      "valentine",
      "halloween",
      "forest",
      "aqua",
      "lofi",
      "pastel",
      "fantasy",
      "wireframe",
      "black",
      "luxury",
      "dracula",
      "cmyk",
      "autumn",
      "business",
      "acid",
      "lemonade",
      "night",
      "coffee",
      "winter",
    ]);
    expect(darkThemes).toContain("cinematic");
    for (const theme of themes) {
      expect(css).toContain(theme.value === "homebox" ? ".homebox" : `.theme-${theme.value}`);
    }
  });

  it("replaces only theme classes across saved-theme switches", () => {
    const classes = new Set(["app-ready", "theme-light", "theme-stale", "dark"]);
    const attributes = new Map<string, string>();
    const element = {
      classList: {
        [Symbol.iterator]: () => classes.values(),
        remove: (...names: string[]) => names.forEach(name => classes.delete(name)),
        add: (name: string) => classes.add(name),
      },
      setAttribute: (name: string, value: string) => attributes.set(name, value),
    } as unknown as HTMLElement;
    for (const theme of themes) {
      applyThemeToElement(element, theme.value);
      expect([...classes]).toEqual(["app-ready", `theme-${theme.value}`]);
      expect(attributes.get("data-theme")).toBe(theme.value);
    }
  });

  it("measures cinematic normal text at AA (4.5:1) across shared surfaces", () => {
    for (const surface of ["background", "card", "popover", "muted", "background-accent"]) {
      for (const text of ["foreground", "muted-foreground", "primary"]) {
        expect(contrast(text, surface), `${text} on ${surface}`).toBeGreaterThanOrEqual(4.5);
      }
    }
    for (const role of ["primary", "secondary", "accent", "destructive", "sidebar-primary", "sidebar-accent"]) {
      expect(contrast(`${role}-foreground`, role), role).toBeGreaterThanOrEqual(4.5);
    }
    expect(contrast("sidebar-foreground", "sidebar-background")).toBeGreaterThanOrEqual(4.5);
  });

  it("measures focus and control boundaries at AA (3:1)", () => {
    for (const surface of ["background", "card", "popover", "muted", "accent", "sidebar-background"]) {
      expect(contrast("ring", surface), `focus on ${surface}`).toBeGreaterThanOrEqual(3);
      expect(contrast("input", surface), `input on ${surface}`).toBeGreaterThanOrEqual(3);
    }
  });
});
