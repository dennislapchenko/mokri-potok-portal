import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// Served from the site root by the Go binary. src still reads asset URLs off
// import.meta.env.BASE_URL, which is "/" here. In dev, Vite proxies /api to
// the backend on :8788, so the page is same-origin there too and the backend
// needs no CORS anywhere.
export default defineConfig({
  plugins: [react()],
  // 127.0.0.1, not localhost: the dev backend binds loopback v4 only (BIND).
  server: { port: 5173, proxy: { "/api": "http://127.0.0.1:8788" } },
});
