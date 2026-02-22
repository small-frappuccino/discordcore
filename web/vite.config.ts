import { defineConfig, loadEnv } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), "");
  const controlTarget =
    env.VITE_CONTROL_API_PROXY_TARGET ?? "http://127.0.0.1:8080";

  return {
    plugins: [
      react({
        babel: {
          plugins: [["babel-plugin-react-compiler"]],
        },
      }),
    ],
    server: {
      host: true,
      proxy: {
        "/v1": {
          target: controlTarget,
          changeOrigin: false,
        },
      },
    },
    build: {
      manifest: true,
    },
  };
});
