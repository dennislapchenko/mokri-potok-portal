import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import { fileURLToPath } from "node:url";

// Served from the site root by the Go binary. src still reads asset URLs off
// import.meta.env.BASE_URL, which is "/" here. In dev, Vite proxies /api to
// the backend on :8788, so the page is same-origin there too and the backend
// needs no CORS anywhere.
//
// @i18n is the backend's dictionary directory: one JSON per language, read by
// the Go binary for notifications and by the page for everything else, so a
// phrase is translated once. Outside the Vite root, hence fs.allow; the
// Docker web stage copies the directory to the same relative place.
const i18n = fileURLToPath(new URL("../backend/internal/httpapi/i18n", import.meta.url));

export default defineConfig({
  plugins: [react()],
  resolve: { alias: { "@i18n": i18n } },
  // 127.0.0.1, not localhost: the dev backend binds loopback v4 only (BIND).
  server: { port: 5173, proxy: { "/api": "http://127.0.0.1:8788" }, fs: { allow: [".", i18n] } },
});
