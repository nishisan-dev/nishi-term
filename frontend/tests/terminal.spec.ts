import { test, expect, type Page } from "@playwright/test";
import { mkdir } from "node:fs/promises";
import path from "node:path";

async function bridge(page: Page, mode = "stream") {
  await page.addInitScript(
    ({ mode }) => {
      const profiles = [
        {
          id: "a",
          name: "Alpha",
          group: "Laboratório",
          host: "alpha.invalid",
          port: 22,
          user: "tester",
          knownHosts: "/trusted/known_hosts",
        },
        {
          id: "b",
          name: "Beta",
          group: "Laboratório",
          host: "beta.invalid",
          port: 22,
          user: "tester",
          knownHosts: "/trusted/known_hosts",
        },
      ];
      const w = window as any;
      const m = (w.mock = {
        profiles,
        polls: [] as any[],
        closes: [] as string[],
        inputs: [] as any[],
        saves: [] as any[],
        next: 0,
      });
      const sessions: Record<
        string,
        { seq: number; host: string; closed: boolean }
      > = {};
      w.go = {
        main: {
          App: {
            ListProfiles: async () => m.profiles.map((p: any) => ({ ...p })),
            SaveProfile: async (p: any) => {
              if (m.failSave) throw "Não foi possível salvar os perfis";
              m.saves.push({ ...p });
              const saved = { ...p, id: p.id || "new-profile" };
              m.profiles = p.id
                ? m.profiles.map((x: any) => (x.id === p.id ? saved : x))
                : [...m.profiles, saved];
              return m.profiles;
            },
            DeleteProfile: async (id: string) => {
              m.profiles = m.profiles.filter((p: any) => p.id !== id);
              return m.profiles;
            },
            Start: async (c: any) => {
              m.auth = {
                method: c.authMethod,
                path: c.privateKeyPath,
                hasPassword: !!c.password,
                hasPassphrase: !!c.passphrase,
              };
              m.configRef = c;
              if (mode === "fail")
                throw "handshake SSH recusado: verifique identidade do host, credenciais e rede";
              const id = `session-${++m.next}`;
              sessions[id] = { seq: 0, host: c.host, closed: false };
              if (mode === "pending")
                await new Promise<void>((r) => {
                  m.finishStart = r;
                });
              return id;
            },
            Poll: async (id: string, ack: number) => {
              m.polls.push([id, ack]);
              await new Promise((r) => setTimeout(r, 15));
              const s = sessions[id];
              if (s.closed) throw "sessão encerrada";
              if (mode === "fragment") {
                const chunks = [
                  [79, 108, 195],
                  [161, 32, 240, 159],
                  [140, 142],
                ];
                if (ack < chunks.length)
                  return {
                    sequence: ack + 1,
                    data: btoa(String.fromCharCode(...chunks[ack])),
                    state: "open",
                    error: "",
                  };
                return { sequence: ack, data: "", state: "closed", error: "" };
              }
              s.seq++;
              return {
                sequence: s.seq,
                data: btoa(`${s.host} frame ${s.seq}\r\n`),
                state: "open",
                error: "",
              };
            },
            Close: async (id: string) => {
              m.closes.push(id);
              if (sessions[id]) sessions[id].closed = true;
            },
            Resize: async () => {},
            Input: async (id: string, data: string) => {
              m.inputs.push([id, atob(data)]);
            },
          },
        },
      };
    },
    { mode },
  );
}
async function fitted(page: Page) {
  await expect
    .poll(() =>
      page.locator("[role=tabpanel]:visible .xterm-screen").evaluate((el) => {
        const mount = el.closest(".terminal")!;
        const widthGap = mount.clientWidth - el.getBoundingClientRect().width;
        const heightGap =
          mount.clientHeight - el.getBoundingClientRect().height;
        return (
          widthGap >= 0 && widthGap < 60 && heightGap >= 0 && heightGap < 60
        );
      }),
    )
    .toBe(true);
}
async function open(page: Page, name = "Alpha") {
  await page.getByRole("button", { name, exact: true }).click();
  await page.getByLabel(`Senha para ${name}`).fill("canary-not-persisted");
  await page.getByRole("button", { name: "Conectar", exact: true }).click();
}

