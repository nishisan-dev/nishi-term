# Capturas da área de trabalho

Capturadas pela suíte Playwright em 24/10/2026, no Chrome/Linux, com bridge Wails **simulado**. Perfis Alpha/Beta e endereços `.invalid` são fixtures. Texto do terminal e estado conectado são simulados: estas imagens não comprovam SSH real nem execução Windows. O aplicativo normal não inclui essas fixtures.

## Área de trabalho — 1280×800

![Área de trabalho](workspace-fixture.png)

## Cadastro de conexão

![Editor](profile-editor-fixture.png)

## Abas — 1280×800

![Abas com saída simulada](tabs-fixture.png)

## Janela compacta — 800×540

![Janela compacta com saída simulada](compact-fixture.png)

As capturas foram inspecionadas visualmente. O teste aguarda o ajuste do xterm antes de capturar os terminais. Reprodução: `CHROME_PATH=/usr/bin/google-chrome npm test --prefix frontend`. Ver [guia e limitações](../../PRODUCT_WORKSPACE.md).

## Autenticação por chave — 800×540

![Formulário de chave após falha simulada, sem segredo preenchido](key-auth-fixture.png)

Captura adicional do fluxo de chave com bridge simulado. A mensagem de erro é produzida pela fixture; não é falha de conexão real.

## Confirmação da identidade e criação do cofre

![Identidade simulada](trust-fixture.png)

![Criação do cofre simulado](vault-fixture.png)

Ambas as imagens usam bridge simulado, sem host real ou credencial preenchida.

## Conexão em andamento

![Feedback de conexão com bridge pendente simulado](connecting-fixture.png)
