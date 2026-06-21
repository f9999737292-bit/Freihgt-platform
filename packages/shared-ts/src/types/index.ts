export type AppHealthResponse = {
  status: "ok" | "degraded" | "down";
  timestamp: string;
  version?: string;
};
