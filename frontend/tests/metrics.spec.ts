import { test, expect, type Page } from "@playwright/test";
import path from "node:path";
async function fixture(page: Page) {
  await page.addInitScript(() => {
    const w = window as any;
    const m = (w.metricsMock = {
      calls: [] as string[],
      cancels: [] as string[],
      fail: false,
      next: 0,
    });
    const seq: Record<string, number> = {};
    w.go = {
      main: {
        App: {
          ListProfiles: async () => [
            {
              id: "a",
              name: "Alpha",
              group: "Laboratório",
              host: "alpha.invalid",
              user: "tester",
              port: 22,
              knownHosts: "",
            },
            {
              id: "b",
              name: "Beta",
              group: "Laboratório",
              host: "beta.invalid",
              user: "tester",
              port: 22,
              knownHosts: "",
            },
          ],
          SessionUsage: async () => m.next,
          VaultStatus: async () => ({ exists: false, locked: true }),
          HostLatencies: async () => ({}),
          Start: async () => {
            const id = "s" + ++m.next;
            seq[id] = 0;
            return id;
          },
          Poll: async (id: string) => {
            await new Promise((r) => setTimeout(r, 40));
            return {
              sequence: ++seq[id],
              data: btoa("fixture terminal\r\n"),
              state: "open",
              error: "",
            };
          },
          Input: async () => {},
          Resize: async () => {},
          Close: async () => {},
          Metrics: async (id: string) => {
            m.calls.push(id);
            if (m.fail) throw "unsupported";
            const first = m.calls.filter((x) => x === id).length === 1;
            return {
              measuredAt: Date.now(),
              cpu: first ? null : 37,
              cores: 8,
              memoryTotal: 16 * 1024 ** 3,
              memoryUsed: 10 * 1024 ** 3,
              diskTotal: 100 * 1024 ** 3,
              diskUsed: 71 * 1024 ** 3,
              uptime: 360000,
              receive: first ? null : 12500000,
              transmit: first ? null : 3200000,
            };
          },
          CancelMetrics: async (id: string) => {
            m.cancels.push(id);
          },
        },
      },
    };
  });
  await page.goto("/");
}
async function connect(page: Page, name = "Alpha") {
  await page.getByRole("button", { name, exact: true }).click();
  await page.getByLabel(`Senha para ${name}`, { exact: true }).fill("fixture");
  await page.getByRole("button", { name: "Conectar", exact: true }).click();
  await expect(
    page.locator('[role="tabpanel"]:visible .connection-state'),
  ).toHaveText("Conectado");
}
test("metrics samples, gaps, pause and compact layout", async ({ page }) => {
  test.setTimeout(30000);
  await fixture(page);
  await connect(page);
  await page.getByRole("button", { name: "Métricas", exact: true }).click();
  const panel = page.getByRole("complementary", { name: "Visão do servidor" });
  await expect(panel).toBeVisible();
  await expect(panel.locator(".cpu > strong")).toHaveText("—");
  await expect(panel.locator(".cpu > strong")).toHaveText("37%", {
    timeout: 8000,
  });
  await expect(
    panel.getByRole("img", {
      name: "Recepção: histórico dos últimos 5 minutos",
    }),
  ).toBeVisible();
  await page.screenshot({
    path: path.resolve("../docs/evidence/product-ui/metrics-fixture.png"),
  });
  await page.setViewportSize({ width: 800, height: 540 });
  await expect(
    panel.getByRole("button", { name: "Recolher métricas" }),
  ).toBeVisible();
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await page.screenshot({
    path: path.resolve(
      "../docs/evidence/product-ui/metrics-compact-fixture.png",
    ),
  });
  await page.evaluate(() => {
    (window as any).metricsMock.fail = true;
  });
  await expect(panel).toContainText("Sem atualização", { timeout: 8000 });
  await expect(panel.locator(".cpu > strong")).toHaveText("—");
  await page.getByRole("button", { name: "Recolher métricas" }).click();
  const calls = await page.evaluate(
    () => (window as any).metricsMock.calls.length,
  );
  await page.waitForTimeout(5200);
  expect(
    await page.evaluate(() => (window as any).metricsMock.calls.length),
  ).toBe(calls);
  expect(
    await page.evaluate(() => (window as any).metricsMock.cancels.length),
  ).toBeGreaterThan(0);
});
test("hidden tab pauses metrics and returning resumes", async ({ page }) => {
  await fixture(page);
  await connect(page);
  await page.getByRole("button", { name: "Métricas", exact: true }).click();
  await expect
    .poll(() => page.evaluate(() => (window as any).metricsMock.calls.length))
    .toBe(1);
  await connect(page, "Beta");
  const calls = await page.evaluate(
    () => (window as any).metricsMock.calls.length,
  );
  await page.waitForTimeout(5200);
  expect(
    await page.evaluate(() => (window as any).metricsMock.calls.length),
  ).toBe(calls);
  await page.getByRole("tab", { name: /Alpha/ }).click();
  await expect
    .poll(() => page.evaluate(() => (window as any).metricsMock.calls.length))
    .toBe(calls + 1);
});
