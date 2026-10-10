export default async () => {
  const { defineConfig } = await import("vitest/config");
  const path = await import("path");

  return defineConfig({
    // Nuxt's application composables run client-side (SSR is disabled).
    define: { "import.meta.client": true },
    test: {
      globalSetup: "./test/setup.ts",
      include: ["**/*.test.ts"],
    },
    resolve: {
      alias: {
        "@": path.resolve(__dirname, ".."),
        "~": path.resolve(__dirname, ".."),
        "~~": path.resolve(__dirname, ".."),
      },
    },
  });
};
