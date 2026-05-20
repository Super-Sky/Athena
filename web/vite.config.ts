import { defineConfig, loadEnv } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, ".", "");
  const proxyTarget = env.ATHENA_WEB_PROXY_TARGET || "http://127.0.0.1:8090";

  return {
    plugins: [react()],
    server: {
      host: "127.0.0.1",
      port: 5173,
      proxy: {
        "/api": proxyTarget,
        "/swagger": proxyTarget
      }
    },
    build: {
      chunkSizeWarningLimit: 1200,
      rollupOptions: {
        output: {
          manualChunks(id) {
            if (id.indexOf("node_modules/swagger-ui-react") !== -1 || id.indexOf("node_modules/swagger-ui-dist") !== -1) {
              return "swagger";
            }
            if (id.indexOf("node_modules/react") !== -1 || id.indexOf("node_modules/react-dom") !== -1) {
              return "react-vendor";
            }
          }
        }
      }
    }
  };
});