test("fragmented UTF-8 is processed and ACKed; password is cleared", async ({
  page,
}) => {
  await bridge(page, "fragment");
  await page.goto("/");
  await open(page);
  await expect(page.getByRole("status")).toHaveText("Encerrado");
  await expect(page.getByLabel("Senha para Alpha")).toHaveValue("");
  await expect(page.locator(".xterm-rows")).toContainText("Olá 🌎");
  expect(
    await page.evaluate(() => (window as any).mock.polls.map((c: any) => c[1])),
  ).toEqual([0, 1, 2, 3]);
  expect(await page.evaluate(() => localStorage.length)).toBe(0);
});
test("closing a tab during pending Start closes the late session without polling", async ({
  page,
}) => {
  await bridge(page, "pending");
  await page.goto("/");
  await open(page);
  await page.getByRole("button", { name: "Fechar aba Alpha" }).click();
  await page.getByRole("button", { name: "Encerrar e fechar" }).click();
  await page.evaluate(() => (window as any).mock.finishStart());
  await expect
    .poll(() => page.evaluate(() => (window as any).mock.closes))
    .toEqual(["session-1"]);
  expect(await page.evaluate(() => (window as any).mock.polls)).toEqual([]);
  await expect(page.getByRole("tab")).toHaveCount(0);
});
test("hidden tab pauses consumption, resumes intact, and input stays in its session", async ({
  page,
}) => {
  await bridge(page);
  await page.goto("/");
  await open(page);
  await expect(page.getByRole("status")).toHaveText("Conectado");
  await open(page, "Beta");
  await expect(page.getByRole("status")).toHaveText("Conectado");
  await page.waitForTimeout(100);
  const count = () =>
    page.evaluate(
      () =>
        (window as any).mock.polls.filter((p: any) => p[0] === "session-1")
          .length,
    );
  const paused = await count();
  await page.waitForTimeout(350);
  expect(await count()).toBe(paused);
  await page.locator("[role=tabpanel]:visible .xterm-helper-textarea").focus();
  await page.keyboard.type("beta-input");
  await expect
    .poll(() =>
      page.evaluate(() =>
        (window as any).mock.inputs.map((x: any) => x[1]).join(""),
      ),
    )
    .toBe("beta-input");
  expect(
    await page.evaluate(() =>
      (window as any).mock.inputs.every((x: any) => x[0] === "session-2"),
    ),
  ).toBe(true);
  await page.getByRole("tab", { name: "Alpha", exact: false }).click();
  await expect.poll(count).toBeGreaterThan(paused);
  await expect(
    page.locator("[role=tabpanel]:visible .xterm-rows"),
  ).toContainText("alpha.invalid frame 1");
  await expect(
    page.locator("[role=tabpanel]:visible .xterm-rows"),
  ).not.toContainText("beta.invalid");
  await page.getByRole("button", { name: "Fechar aba Beta" }).click();
  await page.getByRole("button", { name: "Encerrar e fechar" }).click();
  await expect
    .poll(() =>
      page.evaluate(() => (window as any).mock.closes.includes("session-2")),
    )
    .toBe(true);
  await expect(page.getByRole("status")).toHaveText("Conectado");
});
test("profiles edit metadata, Ctrl+K searches accents, deleting preserves open snapshot", async ({
  page,
}) => {
  await bridge(page);
  await page.goto("/");
  await page.getByRole("button", { name: "Alpha", exact: true }).click();
  await page.getByRole("button", { name: "Editar Alpha" }).click();
  await page.getByLabel("Nome", { exact: true }).fill("São Paulo");
  await page.getByLabel("Grupo", { exact: true }).fill("Produção");
  await page.getByRole("button", { name: "Salvar conexão" }).click();
  await expect(
    page.getByRole("button", { name: "São Paulo", exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("tab", { name: "Alpha", exact: true }),
  ).toBeVisible();
  expect(
    await page.evaluate(() =>
      Object.keys((window as any).mock.saves[0]).sort(),
    ),
  ).toEqual(["group", "host", "id", "knownHosts", "name", "port", "user"]);
  await page.keyboard.press("Control+k");
  await page.getByLabel("Buscar por nome, grupo ou endereço").fill("sao paulo");
  await page.keyboard.press("Enter");
  await expect(
    page.getByRole("tab", { name: "São Paulo", exact: true }),
  ).toHaveAttribute("aria-selected", "true");
  await page.getByRole("button", { name: "Excluir São Paulo" }).click();
  await page
    .getByRole("button", { name: "Excluir conexão", exact: true })
    .click();
  await expect(
    page.getByRole("button", { name: "São Paulo", exact: true }),
  ).toHaveCount(0);
  await expect(
    page.getByRole("tab", { name: "São Paulo", exact: true }),
  ).toBeVisible();
});
test("four tab cap and keyboard navigation", async ({ page }) => {
  await bridge(page);
  await page.goto("/");
  for (let i = 0; i < 4; i++)
    await page.getByRole("button", { name: "Alpha", exact: true }).click();
  await expect(page.getByRole("tab")).toHaveCount(4);
  await expect(
    page.getByRole("button", { name: "Abrir conexão", exact: true }),
  ).toBeDisabled();
  await page.getByRole("button", { name: "Beta", exact: true }).click();
  await expect(page.getByRole("alert")).toContainText("Limite de quatro abas");
  await expect(page.getByRole("tab")).toHaveCount(4);
  await page.getByRole("tab").last().focus();
  await page.keyboard.press("Home");
  await expect(page.getByRole("tab").first()).toHaveAttribute(
    "aria-selected",
    "true",
  );
});
test("authentication error clears password without writing storage", async ({
  page,
}) => {
  await bridge(page, "fail");
  await page.goto("/");
  await open(page);
  await expect(page.getByRole("status")).toHaveText("Falhou");
  await expect(page.getByRole("alert")).toContainText("handshake SSH");
  await expect(page.getByLabel("Senha para Alpha")).toHaveValue("");
  expect(await page.evaluate(() => localStorage.length)).toBe(0);
});
test("missing bridge shows actionable error and does not pretend persistence works", async ({
  page,
}) => {
  await page.goto("/");
  await expect(page.getByRole("alert")).toContainText("aplicativo Wails");
  await expect(
    page.getByRole("button", { name: "Nova conexão", exact: true }).first(),
  ).toBeDisabled();
});

test("create profile handles save failure and records visual fixtures", async ({
  page,
}) => {
  await bridge(page);
  await page.setViewportSize({ width: 1280, height: 800 });
  await page.goto("/");
  const output = path.resolve("../docs/evidence/product-ui");
  await mkdir(output, { recursive: true });
  await expect(
    page.getByRole("button", { name: "Alpha", exact: true }),
  ).toBeVisible();
  await page.screenshot({ path: path.join(output, "workspace-fixture.png") });
  await page
    .getByRole("button", { name: "Nova conexão", exact: true })
    .first()
    .click();
  await page.getByLabel("Nome", { exact: true }).fill("Novo laboratório");
  await page.getByLabel("Grupo", { exact: true }).fill("Desenvolvimento");
  await page.getByLabel("Host", { exact: true }).fill("new.invalid");
  await page.getByLabel("Usuário", { exact: true }).fill("tester");
  await page
    .getByLabel("Arquivo known_hosts", { exact: true })
    .fill("/trusted/known_hosts");
  await page.screenshot({
    path: path.join(output, "profile-editor-fixture.png"),
  });
  await page.evaluate(() => ((window as any).mock.failSave = true));
  await page.getByRole("button", { name: "Salvar conexão" }).click();
  await expect(page.getByRole("dialog")).toBeVisible();
  await expect(page.getByRole("alert")).toContainText(
    "Não foi possível salvar",
  );
  await page.evaluate(() => ((window as any).mock.failSave = false));
  await page.getByRole("button", { name: "Salvar conexão" }).click();
  await expect(
    page.getByRole("button", { name: "Novo laboratório", exact: true }),
  ).toBeVisible();
  await open(page);
  await expect(page.getByRole("status")).toHaveText("Conectado");
  await open(page, "Beta");
  await expect(page.getByRole("status")).toHaveText("Conectado");
  await expect(
    page.locator("[role=tabpanel]:visible .xterm-rows"),
  ).toContainText("beta.invalid frame");
  await fitted(page);
  await page.screenshot({ path: path.join(output, "tabs-fixture.png") });
  await page.setViewportSize({ width: 800, height: 540 });
  await expect(
    page.getByRole("button", { name: "Desconectar", exact: true }),
  ).toBeVisible();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await fitted(page);
  await page.screenshot({ path: path.join(output, "compact-fixture.png") });
});
test("oversized paste closes only its session without sending partial commands", async ({
  page,
}) => {
  await bridge(page);
  await page.goto("/");
  await open(page);
  await expect(page.getByRole("status")).toHaveText("Conectado");
  await page.locator(".xterm-helper-textarea").evaluate((el) => {
    const data = new DataTransfer();
    data.setData("text/plain", "x".repeat(40000));
    el.dispatchEvent(
      new ClipboardEvent("paste", {
        clipboardData: data,
        bubbles: true,
        cancelable: true,
      }),
    );
  });
  await expect(page.getByRole("status")).toHaveText("Falhou");
  await expect(page.getByRole("alert")).toContainText("32 KiB");
  expect(await page.evaluate(() => (window as any).mock.inputs.length)).toBe(0);
  await expect
    .poll(() => page.evaluate(() => (window as any).mock.closes))
    .toEqual(["session-1"]);
});

test("key authentication clears passphrase and never saves credentials", async ({
  page,
}) => {
  await bridge(page, "fail");
  await page.goto("/");
  await page.getByRole("button", { name: "Alpha", exact: true }).click();
  await page.getByLabel("Autenticação", { exact: true }).selectOption("key");
  await page.getByLabel("Arquivo da chave privada").fill("/fixture/id_ed25519");
  await page.getByLabel("Passphrase (se houver)").fill("ephemeral-key-canary");
  await page.getByRole("button", { name: "Conectar", exact: true }).click();
  await expect(page.getByRole("alert")).toContainText("handshake SSH recusado");
  await expect(page.getByLabel("Passphrase (se houver)")).toHaveValue("");
  expect(await page.evaluate(() => (window as any).mock.auth)).toEqual({
    method: "key",
    path: "/fixture/id_ed25519",
    hasPassword: false,
    hasPassphrase: true,
  });
  expect(
    await page.evaluate(() => (window as any).mock.configRef.passphrase),
  ).toBe("");
  expect(await page.evaluate(() => (window as any).mock.saves)).toEqual([]);
  expect(await page.evaluate(() => localStorage.length)).toBe(0);
  await page.getByLabel("Passphrase (se houver)").fill("discard-on-switch");
  await page
    .getByLabel("Autenticação", { exact: true })
    .selectOption("password");
  await expect(page.getByLabel("Senha para Alpha")).toHaveValue("");
  await page.getByLabel("Autenticação", { exact: true }).selectOption("key");
  await expect(page.getByLabel("Passphrase (se houver)")).toHaveValue("");
  await page.setViewportSize({ width: 800, height: 540 });
  await expect(
    page.getByRole("button", { name: "Conectar", exact: true }),
  ).toBeInViewport();
  await page.screenshot({
    path: path.resolve("../docs/evidence/product-ui/key-auth-fixture.png"),
  });
});
