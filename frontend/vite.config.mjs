import { defineConfig } from 'vite'
import tailwindcss from '@tailwindcss/vite'
import vuePlugin from "@vitejs/plugin-vue";
import wails from "@wailsio/runtime/plugins/vite";
import legacy from "@vitejs/plugin-legacy";

export default defineConfig({
    plugins: [
        tailwindcss(),
        vuePlugin(),
        wails("./bindings"),
        legacy({
          targets: [
            "Chrome 83",
            "Android >= 10",
          ],
        }),
    ],
})
