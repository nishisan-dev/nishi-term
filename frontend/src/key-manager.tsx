import React, { useEffect, useRef, useState } from "react";
import {
  api,
  type Profile,
  type KeyInfo,
  type Coverage,
  type VaultStatus,
} from "./bridge";
const message = (e: unknown) =>
  typeof e === "string"
    ? e
    : e instanceof Error
      ? e.message
      : "Não foi possível concluir.";

// A gate asks once for the master password; only metadata crosses the bridge.
export function VaultGate({ children }: { children: React.ReactNode }) {
  const [state, setState] = useState<VaultStatus | null>(null),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false);
  const master = useRef<HTMLInputElement>(null);
  useEffect(() => {
    let alive = true;
    api()
      .VaultStatus()
      .then((s) => {
        if (alive) setState(s);
      })
      .catch((e) => {
        if (alive) setError(message(e));
      });
    return () => {
      alive = false;
    };
  }, []);
  async function unlock(e: React.FormEvent) {
    e.preventDefault();
    if (busy) return;
    setBusy(true);
    setError("");
    let secret = master.current?.value ?? "";
    if (master.current) master.current.value = "";
    try {
      const pending = api().UnlockVault(secret);
      secret = "";
      await pending;
      setState(await api().VaultStatus());
    } catch (e) {
      setError(message(e));
    } finally {
      secret = "";
      setBusy(false);
    }
  }
  if (state?.exists && !state.locked) return <>{children}</>;
  return (
    <div>
      {error && (
        <p role="alert" className="error">
          {error}
        </p>
      )}
      {!state ? (
        <p>Consultando cofre…</p>
      ) : !state.exists ? (
        <p>Crie o cofre pelo painel lateral antes de usar este recurso.</p>
      ) : (
        <form onSubmit={unlock}>
          <p>Desbloqueie o cofre para continuar.</p>
          <label>
            Senha mestra
            <input
              ref={master}
              type="password"
              autoComplete="off"
              required
              autoFocus
              disabled={busy}
            />
          </label>
          <div className="modal-actions">
            <button disabled={busy}>
              {busy ? "Desbloqueando…" : "Desbloquear"}
            </button>
          </div>
        </form>
      )}
    </div>
  );
}

