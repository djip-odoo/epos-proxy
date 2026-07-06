import { defineConfig } from 'vite'
import tailwindcss from '@tailwindcss/vite'
import vuePlugin from "@vitejs/plugin-vue";
import wails from "@wailsio/runtime/plugins/vite";

export default defineConfig({
    plugins: [
        tailwindcss(),
        vuePlugin(),
        wails("./bindings"),
    ],
})
