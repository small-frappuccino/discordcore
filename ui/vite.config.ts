import { defineConfig, loadEnv } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import { compression } from "vite-plugin-compression2";

export default defineConfig(({ mode, command }) => {
  const env = loadEnv(mode, process.cwd(), "");
  const controlTarget =
    env.VITE_CONTROL_API_PROXY_TARGET ?? "http://127.0.0.1:8080";

  return {
    base: command === "build" ? "/manage/" : "/",
    plugins: [
      tailwindcss(),
      react({
        babel: {
          plugins: [["babel-plugin-react-compiler"]],
        },
      }),
      // @ts-expect-error: 'algorithm' type mismatch in plugin types
      compression({ algorithm: "gzip", exclude: [/\.(br)$/, /\.(gz)$/] }),
      // @ts-expect-error: 'algorithm' type mismatch in plugin types
      compression({ algorithm: "brotliCompress", exclude: [/\.(br)$/, /\.(gz)$/] }),
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
    test: {
      environment: "jsdom",
      setupFiles: "./src/test/setup.ts",
      css: true,
    },
  };
});