export function KeyManager({
  profiles,
  active,
}: {
  profiles: Profile[];
  active?: { sessionID: string; profile: Profile };
}) {
  const [keys, setKeys] = useState<KeyInfo[]>([]),
    [selected, setSelected] = useState(""),
    [label, setLabel] = useState("");
  const [scope, setScope] = useState("host"),
    [target, setTarget] = useState(profiles[0]?.id ?? "");
  const [busy, setBusy] = useState(true),
    [error, setError] = useState(""),
    [feedback, setFeedback] = useState("");
  const [confirmation, setConfirmation] = useState<null | {
    kind: "assign" | "install";
    key: KeyInfo;
    target: string;
    scope: string;
    active?: { sessionID: string; profile: Profile };
  }>(null);
  const groups = [...new Set(profiles.map((p) => p.group))];
  const key = keys.find((k) => k.id === selected);
  useEffect(() => {
    api()
      .ListKeys()
      .then((items) => {
        setKeys([...items]);
        setSelected(items[0]?.id ?? "");
      })
      .catch((e) => setError(message(e)))
      .finally(() => setBusy(false));
  }, []);
  async function generate(e: React.FormEvent) {
    e.preventDefault();
    if (busy) return;
    setBusy(true);
    setError("");
    setFeedback("");
    try {
      const item = await api().GenerateKey(label);
      setKeys((old) => [...old, item]);
      setSelected(item.id);
      setLabel("");
      setFeedback("Chave Ed25519 gerada e guardada no cofre.");
    } catch (e) {
      setError(message(e));
    } finally {
      setBusy(false);
    }
  }
  async function confirm() {
    if (!confirmation || busy) return;
    setBusy(true);
    setError("");
    setFeedback("");
    try {
      const c = confirmation;
      if (c.kind === "install" && c.active) {
        await api().InstallKey(
          c.active.sessionID,
          c.active.profile.id,
          c.key.id,
        );
        setFeedback(
          "Chave pública instalada ou já presente. Teste uma nova conexão antes de encerrar esta sessão; a credencial atual foi preservada.",
        );
      } else {
        await api().AssignKey(
          c.key.id,
          c.scope === "host" ? c.target : "",
          c.scope === "group" ? c.target : "",
          c.scope === "group",
        );
        setFeedback(
          "Associação salva no cofre. A chave pública precisa estar autorizada nos destinos.",
        );
      }
      setConfirmation(null);
    } catch (e) {
      setError(message(e));
    } finally {
      setBusy(false);
    }
  }
  return (
    <div className="key-manager">
      <div className="key-fields" hidden={!!confirmation}>
        <p>
          A chave privada permanece no cofre. Somente a chave pública é enviada
          ao servidor.
        </p>
        <form className="key-generate" onSubmit={generate}>
          <label>
            Nome da nova chave
            <input
              value={label}
              onChange={(e) => setLabel(e.target.value)}
              maxLength={100}
              required
              disabled={busy}
            />
          </label>
          <button disabled={busy || !!confirmation}>Gerar Ed25519</button>
        </form>
        {keys.length > 0 && (
          <>
            <label>
              Chave do cofre
              <select
                value={selected}
                onChange={(e) => setSelected(e.target.value)}
                disabled={busy || !!confirmation}
              >
                {keys.map((k) => (
                  <option key={k.id} value={k.id}>
                    {k.label}
                  </option>
                ))}
              </select>
            </label>
            {key && (
              <>
                <code className="fingerprint">{key.fingerprint}</code>
                <label>
                  Chave pública
                  <textarea readOnly value={key.publicKey} rows={3} />
                </label>
              </>
            )}
            <div className="key-assignment">
              <label>
                Associar a
                <select
                  value={scope}
                  disabled={busy || !!confirmation}
                  onChange={(e) => {
                    setScope(e.target.value);
                    setTarget(
                      e.target.value === "host"
                        ? (profiles[0]?.id ?? "")
                        : (groups[0] ?? ""),
                    );
                  }}
                >
                  <option value="host">Host</option>
                  <option value="group">Grupo</option>
                </select>
              </label>
              <label>
                Destino
                <select
                  value={target}
                  disabled={busy || !!confirmation}
                  onChange={(e) => setTarget(e.target.value)}
                >
                  {scope === "host"
                    ? profiles.map((p) => (
                        <option key={p.id} value={p.id}>
                          {p.name} · {p.user}@{p.host}
                        </option>
                      ))
                    : groups.map((g) => (
                        <option key={g} value={g}>
                          {g || "Sem grupo"}
                        </option>
                      ))}
                </select>
              </label>
            </div>
            <button
              className="secondary"
              disabled={busy || !!confirmation || !key || !profiles.length}
              onClick={() =>
                key && setConfirmation({ kind: "assign", key, target, scope })
              }
            >
              Associar chave
            </button>
            <hr />
            <p>
              {active
                ? `Instalar a chave pública como ${active.profile.user}@${active.profile.host}:${active.profile.port}.`
                : "Conecte um host e selecione sua aba para instalar uma chave pública."}
            </p>
            <button
              className="secondary"
              disabled={busy || !!confirmation || !active || !key}
              onClick={() =>
                key &&
                active &&
                setConfirmation({
                  kind: "install",
                  key,
                  target: "",
                  scope: "",
                  active,
                })
              }
            >
              Instalar chave na sessão atual
            </button>
          </>
        )}
      </div>
      {confirmation && (
        <div className="group-confirm">
          <strong>{confirmation.key.label}</strong>
          {confirmation.kind === "assign" && (
            <p>
              Destino:{" "}
              {confirmation.scope === "host"
                ? (() => {
                    const p = profiles.find(
                      (p) => p.id === confirmation.target,
                    );
                    return p
                      ? `${p.name} · ${p.user}@${p.host}:${p.port}`
                      : "host indisponível";
                  })()
                : confirmation.target || "Sem grupo"}
            </p>
          )}
          <code className="fingerprint">{confirmation.key.fingerprint}</code>
          <p>
            {confirmation.kind === "install"
              ? `Adicionar esta chave pública ao authorized_keys de ${confirmation.active?.profile.user}@${confirmation.active?.profile.host}:${confirmation.active?.profile.port}? Requer shell POSIX; ajusta permissões de .ssh e authorized_keys. Não remove outras chaves.`
              : confirmation.scope === "host"
                ? "Usar esta chave nas próximas conexões ao host selecionado? Substitui a credencial salva para este host, inclusive sua senha."
                : "Definir a chave padrão deste grupo? Vale para membros atuais e futuros sem credencial própria. Credenciais específicas dos hosts têm prioridade."}
          </p>
          <div className="modal-actions">
            <button
              className="secondary"
              disabled={busy}
              onClick={() => setConfirmation(null)}
            >
              Cancelar ação
            </button>
            <button disabled={busy} onClick={() => void confirm()}>
              {busy ? "Executando…" : "Confirmar ação"}
            </button>
          </div>
        </div>
      )}
      {error && (
        <p role="alert" className="error">
          {error}
        </p>
      )}
      {feedback && <p role="status">{feedback}</p>}
    </div>
  );
}

export function GroupConnect({
  group,
  onConnect,
}: {
  group: string;
  onConnect: (profiles: Profile[]) => Promise<void>;
}) {
  const [coverage, setCoverage] = useState<Coverage[] | null>(null),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false);
  useEffect(() => {
    api()
      .GroupCoverage(group)
      .then(setCoverage)
      .catch((e) => setError(message(e)));
  }, [group]);
  const ready =
    coverage && coverage.length > 0 && coverage.every((c) => c.ready);
  async function connect() {
    if (busy) return;
    setBusy(true);
    setError("");
    try {
      const current = await api().GroupCoverage(group);
      setCoverage(current);
      if (!current.length || !current.every((c) => c.ready))
        throw "Há hosts sem credenciais válidas no cofre.";
      await onConnect(current.map((c) => c.profile));
    } catch (e) {
      setError(message(e));
    } finally {
      setBusy(false);
    }
  }
  return (
    <div>
      <p>
        {coverage
          ? `${coverage.filter((c) => c.ready).length}/${coverage.length} hosts com credencial no cofre.`
          : "Consultando credenciais…"}
      </p>
      <p>
        Serão ignorados os hosts já ativos nesta área de trabalho. O grupo
        precisa caber nas vagas disponíveis, até 16 sessões. Hosts desconhecidos
        ainda pedem confirmação de identidade em suas abas.
      </p>
      <ul className="group-coverage">
        {coverage?.map((c) => (
          <li key={c.profile.id}>
            {c.profile.name} —{" "}
            {c.ready ? `pronto (${c.source})` : "sem credencial válida"}
          </li>
        ))}
      </ul>
      {error && (
        <p role="alert" className="error">
          {error}
        </p>
      )}
      <div className="modal-actions">
        <button disabled={!ready || busy} onClick={() => void connect()}>
          {busy ? "Preparando…" : "Conectar hosts do grupo"}
        </button>
      </div>
    </div>
  );
}
