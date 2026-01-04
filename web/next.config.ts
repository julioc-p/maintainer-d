import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  distDir: process.env.NEXT_DIST_DIR || ".next",
  turbopack: {
    root: process.env.TURBOPACK_ROOT || "./",
  },
  outputFileTracingRoot: process.env.OUTPUT_FILE_TRACING_ROOT,
};

export default nextConfig;
