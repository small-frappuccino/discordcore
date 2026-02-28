import { defineConfig, loadEnv } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig(({ mode, command }) => {
  const env = loadEnv(mode, process.cwd(), "");
  const controlTarget =
    env.VITE_CONTROL_API_PROXY_TARGET ?? "http://127.0.0.1:8080";

  return {
    base: command === "build" ? "/dashboard/" : "/",
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
        "/auth": {
          target: controlTarget,
          changeOrigin: false,
        },
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
