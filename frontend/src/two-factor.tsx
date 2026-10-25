import React, { useEffect, useRef, useState } from "react";
import QRCode from "qrcode";
import { api, type Enrollment } from "./bridge";
const message = (e: unknown) =>
  typeof e === "string"
    ? e
    : e instanceof Error
      ? e.message
      : "Não foi possível alterar o 2FA.";
export function FactorInput({
  inputRef,
  disabled,
}: {
  inputRef: React.RefObject<HTMLInputElement | null>;
  disabled: boolean;
}) {
  return (
    <label>
      Código 2FA ou de recuperação
      <input
        ref={inputRef}
        type="password"
        autoComplete="one-time-code"
        maxLength={64}
        required
        disabled={disabled}
        spellCheck={false}
      />
    </label>
  );
}
export function TwoFactorSettings({
  enabled,
  onChanged,
  onBusy,
}: {
  enabled: boolean;
  onChanged: () => void;
  onBusy: (busy: boolean) => void;
}) {
  const [editing, setEditing] = useState(false),
    [busy, setBusy] = useState(false),
    [error, setError] = useState("");
  const [enrollment, setEnrollment] = useState<Enrollment | null>(null),
    [qr, setQR] = useState(""),
    [codes, setCodes] = useState<string[]>([]);
  const master = useRef<HTMLInputElement>(null),
    code = useRef<HTMLInputElement>(null);
  useEffect(
    () => () => {
      void api()
        .CancelTwoFactorEnrollment()
        .catch(() => {});
    },
    [],
  );
  useEffect(() => {
    let alive = true;
    setQR("");
    if (enrollment)
      void QRCode.toDataURL(enrollment.uri, {
        width: 200,
        margin: 4,
        errorCorrectionLevel: "M",
      })
        .then((data) => {
          if (alive) setQR(data);
        })
        .catch(() => {
          if (alive) setError("QR indisponível; use a chave manual abaixo.");
        });
    return () => {
      alive = false;
    };
  }, [enrollment]);
  function pending(value: boolean) {
    setBusy(value);
    onBusy(value);
  }
  async function submit(e: React.FormEvent) {
    e.preventDefault();
    if (busy) return;
    pending(true);
    setError("");
    let password = master.current?.value ?? "",
      otp = code.current?.value ?? "";
    if (master.current) master.current.value = "";
    if (code.current) code.current.value = "";
    try {
      if (enrollment) {
        const request = api().ConfirmTwoFactor(enrollment.token, otp);
        otp = "";
        const recovery = await request;
        setEnrollment(null);
        setQR("");
        setCodes(recovery);
        setEditing(false);
        onChanged();
      } else if (enabled) {
        const request = api().DisableTwoFactor(password, otp);
        password = "";
        otp = "";
        await request;
        setEditing(false);
        onChanged();
      } else {
        const request = api().BeginTwoFactor(password);
        password = "";
        setEnrollment(await request);
      }
    } catch (e) {
      setError(message(e));
    } finally {
      password = "";
      otp = "";
      pending(false);
    }
  }
  async function cancel() {
    if (busy) return;
    await api().CancelTwoFactorEnrollment();
    setEnrollment(null);
    setQR("");
    setEditing(false);
    setError("");
  }
  if (codes.length)
    return (
      <section className="two-factor">
        <h3>Guarde os códigos de recuperação</h3>
        <p>
          Cada código funciona uma única vez, junto da senha mestra. Eles só são
          exibidos agora. Guarde-os fora deste cofre. Aguarde um novo código do
          autenticador antes de desbloquear novamente.
        </p>
        <pre className="recovery-codes">{codes.join("\n")}</pre>
        <button className="secondary" onClick={() => setCodes([])}>
          Guardei os códigos
        </button>
      </section>
    );
  return (
    <section className="two-factor">
      {!editing && (
        <>
          <h3>Autenticação em duas etapas</h3>
          <p>
            {enabled ? "2FA ativado" : "2FA desativado"} · Google Authenticator
            e aplicativos TOTP compatíveis.
          </p>
          <p className="trust-note">
            O código protege o desbloqueio neste aplicativo. A senha mestra
            continua protegendo o arquivo contra acesso offline.
          </p>
        </>
      )}
      {!editing ? (
        <button
          className="secondary"
          onClick={() => {
            setEditing(true);
            setError("");
          }}
        >
          {enabled ? "Desativar 2FA" : "Configurar 2FA"}
        </button>
      ) : (
        <form onSubmit={submit}>
          {enabled && !enrollment && (
            <p>
              Confirme com um código novo. Se entrou com recuperação, pode
              repetir aquele código nesta sessão por até 5 minutos.
            </p>
          )}
          {enrollment ? (
            <>
              <p>
                Escaneie o QR code no autenticador ou cadastre a chave manual.
                Confirme com um código de seis dígitos para ativar.
              </p>
              {qr && (
                <img
                  className="totp-qr"
                  src={qr}
                  alt="QR code para cadastrar o cofre no autenticador"
                />
              )}
              <label>
                Chave de configuração 2FA
                <input readOnly value={enrollment.secret} />
              </label>
              <FactorInput inputRef={code} disabled={busy} />
            </>
          ) : (
            <>
              <label>
                Confirme a senha mestra
                <input
                  ref={master}
                  type="password"
                  autoComplete="off"
                  maxLength={1024}
                  required
                  disabled={busy}
                />
              </label>
              {enabled && <FactorInput inputRef={code} disabled={busy} />}
            </>
          )}
          {error && (
            <p role="alert" className="error">
              {error}
            </p>
          )}
          <div className="modal-actions">
            <button
              type="button"
              className="secondary"
              disabled={busy}
              onClick={() => void cancel()}
            >
              Cancelar configuração
            </button>
            <button disabled={busy}>
              {busy
                ? "Aguarde…"
                : enrollment
                  ? "Confirmar e ativar 2FA"
                  : enabled
                    ? "Confirmar desativação"
                    : "Gerar QR code"}
            </button>
          </div>
        </form>
      )}
    </section>
  );
}
