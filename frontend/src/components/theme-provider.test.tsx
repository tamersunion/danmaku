import { afterEach, describe, expect, it, vi } from "vitest";
import { renderToStaticMarkup } from "react-dom/server";
import { ThemeProvider, useTheme } from "./theme-provider";

function Probe() {
  const { theme, resolvedTheme } = useTheme();
  return <output>{theme}:{resolvedTheme}</output>;
}

afterEach(() => vi.unstubAllGlobals());

describe("theme preference", () => {
  it.each([
    [null, true, "system:dark"],
    [null, false, "system:light"],
    ["invalid", true, "system:dark"],
    ["system", false, "system:light"],
    ["light", true, "light:light"],
    ["dark", false, "dark:dark"],
  ])("resolves saved %s with system dark=%s", (saved, dark, expected) => {
    vi.stubGlobal("localStorage", { getItem: () => saved });
    vi.stubGlobal("window", { matchMedia: () => ({ matches: dark }) });
    expect(renderToStaticMarkup(<ThemeProvider><Probe /></ThemeProvider>)).toContain(expected);
  });
});
