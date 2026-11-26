import { defineConfig } from "vite"
import react from "@vitejs/plugin-react"
import tailwindcss from "@tailwindcss/vite"

export default defineConfig({
  plugins: [react(), tailwindcss()],
  base: "./",
  build: {
    target: "es2022",
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (/\/node_modules\/(react|react-dom|scheduler|sonner|motion|motion-dom|motion-utils|framer-motion)\//.test(id)) {
            return "ui-runtime"
          }
        },
      },
    },
  },
})
