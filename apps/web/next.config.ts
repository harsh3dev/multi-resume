import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  async rewrites() {
    const backend = process.env.GO_BACKEND_URL;
    // Builds without backend configuration still start; API calls fail closed.
    const url = new URL(backend ?? "http://127.0.0.1:8080");
    if (!["http:", "https:"].includes(url.protocol) || url.username || url.password || url.pathname !== "/" || url.search || url.hash) {
      throw new Error("GO_BACKEND_URL must be an HTTP(S) origin");
    }
    return [
      { source: "/api/healthz", destination: `${url.origin}/healthz` },
      { source: "/api/:path*", destination: `${url.origin}/api/:path*` },
    ];
  },
};

export default nextConfig;
