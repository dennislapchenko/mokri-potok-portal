import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// Served from the site root by the Go binary. src still reads asset URLs off
// import.meta.env.BASE_URL, which is "/" here.
export default defineConfig({
  plugins: [react()],
  server: { port: 5173 },
});
