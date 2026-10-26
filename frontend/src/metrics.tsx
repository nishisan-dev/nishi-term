import { useEffect, useRef, useState } from "react";
import { api, type MetricsSample } from "./bridge";
import "./metrics.css";

type Point = { time: number; sample: MetricsSample | null };
const windowMs = 5 * 60 * 1000;
const percent = (used: number, total: number) =>
  total > 0 ? (100 * used) / total : null;
const value = (n: number | null | undefined, suffix = "%") =>
  n == null ? "—" : `${n.toFixed(0)}${suffix}`;
function bytes(n: number | null | undefined) {
  if (n == null) return "—";
  const units = ["B", "KiB", "MiB", "GiB", "TiB"];
  let i = 0;
  while (n >= 1024 && i < units.length - 1) {
    n /= 1024;
    i++;
  }
  return `${n.toFixed(i ? 1 : 0)} ${units[i]}`;
}
function uptime(n: number) {
  const days = Math.floor(n / 86400),
    hours = Math.floor(n / 3600) % 24,
    minutes = Math.floor(n / 60) % 60;
  return `${days}d ${hours}h ${minutes}min`;
}
function Spark({
  points,
  pick,
  label,
  ceiling,
  format = (n: number) => value(n),
}: {
  points: Point[];
  pick: (s: MetricsSample) => number | null;
  label: string;
  ceiling?: number;
  format?: (n: number) => string;
}) {
  const end = points.at(-1)?.time ?? Date.now();
  const values = points.map((p) => (p.sample ? pick(p.sample) : null));
  const max =
    ceiling ?? Math.max(1, ...values.filter((n): n is number => n != null));
  let path = "",
    previous = 0,
    connected = false;
  const dots: { x: number; y: number; n: number; time: number }[] = [];
  points.forEach((p, i) => {
    const n = values[i];
    if (n == null) {
      connected = false;
      return;
    }
    const x = 3 + 234 * Math.max(0, 1 - (end - p.time) / windowMs),
      y = 47 - 42 * Math.min(1, Math.max(0, n / max));
    path += `${connected && p.time - previous < 8000 ? "L" : "M"}${x.toFixed(2)},${y.toFixed(2)} `;
    dots.push({ x, y, n, time: p.time });
    previous = p.time;
    connected = true;
  });
  return (
    <svg
      className="metric-spark"
      viewBox="0 0 240 52"
      role="img"
      aria-label={`${label}: histórico dos últimos 5 minutos`}
    >
      <path className="metric-grid" d="M3 5H237 M3 26H237 M3 47H237" />
      <path d={path} fill="none" stroke="currentColor" strokeWidth="1.7" />
      {dots.map((d, i) => (
        <circle key={i} cx={d.x} cy={d.y} r="2" fill="currentColor">
          <title>
            {new Date(d.time).toLocaleTimeString()} · {format(d.n)}
          </title>
        </circle>
      ))}
    </svg>
  );
}
export function MetricsPanel({
  id,
  active,
  onClose,
}: {
  id: string;
  active: boolean;
  onClose: () => void;
}) {
  const [history, setHistory] = useState<Point[]>([]);
  const [sample, setSample] = useState<MetricsSample | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const pending = useRef(false);
  const nextCollection = useRef(0);
  const [foreground, setForeground] = useState(!document.hidden);
  useEffect(() => {
    const changed = () => setForeground(!document.hidden);
    document.addEventListener("visibilitychange", changed);
    return () => document.removeEventListener("visibilitychange", changed);
  }, []);
  useEffect(() => {
    if (!active || !foreground) return;
    let disposed = false;
    let timer: ReturnType<typeof setTimeout>;
    const add = (point: Point) =>
      setHistory((previous) =>
        [
          ...previous.filter((p) => p.time > point.time - windowMs),
          point,
        ].slice(-61),
      );
    async function collect() {
      if (disposed) return;
      if (pending.current) {
        timer = setTimeout(collect, 250);
        return;
      }
      if (Date.now() < nextCollection.current) {
        timer = setTimeout(collect, nextCollection.current - Date.now());
        return;
      }
      nextCollection.current = Date.now() + 5000;
      pending.current = true;
      setBusy(true);
      const started = Date.now();
      try {
        const result = await api().Metrics(id);
        if (!disposed) {
          setSample(result);
          setError("");
          add({ time: result.measuredAt, sample: result });
        }
      } catch {
        if (!disposed) {
          setError(
            "Sem atualização. Verifique o suporte Linux e as permissões de leitura.",
          );
          setSample(null);
          add({ time: Date.now(), sample: null });
        }
      } finally {
        pending.current = false;
        if (!disposed) {
          setBusy(false);
          timer = setTimeout(
            collect,
            Math.max(0, 5000 - (Date.now() - started)),
          );
        }
      }
    }
    setSample(null);
    void collect();
    return () => {
      disposed = true;
      clearTimeout(timer);
      void api()
        .CancelMetrics(id)
        .catch(() => {});
    };
  }, [id, active, foreground]);
  const memory = (s: MetricsSample) => percent(s.memoryUsed, s.memoryTotal);
  const disk = (s: MetricsSample) => percent(s.diskUsed, s.diskTotal);
  return (
    <aside className="metrics-panel" aria-label="Visão do servidor">
      <header>
        <div>
          <h2>Visão do servidor</h2>
          <small>Histórico · 5 min</small>
        </div>
        <button
          className="icon-button"
          aria-label="Recolher métricas"
          onClick={onClose}
        >
          ×
        </button>
      </header>
      <p className="metrics-update" role="status">
        {error ||
          (sample
            ? `Atualizado ${new Date(sample.measuredAt).toLocaleTimeString()}`
            : busy
              ? "Coletando métricas…"
              : "Coleta pausada")}
      </p>
      <div className="metric-summary">
        <span>Uptime</span>
        <strong>{sample ? uptime(sample.uptime) : "—"}</strong>
      </div>
      <section className="metric-card cpu">
        <h3>
          CPU <small>{sample ? `${sample.cores} vCPU` : ""}</small>
        </h3>
        <strong>{value(sample?.cpu)}</strong>
        <Spark points={history} pick={(s) => s.cpu} label="CPU" ceiling={100} />
      </section>
      <section className="metric-card memory">
        <h3>Memória</h3>
        <strong>{sample ? value(memory(sample)) : "—"}</strong>
        <small>
          {sample
            ? `${bytes(sample.memoryUsed)} / ${bytes(sample.memoryTotal)}`
            : "—"}
        </small>
        <Spark points={history} pick={memory} label="Memória" ceiling={100} />
      </section>
      <section className="metric-card disk">
        <h3>Disco /</h3>
        <strong>{sample ? value(disk(sample)) : "—"}</strong>
        <small>
          {sample
            ? `${bytes(sample.diskUsed)} / ${bytes(sample.diskTotal)}`
            : "—"}
        </small>
        <Spark points={history} pick={disk} label="Disco" ceiling={100} />
      </section>
      <section className="metric-card network">
        <h3>Rede</h3>
        <div className="network-rates">
          <span>↓ {bytes(sample?.receive)}/s</span>
          <span>↑ {bytes(sample?.transmit)}/s</span>
        </div>
        <Spark
          points={history}
          pick={(s) => s.receive}
          label="Recepção"
          format={(n) => bytes(n) + "/s"}
        />
        <Spark
          points={history}
          pick={(s) => s.transmit}
          label="Transmissão"
          format={(n) => bytes(n) + "/s"}
        />
        <small>Soma das interfaces, exceto loopback.</small>
      </section>
      <p className="metrics-footnote">
        A cada 5 s enquanto visível. CPU e rede aguardam duas amostras.
        Interfaces virtuais podem contar tráfego mais de uma vez.
      </p>
    </aside>
  );
}
