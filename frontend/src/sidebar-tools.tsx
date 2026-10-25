import { useEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { api, type Latencies } from "./bridge";

export function useLatencies() {
  const [samples, setSamples] = useState<Latencies>({});
  useEffect(() => {
    let disposed = false;
    let timer: ReturnType<typeof setTimeout>;
    async function refresh() {
      try {
        const values = await api().HostLatencies();
        if (!disposed)
          setSamples(
            Object.fromEntries(
              Object.entries(values).filter(
                ([, s]) => Date.now() - s.measuredAt < 20000,
              ),
            ),
          );
      } catch {
        if (!disposed) setSamples({});
      }
      if (!disposed) timer = setTimeout(refresh, 1000);
    }
    void refresh();
    return () => {
      disposed = true;
      clearTimeout(timer);
    };
  }, []);
  return samples;
}
export function HostRTT({ sample }: { sample?: Latencies[string] }) {
  if (
    !sample ||
    !Number.isFinite(sample.milliseconds) ||
    sample.milliseconds < 0
  )
    return null;
  return (
    <small
      className="host-rtt"
      title={`RTT ICMP · ${sample.address}. Ausência de resposta não indica indisponibilidade.`}
    >
      {sample.milliseconds < 1 ? "<1" : Math.round(sample.milliseconds)} ms
    </small>
  );
}
export function GroupMenu({
  menu,
  onClose,
  onConnect,
}: {
  menu: { group: string; x: number; y: number; trigger: HTMLElement };
  onClose: () => void;
  onConnect: () => void;
}) {
  const root = useRef<HTMLDivElement>(null);
  useEffect(() => {
    root.current?.querySelector<HTMLButtonElement>("button")?.focus();
    const outside = (e: PointerEvent) => {
      if (!root.current?.contains(e.target as Node)) onClose();
    };
    const dismiss = () => onClose();
    document.addEventListener("pointerdown", outside);
    window.addEventListener("resize", dismiss);
    window.addEventListener("scroll", dismiss, true);
    return () => {
      document.removeEventListener("pointerdown", outside);
      window.removeEventListener("resize", dismiss);
      window.removeEventListener("scroll", dismiss, true);
    };
  }, [menu]);
  return createPortal(
    <div
      ref={root}
      role="menu"
      aria-label={`Ações de ${menu.group || "Sem grupo"}`}
      className="group-menu"
      style={{
        left: Math.max(8, Math.min(menu.x, window.innerWidth - 162)),
        top: Math.max(8, Math.min(menu.y, window.innerHeight - 46)),
      }}
      onKeyDown={(e) => {
        if (e.key === "Escape" || e.key === "Tab") {
          e.preventDefault();
          menu.trigger.focus();
          onClose();
        }
      }}
    >
      <button role="menuitem" onClick={onConnect}>
        Conectar grupo
      </button>
    </div>,
    document.body,
  );
}
