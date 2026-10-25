import React, { useCallback, useEffect, useRef, useState } from "react";
import { createRoot } from "react-dom/client";
import { createPortal } from "react-dom";
import { api, type Profile, type Config, type VaultStatus } from "./bridge";
import { TerminalSession, stateLabels, type SessionStatus } from "./session";
import "@xterm/xterm/css/xterm.css";
import "./style.css";

const initialStatus: SessionStatus = {
  state: "idle",
  error: "",
  cols: 80,
  rows: 24,
};
const blank: Profile = {
  id: "",
  name: "",
  group: "",
  host: "",
  port: 22,
  user: "",
  knownHosts: "",
};
type Tab = { id: string; profile: Profile; status: SessionStatus };
const message = (e: unknown) =>
  typeof e === "string"
    ? e
    : e instanceof Error
      ? e.message
      : "Não foi possível concluir a operação.";
const normalize = (s: string) =>
  s
    .normalize("NFD")
    .replace(/[\u0300-\u036f]/g, "")
    .toLocaleLowerCase("pt-BR");
function Icon({
  name,
}: {
  name: "terminal" | "folder" | "search" | "plus" | "edit" | "close" | "server";
}) {
  const paths = {
    terminal: "m5 7 5 5-5 5 M13 17h6",
    folder: "M3 7V5h6l2 2h10v13H3Z",
    search: "M16 16l5 5 M18 10a8 8 0 1 1-16 0 8 8 0 0 1 16 0",
    plus: "M12 4v16 M4 12h16",
    edit: "m4 15 11-11 5 5-11 11H4Z M13 6l5 5",
    close: "m6 6 12 12 M18 6 6 18",
    server: "M3 4h18v6H3Z M3 14h18v6H3Z M6 7h1 M6 17h1",
  };
  return (
    <svg
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.6"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d={paths[name]} />
    </svg>
  );
}
function Modal({
  title,
  onClose,
  children,
}: {
  title: string;
  onClose: () => void;
  children: React.ReactNode;
}) {
  const dialog = useRef<HTMLDialogElement>(null);
  const titleId = React.useId();
  useEffect(() => {
    dialog.current?.showModal();
    return () => dialog.current?.close();
  }, []);
  return createPortal(
    <dialog
      ref={dialog}
      aria-labelledby={titleId}
      onCancel={(e) => {
        e.preventDefault();
        onClose();
      }}
    >
      <div className="modal-heading">
        <h2 id={titleId}>{title}</h2>
        <button
          className="icon-button"
          aria-label="Fechar diálogo"
          onClick={onClose}
        >
          <Icon name="close" />
        </button>
      </div>
      {children}
    </dialog>,
    document.body,
  );
}
function ProfileEditor({
  profile,
  groups,
  onClose,
  onSaved,
}: {
  profile: Profile;
  groups: string[];
  onClose: () => void;
  onSaved: (p: Profile[]) => void;
}) {
  const [draft, setDraft] = useState({ ...profile }),
    [saving, setSaving] = useState(false),
    [error, setError] = useState("");
  const field = (key: keyof Profile, value: string | number) =>
    setDraft((p) => ({ ...p, [key]: value }));
  async function save(e: React.FormEvent) {
    e.preventDefault();
    if (saving) return;
    setSaving(true);
    setError("");
    try {
      const profiles = await api().SaveProfile(draft);
      onSaved(profiles);
      onClose();
    } catch (e) {
      setError(message(e));
    } finally {
      setSaving(false);
    }
  }
  return (
    <Modal
      title={profile.id ? "Editar conexão" : "Nova conexão"}
      onClose={() => {
        if (!saving) onClose();
      }}
    >
      <form onSubmit={save}>
        <p className="muted">
          Organize o destino. A senha será solicitada ao conectar.
        </p>
        <fieldset disabled={saving} className="form-grid">
          <label>
            Nome
            <input
              autoFocus
              required
              maxLength={100}
              value={draft.name}
              onChange={(e) => field("name", e.target.value)}
              placeholder="Nome da conexão"
            />
          </label>
          <label>
            Grupo
            <input
              maxLength={100}
              list="profile-groups"
              value={draft.group}
              onChange={(e) => field("group", e.target.value)}
              placeholder="Sem grupo"
            />
            <datalist id="profile-groups">
              {groups.map((g) => (
                <option key={g} value={g} />
              ))}
            </datalist>
          </label>
          <label className="host-field">
            Host
            <input
              required
              maxLength={253}
              value={draft.host}
              onChange={(e) => field("host", e.target.value)}
              placeholder="Endereço IP ou domínio"
              autoComplete="off"
            />
          </label>
          <label>
            Porta
            <input
              required
              type="number"
              min={1}
              max={65535}
              value={draft.port}
              onChange={(e) => field("port", Number(e.target.value))}
            />
          </label>
          <label className="wide">
            Usuário
            <input
              required
              maxLength={256}
              value={draft.user}
              onChange={(e) => field("user", e.target.value)}
              autoComplete="off"
            />
          </label>
          <details className="wide" open={draft.knownHosts !== "" || undefined}>
            <summary>Opções avançadas de identidade</summary>
            <label>
              Arquivo known_hosts (opcional)
              <input
                maxLength={4096}
                value={draft.knownHosts}
                onChange={(e) => field("knownHosts", e.target.value)}
                placeholder="Vazio: gerenciado pelo NishiTerm"
              />
            </label>
            <p className="hint">
              Um arquivo externo usa verificação estrita e não permite confirmar
              identidades desconhecidas pela interface.
            </p>
          </details>
        </fieldset>
        <p className="hint">
          Na primeira conexão, confirme a impressão digital do servidor.
          Mudanças posteriores serão bloqueadas.
        </p>
        {error && (
          <p role="alert" className="error">
            {error}
          </p>
        )}
        <div className="modal-actions">
          <button
            type="button"
            className="secondary"
            onClick={onClose}
            disabled={saving}
          >
            Cancelar
          </button>
          <button disabled={saving}>
            {saving ? "Salvando…" : "Salvar conexão"}
          </button>
        </div>
      </form>
    </Modal>
  );
}
function VaultControls() {
  const [status, setStatus] = useState<VaultStatus | null>(null),
    [show, setShow] = useState(false),
    [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  const master = useRef<HTMLInputElement>(null),
    repeat = useRef<HTMLInputElement>(null);
  const refresh = useCallback(async () => {
    try {
      setStatus(await api().VaultStatus());
    } catch {
      setStatus(null);
    }
  }, []);
  useEffect(() => {
    void refresh();
    const timer = setInterval(() => void refresh(), 5000);
    return () => clearInterval(timer);
  }, [refresh]);
  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (!status || busy) return;
    let secret = master.current?.value ?? "";
    let confirmation = repeat.current?.value ?? "";
    if (master.current) master.current.value = "";
    if (repeat.current) repeat.current.value = "";
    if (!status.exists && secret !== confirmation) {
      secret = "";
      confirmation = "";
      setError("As senhas não coincidem. Digite novamente.");
      return;
    }
    confirmation = "";
    setBusy(true);
    setError("");
    try {
      const pending = status.exists
        ? api().UnlockVault(secret)
        : api().CreateVault(secret);
      secret = "";
      await pending;
      await refresh();
      setShow(false);
    } catch (e) {
      setError(message(e));
    } finally {
      secret = "";
      setBusy(false);
    }
  }
  return (
    <>
      <button
        className="secondary"
        onClick={() => {
          setError("");
          setShow(true);
          void refresh();
        }}
      >
        {!status
          ? "Cofre indisponível"
          : !status.exists
            ? "Criar cofre"
            : status.locked
              ? "Desbloquear cofre"
              : "Cofre desbloqueado"}
      </button>
      {show && (
        <Modal
          title={
            !status
              ? "Cofre indisponível"
              : !status.exists
                ? "Criar cofre"
                : status.locked
                  ? "Desbloquear cofre"
                  : "Cofre desbloqueado"
          }
          onClose={() => {
            if (!busy) setShow(false);
          }}
        >
          {!status ? (
            <p>
              Não foi possível acessar o cofre. Confira o diretório do usuário e
              tente novamente.
            </p>
          ) : !status.locked ? (
            <>
              <p>
                Credenciais disponíveis para novas conexões. O cofre bloqueia
                após 5 minutos sem uso de credenciais; sessões SSH abertas
                continuam funcionando.
              </p>
              <button
                onClick={async () => {
                  await api().LockVault();
                  await refresh();
                }}
              >
                Bloquear agora
              </button>
            </>
          ) : (
            <form className="vault-form" onSubmit={submit}>
              <p>
                {status.exists
                  ? "Informe sua senha mestra para liberar as credenciais neste aplicativo."
                  : "Crie uma senha mestra com pelo menos 12 caracteres. Ela é independente da senha do sistema e não exige administrador. Se a esquecer, não será possível recuperar as credenciais do cofre."}
              </p>
              <label>
                Senha mestra
                <input
                  ref={master}
                  type="password"
                  required
                  minLength={status.exists ? 1 : 12}
                  maxLength={1024}
                  autoComplete="off"
                  disabled={busy}
                />
              </label>
              {!status.exists && (
                <label>
                  Confirmar senha mestra
                  <input
                    ref={repeat}
                    type="password"
                    required
                    autoComplete="off"
                    disabled={busy}
                  />
                </label>
              )}
              {error && (
                <p className="error" role="alert">
                  {error}
                </p>
              )}
              <div className="modal-actions">
                <button disabled={busy}>
                  {busy
                    ? "Aguarde…"
                    : status.exists
                      ? "Desbloquear"
                      : "Criar cofre criptografado"}
                </button>
              </div>
            </form>
          )}
        </Modal>
      )}
    </>
  );
}
function SessionPane({
  tab,
  visible,
  onStatus,
}: {
  tab: Tab;
  visible: boolean;
  onStatus: (id: string, status: SessionStatus) => void;
}) {
  const mount = useRef<HTMLDivElement>(null),
    controller = useRef<TerminalSession | null>(null),
    password = useRef<HTMLInputElement>(null),
    passphrase = useRef<HTMLInputElement>(null),
    privateKey = useRef<HTMLInputElement>(null);
  const [authMethod, setAuthMethod] = useState<"password" | "key" | "vault">(
    "password",
  );
  const [status, setStatus] = useState(initialStatus);
  const [savingCredential, setSavingCredential] = useState(false),
    [credentialMessage, setCredentialMessage] = useState("");
  const [confirmingTrust, setConfirmingTrust] = useState(false);
  useEffect(() => {
    setConfirmingTrust(false);
  }, [status.trust?.token]);
  const busy = ["connecting", "open", "draining"].includes(status.state);
  useEffect(() => {
    const c = new TerminalSession(mount.current!, (s) => {
      setStatus(s);
      onStatus(tab.id, s);
    });
    controller.current = c;
    c.setVisible(visible);
    return () => {
      controller.current = null;
      c.dispose();
    };
  }, [tab.id, onStatus]);
  useEffect(() => {
    controller.current?.setVisible(visible);
  }, [visible]);
  function takeConfig(): Config | null {
    const c = controller.current;
    if (!c) return null;
    let secret = password.current?.value ?? "";
    let keySecret = passphrase.current?.value ?? "";
    if (passphrase.current) passphrase.current.value = "";
    if (password.current) password.current.value = "";
    const config: Config = {
      savedCredential: authMethod === "vault" ? tab.profile.id : "",
      host: tab.profile.host,
      port: tab.profile.port,
      user: tab.profile.user,
      knownHosts: tab.profile.knownHosts,
      password: authMethod === "password" ? secret : "",
      authMethod,
      privateKeyPath:
        authMethod === "key" ? (privateKey.current?.value ?? "") : "",
      passphrase: authMethod === "key" ? keySecret : "",
      cols: c.terminal.cols,
      rows: c.terminal.rows,
    };
    secret = "";
    keySecret = "";
    return config;
  }
  function connect(e: React.FormEvent) {
    e.preventDefault();
    setCredentialMessage("");
    const c = takeConfig();
    if (c) void controller.current?.connect(c);
  }
  async function saveCredential() {
    if (savingCredential) return;
    const c = takeConfig();
    if (!c) return;
    setSavingCredential(true);
    setCredentialMessage("");
    try {
      const pending = api().SaveCredential(tab.profile.id, c);
      c.password = "";
      c.passphrase = "";
      await pending;
      setCredentialMessage("Credencial salva no cofre.");
      setAuthMethod("vault");
    } catch (e) {
      setCredentialMessage(message(e));
    } finally {
      c.password = "";
      c.passphrase = "";
      setSavingCredential(false);
    }
  }
  return (
    <section
      className="session-pane"
      role="tabpanel"
      id={`panel-${tab.id}`}
      aria-labelledby={`tab-${tab.id}`}
      hidden={!visible}
    >
      {visible && status.trust && (
        <Modal
          title="Confirmar identidade do servidor"
          onClose={() => void controller.current?.disconnect()}
        >
          <p>
            Este servidor ainda não é conhecido pelo NishiTerm. Confira a
            impressão digital por um canal confiável antes de continuar.
          </p>
          <p>
            <strong>{status.trust.endpoint}</strong>
            <br />
            {status.trust.algorithm}
          </p>
          <code className="fingerprint">{status.trust.fingerprint}</code>
          <p>
            A chave será salva neste computador. Se ela mudar, a conexão será
            bloqueada. Nenhuma credencial de login foi enviada ainda.
          </p>
          <div className="modal-actions">
            <button
              className="secondary"
              onClick={() => void controller.current?.disconnect()}
            >
              Cancelar conexão
            </button>
            <button
              disabled={confirmingTrust}
              onClick={() => {
                setConfirmingTrust(true);
                void controller.current?.confirmHost(status.trust!.token);
              }}
            >
              Confiar e continuar
            </button>
          </div>
        </Modal>
      )}
      <div className="session-toolbar">
        <div className="endpoint">
          <Icon name="server" />
          <strong>
            {tab.profile.user}@{tab.profile.host}
          </strong>
          <span>:{tab.profile.port}</span>
        </div>
        <span className={`connection-state ${status.state}`} role="status">
          <i />
          {stateLabels[status.state]}
        </span>
        {busy && (
          <button
            className="secondary compact"
            onClick={() => void controller.current?.disconnect()}
          >
            {status.state === "connecting" ? "Cancelar conexão" : "Desconectar"}
          </button>
        )}
      </div>
      {!busy && (
        <form className="auth-card" onSubmit={connect}>
          <div>
            <h2>
              {status.state === "idle"
                ? "Conectar ao servidor"
                : "Iniciar uma nova sessão"}
            </h2>
            <p>Credenciais usadas apenas nesta conexão.</p>
          </div>
          <label>
            Autenticação
            <select
              aria-label="Autenticação"
              value={authMethod}
              onChange={(e) => {
                if (password.current) password.current.value = "";
                if (passphrase.current) passphrase.current.value = "";
                setAuthMethod(e.target.value as "password" | "key" | "vault");
              }}
            >
              <option value="password">Senha</option>
              <option value="key">Chave SSH</option>
              <option value="vault">Usar credencial do cofre</option>
            </select>
          </label>
          {authMethod === "vault" ? (
            <p>
              Desbloqueie o cofre na barra lateral para usar a credencial salva
              desta conexão.
            </p>
          ) : authMethod === "key" ? (
            <>
              <label>
                Arquivo da chave privada
                <input
                  ref={privateKey}
                  required
                  maxLength={4096}
                  autoComplete="off"
                  placeholder="Caminho completo no computador"
                />
              </label>
              <label>
                Passphrase (se houver)
                <input ref={passphrase} type="password" autoComplete="off" />
              </label>
            </>
          ) : (
            <label>
              Senha
              <input
                ref={password}
                type="password"
                autoComplete="off"
                aria-label={`Senha para ${tab.profile.name}`}
              />
            </label>
          )}
          <button type="submit" disabled={savingCredential}>
            Conectar <span aria-hidden="true">→</span>
          </button>
          <div className="credential-actions">
            {authMethod !== "vault" ? (
              <button
                type="button"
                className="secondary"
                disabled={savingCredential}
                onClick={() => void saveCredential()}
              >
                Salvar credencial no cofre
              </button>
            ) : (
              <button
                type="button"
                className="secondary"
                onClick={async () => {
                  try {
                    await api().DeleteCredential(tab.profile.id);
                    setCredentialMessage("Credencial removida.");
                    setAuthMethod("password");
                  } catch (e) {
                    setCredentialMessage(message(e));
                  }
                }}
              >
                Remover credencial do cofre
              </button>
            )}
            {credentialMessage && <p role="status">{credentialMessage}</p>}
          </div>
          <p className="trust-note">
            Verificação de identidade:{" "}
            <span title={tab.profile.knownHosts}>
              {tab.profile.knownHosts || "Gerenciada pelo NishiTerm"}
            </span>
          </p>
        </form>
      )}
      {status.error && (
        <div role="alert" className="error session-error">
          {status.error}
        </div>
      )}
      <div className="terminal" ref={mount} />
    </section>
  );
}
function App() {
  const [profiles, setProfiles] = useState<Profile[]>([]),
    [loaded, setLoaded] = useState(false),
    [error, setError] = useState("");
  const [tabs, setTabs] = useState<Tab[]>([]),
    [selected, setSelected] = useState("");
  const [editing, setEditing] = useState<Profile | null>(null),
    [search, setSearch] = useState(false),
    [query, setQuery] = useState("");
  const [confirm, setConfirm] = useState<{
      kind: "tab" | "profile";
      id: string;
      name: string;
    } | null>(null),
    [deleting, setDeleting] = useState(false);
  const [sidebar, setSidebar] = useState(true);
  const active = tabs.find((t) => t.id === selected);
  const groups = [...new Set(profiles.map((p) => p.group))].sort((a, b) =>
    a.localeCompare(b, "pt-BR"),
  );
  const matches = profiles.filter((p) =>
    normalize(`${p.name} ${p.group} ${p.host} ${p.user}`).includes(
      normalize(query),
    ),
  );
  const load = useCallback(async () => {
    setError("");
    try {
      setProfiles(await api().ListProfiles());
      setLoaded(true);
    } catch (e) {
      setError(message(e));
      setLoaded(false);
    }
  }, []);
  useEffect(() => {
    void load();
  }, [load]);
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "k") {
        e.preventDefault();
        e.stopPropagation();
        if (!editing && !confirm) {
          setQuery("");
          setSearch(true);
        }
      }
    };
    document.addEventListener("keydown", handler, true);
    return () => document.removeEventListener("keydown", handler, true);
  }, [editing, confirm]);
  const status = useCallback(
    (id: string, s: SessionStatus) =>
      setTabs((ts) => ts.map((t) => (t.id === id ? { ...t, status: s } : t))),
    [],
  );
  function open(p: Profile) {
    if (tabs.length >= 4) {
      setError(
        "Limite de quatro abas. Feche uma aba para abrir outra conexão.",
      );
      setSearch(false);
      return;
    }
    const id = crypto.randomUUID();
    setTabs((ts) => [...ts, { id, profile: { ...p }, status: initialStatus }]);
    setSelected(id);
    setSearch(false);
    setError("");
  }
  function removeTab(id: string) {
    setTabs((ts) => ts.filter((t) => t.id !== id));
    if (selected === id) {
      const i = tabs.findIndex((t) => t.id === id);
      setSelected(tabs[i - 1]?.id ?? tabs[i + 1]?.id ?? "");
    }
    setConfirm(null);
  }
  function closeTab(t: Tab) {
    if (["connecting", "open", "draining"].includes(t.status.state))
      setConfirm({ kind: "tab", id: t.id, name: t.profile.name });
    else removeTab(t.id);
  }
  async function confirmAction() {
    if (!confirm) return;
    if (confirm.kind === "tab") {
      removeTab(confirm.id);
      return;
    }
    setDeleting(true);
    try {
      setProfiles(await api().DeleteProfile(confirm.id));
      setConfirm(null);
    } catch (e) {
      setError(message(e));
      setConfirm(null);
    } finally {
      setDeleting(false);
    }
  }
  function tabKeys(e: React.KeyboardEvent, index: number) {
    let next = index;
    if (e.key === "ArrowRight") next = (index + 1) % tabs.length;
    else if (e.key === "ArrowLeft")
      next = (index + tabs.length - 1) % tabs.length;
    else if (e.key === "Home") next = 0;
    else if (e.key === "End") next = tabs.length - 1;
    else return;
    e.preventDefault();
    setSelected(tabs[next].id);
    document.getElementById(`tab-${tabs[next].id}`)?.focus();
  }
  return (
    <main className={`app ${sidebar ? "" : "sidebar-collapsed"}`}>
      <header className="topbar">
        <div className="brand">
          <span className="brand-mark">
            <Icon name="terminal" />
          </span>
          <b>NishiTerm</b>
          <span className="edition">SSH WORKSPACE</span>
        </div>
        <button
          className="search-trigger secondary"
          onClick={() => {
            setQuery("");
            setSearch(true);
          }}
        >
          <Icon name="search" />
          <span>Buscar conexões</span>
          <kbd>Ctrl K</kbd>
        </button>
      </header>
      <aside className="sidebar" aria-label="Conexões salvas">
        <div className="sidebar-heading">
          <span>CONEXÕES</span>
          <button
            className="icon-button"
            aria-label="Nova conexão"
            disabled={!loaded}
            onClick={() => setEditing({ ...blank })}
          >
            <Icon name="plus" />
          </button>
        </div>
        <div className="connection-tree">
          {!profiles.length && (
            <p className="sidebar-empty">
              Seus servidores,
              <br />
              organizados aqui.
            </p>
          )}
          {groups.map((group) => (
            <details key={group} open>
              <summary>
                <Icon name="folder" />
                {group || "Sem grupo"}
                <span className="count">
                  {profiles.filter((p) => p.group === group).length}
                </span>
              </summary>
              <div className="group-items">
                {profiles
                  .filter((p) => p.group === group)
                  .sort((a, b) => a.name.localeCompare(b.name, "pt-BR"))
                  .map((p) => (
                    <div
                      className={`profile-row ${active?.profile.id === p.id ? "selected" : ""}`}
                      key={p.id}
                    >
                      <button
                        className="profile-open"
                        onClick={() => open(p)}
                        title={`${p.user}@${p.host}:${p.port}`}
                      >
                        <Icon name="server" />
                        <span>{p.name}</span>
                      </button>
                      <button
                        className="icon-button"
                        aria-label={`Editar ${p.name}`}
                        onClick={() => setEditing({ ...p })}
                      >
                        <Icon name="edit" />
                      </button>
                      <button
                        className="icon-button delete-profile"
                        aria-label={`Excluir ${p.name}`}
                        onClick={() =>
                          setConfirm({
                            kind: "profile",
                            id: p.id,
                            name: p.name,
                          })
                        }
                      >
                        <Icon name="close" />
                      </button>
                    </div>
                  ))}
              </div>
            </details>
          ))}
        </div>
        <div className="sidebar-note">
          <VaultControls />
          <p>Salvar credenciais é opcional. O cofre exige sua senha mestra.</p>
        </div>
      </aside>
      <div className="workspace">
        <div className="tabs-bar">
          <button
            className="icon-button sidebar-toggle"
            aria-label={sidebar ? "Recolher conexões" : "Mostrar conexões"}
            onClick={() => setSidebar(!sidebar)}
          >
            <Icon name="folder" />
          </button>
          <div className="tabs" role="tablist" aria-label="Sessões SSH">
            {tabs.map((t, i) => (
              <div
                key={t.id}
                className={`tab-wrap ${selected === t.id ? "active" : ""}`}
              >
                <button
                  role="tab"
                  id={`tab-${t.id}`}
                  aria-selected={selected === t.id}
                  aria-controls={`panel-${t.id}`}
                  tabIndex={selected === t.id ? 0 : -1}
                  onKeyDown={(e) => tabKeys(e, i)}
                  onClick={() => setSelected(t.id)}
                >
                  <i className={`dot ${t.status.state}`} />
                  <span>{t.profile.name}</span>
                  {selected !== t.id &&
                    ["open", "draining"].includes(t.status.state) && (
                      <span
                        className="paused-mark"
                        title="Leitura pausada; estado atualizado ao selecionar esta aba"
                        aria-label="Saída pausada"
                      >
                        Ⅱ
                      </span>
                    )}
                </button>
                <button
                  className="icon-button close-tab"
                  aria-label={`Fechar aba ${t.profile.name}`}
                  onClick={() => closeTab(t)}
                >
                  <Icon name="close" />
                </button>
              </div>
            ))}
          </div>
          <button
            className="icon-button new-tab"
            aria-label="Abrir conexão"
            disabled={tabs.length >= 4}
            onClick={() => {
              setQuery("");
              setSearch(true);
            }}
          >
            <Icon name="plus" />
          </button>
        </div>
        {error && (
          <div role="alert" className="error app-error">
            <span>{error}</span>
            {!loaded ? (
              <button className="secondary compact" onClick={() => void load()}>
                Tentar novamente
              </button>
            ) : (
              <button
                className="icon-button"
                aria-label="Dispensar aviso"
                onClick={() => setError("")}
              >
                <Icon name="close" />
              </button>
            )}
          </div>
        )}
        <div className="panels">
          {tabs.length ? (
            tabs.map((t) => (
              <SessionPane
                key={t.id}
                tab={t}
                visible={selected === t.id}
                onStatus={status}
              />
            ))
          ) : (
            <section className="welcome">
              <div className="welcome-eyebrow">
                <span /> SEU ESPAÇO DE TRABALHO REMOTO
              </div>
              <div className="welcome-symbol">
                <Icon name="terminal" />
              </div>
              <h1>
                Uma janela.
                <br />
                <span>Suas conexões.</span>
              </h1>
              <p>
                Organize seus servidores em grupos e trabalhe
                <br />
                com até quatro sessões SSH independentes.
              </p>
              <div className="welcome-actions">
                <button
                  disabled={!loaded}
                  onClick={() => setEditing({ ...blank })}
                >
                  <Icon name="plus" />
                  Nova conexão
                </button>
                {profiles.length > 0 && (
                  <button
                    className="secondary"
                    onClick={() => {
                      setQuery("");
                      setSearch(true);
                    }}
                  >
                    Escolher servidor
                  </button>
                )}
              </div>
              <div className="welcome-facts">
                <span>
                  <b>01</b> Cadastre um destino
                </span>
                <span>
                  <b>02</b> Verifique a identidade
                </span>
                <span>
                  <b>03</b> Abra seu terminal
                </span>
              </div>
            </section>
          )}
        </div>
      </div>
      <footer className="statusbar">
        <span>
          <i className={`dot ${active?.status.state ?? "idle"}`} />
          {active ? stateLabels[active.status.state] : "Pronto para conectar"}
        </span>
        <span className="footer-endpoint">
          {active
            ? `${active.profile.host}:${active.profile.port}`
            : "SSH · verificação estrita de identidade"}
        </span>
        <span className="footer-right">
          {tabs.length}/4 abas
          {active && (
            <>
              <span className="divider" /> {active.status.cols} ×{" "}
              {active.status.rows}
            </>
          )}
        </span>
      </footer>
      {editing && (
        <ProfileEditor
          profile={editing}
          groups={groups.filter(Boolean)}
          onClose={() => setEditing(null)}
          onSaved={setProfiles}
        />
      )}
      {search && (
        <Modal title="Buscar conexões" onClose={() => setSearch(false)}>
          <div className="search-box">
            <Icon name="search" />
            <input
              autoFocus
              aria-label="Buscar por nome, grupo ou endereço"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter" && matches[0]) {
                  e.preventDefault();
                  open(matches[0]);
                }
              }}
              placeholder="Nome, grupo, usuário ou endereço"
            />
          </div>
          <div className="search-results">
            {matches.map((p) => (
              <button
                key={p.id}
                className="search-result"
                onClick={() => open(p)}
              >
                <Icon name="server" />
                <span>
                  <strong>{p.name}</strong>
                  <small>
                    {p.group || "Sem grupo"} · {p.user}@{p.host}:{p.port}
                  </small>
                </span>
                <span aria-hidden="true">↗</span>
              </button>
            ))}
            {!matches.length && (
              <p className="muted">
                {profiles.length
                  ? "Nenhuma conexão encontrada."
                  : "Cadastre sua primeira conexão para começar."}
              </p>
            )}
          </div>
          <p className="hint">
            Enter abre o primeiro resultado. A conexão só começa após informar a
            senha.
          </p>
        </Modal>
      )}
      {confirm && (
        <Modal
          title={
            confirm.kind === "tab" ? "Encerrar sessão?" : "Excluir conexão?"
          }
          onClose={() => {
            if (!deleting) setConfirm(null);
          }}
        >
          <p>
            {confirm.kind === "tab"
              ? `Fechar “${confirm.name}” encerra a conexão SSH. Processos remotos podem ser interrompidos.`
              : `Remover “${confirm.name}” dos perfis salvos? As abas já abertas permanecem independentes.`}
          </p>
          <div className="modal-actions">
            <button
              className="secondary"
              disabled={deleting}
              onClick={() => setConfirm(null)}
            >
              Cancelar
            </button>
            <button
              className="danger"
              disabled={deleting}
              onClick={() => void confirmAction()}
            >
              {confirm.kind === "tab" ? "Encerrar e fechar" : "Excluir conexão"}
            </button>
          </div>
        </Modal>
      )}
    </main>
  );
}
createRoot(document.getElementById("root")!).render(<App />);
