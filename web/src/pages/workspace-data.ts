export type ColKind = "pk" | "ck" | "reg";

export interface Col {
  name: string;
  type: string;
  kind: ColKind;
}

export interface Table {
  cols: Col[];
  indexes: string[];
}

export type Schema = Record<string, Record<string, Table>>;

export const schema: Schema = {
  telemetry: {
    sensor_readings: {
      cols: [
        { name: "device_id", type: "uuid", kind: "pk" },
        { name: "bucket", type: "date", kind: "pk" },
        { name: "ts", type: "timestamp", kind: "ck" },
        { name: "temperature_c", type: "float", kind: "reg" },
        { name: "humidity_pct", type: "float", kind: "reg" },
        { name: "battery_mv", type: "int", kind: "reg" },
        { name: "firmware", type: "text", kind: "reg" },
        { name: "tags", type: "set<text>", kind: "reg" },
      ],
      indexes: ["idx_firmware"],
    },
    devices: {
      cols: [
        { name: "device_id", type: "uuid", kind: "pk" },
        { name: "serial", type: "text", kind: "reg" },
        { name: "model", type: "text", kind: "reg" },
        { name: "region", type: "text", kind: "reg" },
        { name: "created_at", type: "timestamp", kind: "reg" },
      ],
      indexes: [],
    },
    alerts_by_device: {
      cols: [
        { name: "device_id", type: "uuid", kind: "pk" },
        { name: "raised_at", type: "timestamp", kind: "ck" },
        { name: "severity", type: "text", kind: "reg" },
        { name: "message", type: "text", kind: "reg" },
        { name: "acked", type: "boolean", kind: "reg" },
      ],
      indexes: [],
    },
    firmware_versions: { cols: [], indexes: [] },
  },
  analytics: {
    events_by_user: { cols: [], indexes: [] },
    sessions: { cols: [], indexes: [] },
    page_views: { cols: [], indexes: [] },
  },
  system: { local: { cols: [], indexes: [] }, peers: { cols: [], indexes: [] } },
  system_schema: {
    tables: { cols: [], indexes: [] },
    columns: { cols: [], indexes: [] },
  },
};

export const sampleQuery = [
  "-- last 24h temperature for a device",
  "SELECT device_id, ts, temperature_c, humidity_pct",
  "FROM telemetry.sensor_readings",
  "WHERE device_id = 7c4a3b2e-9d18-4a8a-b3e1-39b22c14a55b",
  "  AND bucket IN ('2026-05-19','2026-05-20')",
  "  AND ts > toTimestamp(now()) - 86400000",
  "ORDER BY ts DESC",
  "LIMIT 500;",
];

export const sampleRows = Array.from({ length: 22 }).map((_, i) => ({
  device_id: "7c4a3b2e-9d18-4a8a-b3e1-39b22c14a55b",
  bucket: "2026-05-20",
  ts: `2026-05-20 14:${String(58 - i).padStart(2, "0")}:${String((i * 7) % 60).padStart(2, "0")}.${String((i * 137) % 1000).padStart(3, "0")}+0000`,
  temperature_c: (21.4 + Math.sin(i / 3) * 1.8).toFixed(2),
  humidity_pct: (44 + Math.cos(i / 2.5) * 4.2).toFixed(1),
  battery_mv: 3702 - i * 3,
  firmware: i % 5 === 4 ? "1.4.0-rc2" : "1.3.8",
  tags: i % 7 === 0 ? "{outdoor, qa}" : "{outdoor}",
}));

export interface HistoryEntry {
  id: number;
  status: "ok" | "err" | "warn";
  txt: string;
  ts: string;
  dur: string;
  rows: number;
  err?: string;
  warn?: string;
}

export const queryHistory: HistoryEntry[] = [
  {
    id: 1,
    status: "ok",
    txt: "SELECT * FROM telemetry.sensor_readings WHERE device_id = …",
    ts: "14:58:02",
    dur: "142ms",
    rows: 500,
  },
  {
    id: 2,
    status: "ok",
    txt: "SELECT count(*) FROM telemetry.devices",
    ts: "14:42:11",
    dur: "8.4s",
    rows: 1,
  },
  {
    id: 3,
    status: "err",
    txt: "UPDATE telemetry.devices SET region = … WHERE device_id = …",
    ts: "14:31:09",
    dur: "—",
    rows: 0,
    err: "Unauthorized: read-only connection",
  },
  {
    id: 4,
    status: "ok",
    txt: "DESCRIBE TABLE telemetry.alerts_by_device",
    ts: "13:18:44",
    dur: "4ms",
    rows: 1,
  },
  {
    id: 5,
    status: "warn",
    txt: "SELECT * FROM telemetry.sensor_readings ALLOW FILTERING",
    ts: "12:02:55",
    dur: "12.1s",
    rows: 12044,
    warn: "ALLOW FILTERING — full scan",
  },
  {
    id: 6,
    status: "ok",
    txt: "USE telemetry",
    ts: "11:48:30",
    dur: "1ms",
    rows: 0,
  },
];
