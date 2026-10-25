# NishiTerm

Cliente de terminal SSH desktop em desenvolvimento em Go + Wails v2 estável + React + xterm.js. O projeto está na fase de especificação e validação técnica; não é um cliente SSH pronto para distribuição.

## Documentação

- [Especificação e lacunas da referência](docs/SPEC.md)
- [Arquitetura e protocolo de fluxo](docs/ARCHITECTURE.md)
- [Mockup e direção visual](docs/UI_REFERENCE.md)
- [Roadmap por fases](docs/ROADMAP.md)
- [Riscos e testes de aceitação](docs/RISKS_AND_VALIDATION.md)
- [Validação e limitações](docs/TESTING.md)
- [Instruções permanentes](AGENTS.md)

O mockup foi recebido e está documentado em [UI_REFERENCE.md](docs/UI_REFERENCE.md). A conversa anexada ainda não disponibilizou seu conteúdo completo. A interface atual inclui perfis agrupados, busca Ctrl+K e abas independentes com até 16 sessões ativas ou conectando. O cofre inicial está disponível; SFTP e métricas são futuros. Veja o [guia da área de trabalho](docs/PRODUCT_WORKSPACE.md).

## Pré-requisitos

Go 1.27.3, Node.js 24 com npm; versões de aplicação fixadas em go.mod e frontend/package-lock.json. O build Windows pode ser feito no Linux sem GTK/WebKit e sem instalar o CLI Wails globalmente. A primeira execução baixa dependências e precisa de rede.

Neste ambiente, o wrapper `/snap/bin/go` falhou por permissões do Snap, mas `/snap/go/current/bin/go` funciona. Se necessário, ajuste somente o terminal da tarefa:

```bash
export PATH="/snap/go/current/bin:$PATH"
```

## Compilar Windows no Linux

```bash
./scripts/build-windows.sh
```

Saída: `build/bin/NishiTerm.exe` (Windows amd64). O script obtém a versão exata do Wails no módulo, instala frontend pelo lockfile, gera bindings e compila. Manifesto `asInvoker`; sem instalador NSIS. A estratégia `-webview2 error` não instala runtime nem solicita elevação. O Windows de destino precisa de WebView2 compatível. O cache WebView fica na pasta do usuário. A PoC anterior foi validada no Windows; a nova interface ainda precisa de ensaio nativo próprio.

## Usar o NishiTerm

1. Copie o executável para uma pasta gravável pelo usuário em Windows com WebView2.
2. Na primeira conexão, confira a impressão digital do servidor por um canal confiável e confirme. O NishiTerm salva a identidade e bloqueia mudanças. Arquivo known_hosts externo continua disponível nas opções avançadas.
3. Cadastre uma conexão com nome, grupo, host, porta, usuário (known_hosts externo é opcional). Abra uma aba pela árvore ou busca Ctrl+K, informe a senha e conecte a um servidor de teste que permita senha e PTY.
4. Teste teclado, comandos, resize e desconexão. A senha sai do formulário ao conectar e não é salva. Host desconhecido/trocado/revogado é recusado.

Perfis persistem somente metadados. Até 16 sessões podem estar ativas ou conectando; abas desconectadas não ocupam vagas; abas ocultas pausam leitura e retomam ao selecionar. A conexão também aceita arquivo de chave privada com passphrase opcional. O cofre opcional exige senha mestra e salva somente credenciais criptografadas. Agent e reconexão automática ainda não estão disponíveis. Colagens acima de 32 KiB ou congestionamento de entrada encerram a sessão com aviso; comandos já enviados não podem ser desfeitos.

## Desenvolvimento e testes

```bash
go test -race ./internal/...
go vet ./internal/...
cd frontend
npm ci
npm run build
npx playwright install chromium
npm test
```

Se Chrome já estiver instalado, é possível evitar baixar outro browser com `CHROME_PATH=/usr/bin/google-chrome npm test` (Linux). Os testes React usam bridge simulado; os testes Go fazem handshake SSH real em loopback com chaves efêmeras. Nenhum teste precisa de credenciais reais.

Para validar o adaptador Windows sem executar o EXE, após gerar frontend:

```bash
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go vet ./...
```

Para desenvolvimento desktop Linux, instale as dependências GTK3/WebKitGTK da distribuição e use a mesma versão do CLI: `go run github.com/wailsapp/wails/v2/cmd/wails@v2.17.0 dev -tags webkit2_41` em sistemas com WebKitGTK 4.1. `npm run dev` isoladamente mostra a interface, mas não disponibiliza SSH.

## Estrutura

`internal/sshengine` contém conexão, identidade do host, PTY, filas e sessões; `app.go` é o adaptador Wails; `frontend/src` contém UI e terminal. O motor não importa Wails. Artefatos, dependências instaladas e bindings gerados não devem ser versionados. Os ícones de build são placeholders do Wails, não identidade visual aprovada.

## Licença e autoria

Copyright 2026 Lucas Nishimura — [lucas@nishisan.dev](mailto:lucas@nishisan.dev). Código original sob [Apache-2.0](LICENSE), com atribuição em [NOTICE](NOTICE). A licença permite uso comercial e derivados fechados, exigindo preservar os avisos aplicáveis conforme seus termos; não obriga exibir crédito na tela do aplicativo. Dependências e recursos de terceiros mantêm suas próprias licenças.

Veja [CONTRIBUTING.md](CONTRIBUTING.md) para contribuir. Relatórios e roteiros específicos da infraestrutura de laboratório original ficam locais; a base pública inclui testes reproduzíveis sem credenciais reais.

## CI, Linux e chaves SSH

[Testes e builds no GitHub Actions](https://github.com/nishisan-dev/nishi-term/actions/workflows/ci.yml) geram artifacts Windows e Linux após aprovação das suítes. Veja [CI e execução Linux](docs/CI.md). No formulário da aba, selecione **Chave SSH**, informe o caminho local e a passphrase se necessária. O conteúdo da chave fica no backend; não é salvo nos perfis. Veja [formatos, limites e requisito do cofre autenticado](docs/CREDENTIALS.md).

Veja [identidades gerenciadas](docs/HOST_TRUST.md) e [cofre com senha mestra](docs/CREDENTIALS.md). O cofre é uma implementação inicial com testes automatizados; os gates de revisão/validação nativa estão documentados.

Capacidade atual e evidências de carga: [16 sessões](docs/SESSION_CAPACITY.md).
