import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// BASE_PATH is set by the Pages workflow (/mokri-potok-portal/); local builds
// serve from "/". Everything in src uses relative asset URLs via import.meta.env.BASE_URL.
export default defineConfig({
  plugins: [react()],
  base: process.env.BASE_PATH || "/",
  server: { port: 5173 },
});
