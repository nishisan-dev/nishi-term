import React, { useEffect, useRef, useState } from "react";
import { api, type RemoteDirectory } from "./bridge";

export function FilesPanel({
  id,
  onClose,
}: {
  id: string;
  onClose: () => void;
}) {
  const [listing, setListing] = useState<RemoteDirectory | null>(null);
  const [path, setPath] = useState(".");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [filter, setFilter] = useState("");
  const generation = useRef(0),
    pending = useRef(false);
  async function load(target: string) {
    if (pending.current) return;
    pending.current = true;
    const gen = ++generation.current;
    setBusy(true);
    setListing(null);
    setError("");
    try {
      const result = await api().ListFiles(id, target);
      if (generation.current !== gen) return;
      setListing(result);
      setPath(result.path);
      setFilter("");
    } catch (e) {
      if (generation.current === gen)
        setError(
          typeof e === "string" ? e : "Não foi possível carregar os arquivos.",
        );
    } finally {
      pending.current = false;
      if (generation.current === gen) setBusy(false);
    }
  }
  useEffect(() => {
    void load(".");
    return () => {
      ++generation.current;
      void api()
        .CancelFiles(id)
        .catch(() => {});
    };
  }, [id]);
  const rows =
    listing?.entries.filter((e) =>
      e.name.toLocaleLowerCase().includes(filter.toLocaleLowerCase()),
    ) ?? [];
  return (
    <section className="files-panel" aria-label="Arquivos remotos">
      <form
        className="files-toolbar"
        onSubmit={(e) => {
          e.preventDefault();
          void load(path);
        }}
      >
        <strong>
          Arquivos remotos <small>SFTP</small>
        </strong>
        <button
          type="button"
          className="secondary compact"
          disabled={busy || !listing || listing.path === "/"}
          onClick={() => void load(`${listing!.path}/..`)}
          aria-label="Diretório superior"
        >
          ↑
        </button>
        <input
          aria-label="Caminho remoto"
          value={path}
          maxLength={4096}
          onChange={(e) => setPath(e.target.value)}
          disabled={busy}
        />
        <button className="secondary compact" disabled={busy}>
          Ir
        </button>
        <button
          type="button"
          className="secondary compact"
          disabled={busy || !listing}
          onClick={() => void load(listing!.path)}
        >
          Atualizar
        </button>
        <button
          type="button"
          className="secondary compact"
          onClick={onClose}
          aria-label="Ocultar arquivos"
        >
          ×
        </button>
      </form>
      <div className="files-info">
        <input
          aria-label="Filtrar arquivos"
          placeholder="Filtrar nesta pasta…"
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
        />
        <span>
          {busy
            ? "Carregando arquivos…"
            : `${rows.length} itens · somente leitura`}
        </span>
        {busy && (
          <button
            className="secondary compact"
            onClick={() => void api().CancelFiles(id)}
          >
            Cancelar leitura
          </button>
        )}
      </div>
      {error && (
        <p role="alert" className="error">
          {error}
        </p>
      )}
      <div className="files-scroll" aria-busy={busy}>
        <table>
          <thead>
            <tr>
              <th>Nome</th>
              <th>Tamanho</th>
              <th>Modificação</th>
              <th>Permissões</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((entry) => (
              <tr key={entry.name}>
                <td>
                  {entry.directory || entry.symlink ? (
                    <button
                      disabled={busy}
                      className="file-link"
                      onClick={() =>
                        void load(`${listing!.path}/${entry.name}`)
                      }
                    >
                      {entry.directory ? "▸" : "↗"} {entry.name}
                    </button>
                  ) : (
                    <span>{entry.name}</span>
                  )}
                </td>
                <td>
                  {entry.directory
                    ? "—"
                    : new Intl.NumberFormat("pt-BR").format(entry.size) + " B"}
                </td>
                <td>
                  {new Date(entry.modified * 1000).toLocaleString("pt-BR")}
                </td>
                <td>
                  <code>{entry.permissions}</code>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {!busy && !error && rows.length === 0 && (
          <p className="files-empty">Nenhum arquivo encontrado.</p>
        )}
      </div>
    </section>
  );
}
