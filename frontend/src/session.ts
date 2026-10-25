import { Terminal } from "@xterm/xterm";
import { FitAddon } from "@xterm/addon-fit";
import { api, type Config } from "./bridge";

export type SessionState =
  | "idle"
  | "connecting"
  | "open"
  | "draining"
  | "closed"
  | "failed";
export const stateLabels: Record<SessionState, string> = {
  idle: "Desconectado",
  connecting: "Conectando",
  open: "Conectado",
  draining: "Recebendo saída final",
  closed: "Encerrado",
  failed: "Falhou",
};
export type SessionStatus = {
  state: SessionState;
  error: string;
  cols: number;
  rows: number;
};

// One controller owns exactly one terminal and one connection attempt at a time.
export class TerminalSession {
  readonly terminal: Terminal;
  private fit = new FitAddon();
  private id = "";
  private generation = 0;
  private disposed = false;
  private visible = true;
  private wake?: () => void;
  private finishWrite?: () => void;
  private ready = false;
  private busy = false;
  private status: SessionState = "idle";
  private error = "";
  private queue: Uint8Array[] = [];
  private queuedBytes = 0;
  private sending = false;
  private resizing = false;
  private resizeTimer?: ReturnType<typeof setTimeout>;
  private observer: ResizeObserver;
  private lastReport = "";
  constructor(
    private mount: HTMLElement,
    private report: (s: SessionStatus) => void,
  ) {
    const t = (this.terminal = new Terminal({
      cursorBlink: true,
      scrollback: 2000,
      fontSize: 14,
      fontFamily: "Cascadia Code, Consolas, monospace",
      linkHandler: { activate: () => {} },
      theme: {
        background: "#090f18",
        foreground: "#d8e4f0",
        cursor: "#67d8d0",
        selectionBackground: "#23425c",
      },
    }));
    t.loadAddon(this.fit);
    t.open(mount);
    t.parser.registerOscHandler(52, () => true);
    t.onData((text) => {
      if (text.length > 32768) {
        void this.fail("Entrada acima do limite de 32 KiB. Sessão encerrada.");
        return;
      }
      this.enqueue(new TextEncoder().encode(text));
    });
    t.onBinary((text) => {
      if (text.length > 32768) {
        void this.fail("Entrada acima do limite de 32 KiB. Sessão encerrada.");
        return;
      }
      this.enqueue(Uint8Array.from(text, (c) => c.charCodeAt(0) & 255));
    });
    this.observer = new ResizeObserver(() => this.resize());
    this.observer.observe(mount);
    this.resize();
    this.publish();
  }
  private publish() {
    if (this.disposed) return;
    const s = {
      state: this.status,
      error: this.error,
      cols: this.terminal.cols,
      rows: this.terminal.rows,
    };
    const key = JSON.stringify(s);
    if (key !== this.lastReport) {
      this.lastReport = key;
      this.report(s);
    }
  }
  setVisible(visible: boolean) {
    this.visible = visible;
    if (visible) {
      this.wake?.();
      this.wake = undefined;
      this.resize();
    }
  }
  private async whenVisible(gen: number) {
    while (!this.visible && this.current(gen))
      await new Promise<void>((r) => {
        this.wake = r;
      });
  }
  private current(gen: number) {
    return !this.disposed && gen === this.generation;
  }
  private resize() {
    clearTimeout(this.resizeTimer);
    this.resizeTimer = setTimeout(() => {
      void this.resizeNow();
    }, 100);
  }
  private async resizeNow() {
    if (
      this.disposed ||
      !this.visible ||
      !this.mount.clientWidth ||
      !this.mount.clientHeight
    )
      return;
    this.fit.fit();
    this.publish();
    if (!this.id || !this.ready) return;
    if (this.resizing) {
      this.resize();
      return;
    }
    this.resizing = true;
    try {
      await api().Resize(this.id, this.terminal.cols, this.terminal.rows);
    } catch {
      /* Poll owns transport error reporting. */
    } finally {
      this.resizing = false;
    }
  }
  private enqueue(bytes: Uint8Array) {
    if (!this.ready || !this.id || !this.visible || this.disposed) return;
    if (this.queuedBytes + bytes.length > 32768) {
      void this.fail(
        "Entrada congestionada. Sessão encerrada para evitar comandos parciais.",
      );
      return;
    }
    for (let i = 0; i < bytes.length; i += 4096) {
      const b = bytes.slice(i, i + 4096);
      this.queue.push(b);
      this.queuedBytes += b.length;
    }
    void this.send();
  }
  private async send() {
    if (this.sending) return;
    this.sending = true;
    const id = this.id,
      gen = this.generation;
    try {
      while (this.queue.length && this.current(gen) && id === this.id) {
        const b = this.queue.shift()!;
        this.queuedBytes -= b.length;
        await api().Input(id, btoa(String.fromCharCode(...b)));
      }
    } catch {
      if (this.current(gen))
        await this.fail(
          "Entrada congestionada ou conexão perdida. Sessão encerrada.",
        );
    } finally {
      this.sending = false;
      if (this.queue.length && this.ready && !this.disposed) void this.send();
    }
  }
  async connect(c: Config) {
    if (this.busy || this.disposed) {
      c.password = "";
      c.passphrase = "";
      return;
    }
    this.busy = true;
    const gen = ++this.generation;
    this.status = "connecting";
    this.error = "";
    this.terminal.reset();
    this.publish();
    try {
      let pending: Promise<string>;
      try {
        pending = api().Start(c);
      } finally {
        c.password = "";
        c.passphrase = "";
      }
      const id = await pending;
      if (!this.current(gen)) {
        await api().Close(id);
        return;
      }
      this.id = id;
      let ack = 0,
        opened = false;
      while (this.current(gen)) {
        await this.whenVisible(gen);
        if (!this.current(gen)) return;
        const frame = await api().Poll(id, ack);
        if (!this.current(gen)) return;
        this.status = frame.state as SessionState;
        this.ready = frame.state === "open";
        if (frame.data || !["closed", "failed"].includes(frame.state))
          this.publish();
        if (this.ready && !opened) {
          opened = true;
          this.resize();
          if (this.visible) this.terminal.focus();
        }
        if (frame.data) {
          // Keep at most one delivered frame while hidden. Never ACK before xterm's callback.
          await this.whenVisible(gen);
          if (!this.current(gen)) return;
          const bytes = Uint8Array.from(atob(frame.data), (c) =>
            c.charCodeAt(0),
          );
          await new Promise<void>((resolve) => {
            this.finishWrite = resolve;
            this.terminal.write(bytes, () => {
              this.finishWrite = undefined;
              resolve();
            });
          });
          if (!this.current(gen)) return;
          ack = frame.sequence;
        } else if (frame.state === "closed" || frame.state === "failed") {
          this.ready = false;
          this.error = frame.error;
          this.id = "";
          this.queue = [];
          this.queuedBytes = 0;
          await api().Close(id);
          if (!this.current(gen)) return;
          this.busy = false;
          this.publish();
          return;
        }
      }
    } catch (e) {
      if (this.current(gen)) {
        await this.fail(
          typeof e === "string"
            ? e
            : e instanceof Error
              ? e.message
              : "Falha na conexão SSH.",
        );
      }
    }
  }
  async disconnect() {
    const gen = ++this.generation;
    this.ready = false;
    this.busy = false;
    const id = this.id;
    this.id = "";
    this.queue = [];
    this.queuedBytes = 0;
    this.wake?.();
    this.wake = undefined;
    this.finishWrite?.();
    this.finishWrite = undefined;
    this.status = "idle";
    this.publish();
    if (id)
      try {
        await api().Close(id);
      } catch {
        if (this.current(gen)) {
          this.error = "Não foi possível confirmar o fechamento.";
          this.publish();
        }
      }
  }
  private async fail(message: string) {
    const closing = this.disconnect();
    const gen = this.generation;
    await closing;
    if (!this.current(gen)) return;
    this.status = "failed";
    this.error = message;
    this.publish();
  }
  dispose() {
    if (this.disposed) return;
    this.disposed = true;
    void this.disconnect();
    clearTimeout(this.resizeTimer);
    this.observer.disconnect();
    this.terminal.dispose();
  }
}
