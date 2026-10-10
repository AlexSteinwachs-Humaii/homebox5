import { describe, expect, it } from "vitest";
import { getContrastTextColor } from "./utils";

describe("tag text WCAG contrast", () => {
  it.each(["#fff", "#ffffff", "rgb(255, 255, 255)", "#e09020", "#777", "#00aaff"])(
    "chooses black on %s (including colors the old brightness heuristic missed)",
    color => expect(getContrastTextColor(color)).toBe("#000")
  );
  it.each(["#000", "#000000", "rgb(0, 0, 0)", "#0000ff", "#660033"])("chooses white on %s", color =>
    expect(getContrastTextColor(color)).toBe("#fff")
  );
  it("chooses an AA pair across an sRGB sample grid", () => {
    for (let r = 0; r <= 255; r += 17) {
      for (let g = 0; g <= 255; g += 17) {
        for (let b = 0; b <= 255; b += 17) {
          const luminance = [r, g, b].reduce((sum, channel, i) => {
            const v = channel / 255;
            return sum + (v <= 0.04045 ? v / 12.92 : ((v + 0.055) / 1.055) ** 2.4) * [0.2126, 0.7152, 0.0722][i]!;
          }, 0);
          const foreground = getContrastTextColor(`rgb(${r}, ${g}, ${b})`);
          const ratio = foreground === "#000" ? (luminance + 0.05) / 0.05 : 1.05 / (luminance + 0.05);
          expect(ratio).toBeGreaterThanOrEqual(4.5);
        }
      }
    }
  });
});
