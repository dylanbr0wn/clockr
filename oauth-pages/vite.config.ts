import path from "path"
import tailwindcss from "@tailwindcss/vite"
import { defineConfig } from "vite"

export default defineConfig({
  plugins: [tailwindcss()],
  build: {
    outDir: path.resolve(__dirname, "../internal/oauthpages/assets"),
    emptyOutDir: true,
    cssCodeSplit: false,
    rollupOptions: {
      input: path.resolve(__dirname, "src/main.js"),
      output: {
        assetFileNames: "styles[extname]",
        entryFileNames: "_/[name].js",
      },
    },
  },
})
