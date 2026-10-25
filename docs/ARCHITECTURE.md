# Arquitetura

## Fronteiras

```text
React + xterm.js
    │ API Wails: Start / Poll(ack) / Input / Resize / Close
Adaptador desktop (main.go, app.go)
    │ tipos Go, sem dependência gráfica no motor
internal/sshengine: Manager → sessão → x/crypto/ssh → TCP → sshd
```

O motor é proprietário de conexões, contextos, canais, estados e limites. O adaptador só traduz chamadas da interface. Não exportar objetos ssh.Client ou segredos em DTOs. O backend valida todas as entradas, independentemente dos controles React. Nenhum subprocesso ssh e nenhum servidor HTTP/WebSocket próprio na PoC.

## Decisões

- Wails **v2 estável**, versão exata no go.mod, nunca v3/beta por conveniência. React/TypeScript e xterm.js com lockfile. Go toolchain fixada pelo módulo e scripts.
- SSH em `golang.org/x/crypto/ssh`; host keys por `knownhosts.New`. Não usar `InsecureIgnoreHostKey`. Configuração de algoritmos padrão da dependência, sem reabilitar criptografia obsoleta.
- PoC usa RPC pull em vez de eventos de saída: o frontend chama Poll somente após o callback de `terminal.write(Uint8Array, callback)`. Cada resposta tem sequência; ACK inválido falha, ACK repetido permite reenviar bloco não confirmado. No máximo um bloco entregue aguardando ACK e oito blocos enfileirados. Polls simultâneos são rejeitados, sem fila de chamadas no motor.
- Um leitor stdout e um leitor stderr compartilham uma fila limitada. Ordem preservada em cada stream; SSH não define ordem total entre stdout e stderr. Bloqueio da fila suspende leitores e propaga pressão à janela SSH. Pequenos buffers internos da biblioteca continuam existindo; não alegar limite exato de RSS a partir da fila.
- Transporte via base64 conserva bytes, inclusive UTF-8 dividido entre leituras. Decodificar como Uint8Array no frontend; não converter cada fragmento isolado para string.
- Entrada tem fila limitada e envio serial. Se cheia, recusar explicitamente, informar e encerrar na UI para não continuar com comandos parcialmente transmitidos. Colagens grandes são fracionadas com limite total no cliente.
- Estados: connecting → open → draining → closed/failed. Erro na conexão: connecting → failed. Fechamento solicitado cancela qualquer estado e remove sessão do manager. Sessões terminadas retêm saída até ACK/drenagem; usuário deve fechar para liberar slot. O shutdown fecha todas.
- ID aleatório por sessão evita que respostas antigas sejam aplicadas a nova conexão. React usa geração de conexão e cleanup para interromper polls/listeners antigos. Backend limita 16 sessões ativas/conectando, incluindo entrega de saída ainda em andamento. Workers cancelados só liberam vaga após terminar. Resultados finalizados não contam; até 256 registros aguardando consumo/fechamento são preservados, com recusa explícita de novos registros ao atingir esse limite separado. A UI consulta a ocupação do motor; abas desconectadas não ocupam vagas.
- Prazo total de conexão cobre TCP, handshake, canal, PTY e shell. Contexto cancelado fecha TCP. Keepalive termina sessões sem resposta. Resize tem validação e serialização; chamadas bloqueadas são liberadas quando transporte encerra.
- Senha/passphrase existem apenas transitoriamente no formulário, na chamada Wails e no handshake. Sem logs de terminal, senha ou DTO completo. known_hosts é somente leitura. Erros públicos são categorias genéricas; nunca encaminhar texto remoto de autenticação.

## Evolução prevista

Separar `CredentialProvider` quando introduzir Windows Credential Manager/DPAPI e agent/chaves; persistir apenas referências e metadados. Rever ameaça de XSS porque o bridge nativo tem capacidade de abrir conexões. O mockup recebido orienta perfis, abas e múltiplas sessões visuais; os comportamentos estão delimitados em UI_REFERENCE.md. Não adicionar abstrações de SFTP/túneis à PoC agora.

## Fontes técnicas verificadas em 24/10/2026

- [Wails v2: runtime e bridge](https://v2.wails.io/docs/reference/runtime/intro/).
- [Wails: build](https://wails.io/docs/gettingstarted/building/) e [Windows/WebView2](https://wails.io/docs/guides/windows/).
- [xterm.js: flow control](https://xtermjs.org/docs/guides/flowcontrol/): `write` enfileira; callback confirma processamento. Nosso protocolo pull é decisão local conservadora, com desempenho ainda a medir.

Um executável gerado no Linux valida apenas compilação. A distribuição sem administrador só será aceita após ensaio em Windows com usuário padrão, runtime presente e ausente.

## Detalhes dos limites da PoC

Além dos oito blocos de saída enfileirados e um pendente de ACK, cada um dos dois leitores pode reter um bloco de até 16 KiB. Há ainda buffers da biblioteca SSH, do socket, da serialização e do xterm. Na entrada, o frontend permite 32 KiB enfileirados e uma chamada em voo; o backend permite oito blocos de 4 KiB e um em escrita. A janela de memória total precisa de medição, não se limita à soma desses canais. `known_hosts` deve ser arquivo regular de até 8 MiB; caminhos de rede não são recomendados nesta fase.

## Evolução orientada pelo mockup

Na fase de UI de produto, separar árvore/busca de perfis, workspace de abas, terminal, explorador remoto, visão do servidor e barra de status. Cada aba referencia um ID do manager e mantém seu próprio terminal e controle de fluxo. SFTP e coleta de métricas, quando implementados, serão serviços Go separados do renderizador e da entrada interativa, com filas, timeouts e cancelamento próprios. A escolha de transporte compartilhado ou separado exige medir contenção. Não implementar descoberta de diretório pelo parsing do prompt nem alimentar gráficos com dados fictícios. Ver decisões e critérios em [UI_REFERENCE.md](UI_REFERENCE.md).

## Área de trabalho implementada

`app.go` expõe ListProfiles/SaveProfile/DeleteProfile, delegando a `internal/profiles`. Arquivo em `os.UserConfigDir()/NishiTerm/profiles.json` (Windows: `%APPDATA%`). JSON versão 1, até 200 perfis/2 MiB, metadados em texto simples. Não é um cofre; o esquema não contém credenciais. Campos desconhecidos, versão incompatível, registros inválidos e JSON excedente impedem sobrescrita do documento existente.

Alterações usam temporário no mesmo diretório, Sync, Close e Rename. Diretório 0700/arquivo 0600 em POSIX; Windows depende das ACLs da pasta do usuário. Mutex serializa operações dentro do processo. Não há bloqueio entre instâncias nem durabilidade validada diante de queda de energia no Windows.

`frontend/src/session.ts` mantém TerminalSession por painel React: xterm, geração de tentativa, entrada limitada, resize serializado e Poll/ACK. Trocar aba preserva a instância. Aba oculta suspende leitura/processamento; uma resposta em voo pode ficar retida até voltar a ficar visível. Cancelamento acorda a espera; gerações descartam respostas antigas. Backpressure pode bloquear o produtor remoto. Status oculto é o último observado.

Cada aba guarda cópia dos metadados; CRUD não altera conexões existentes. A senha é solicitada por tentativa e removida do formulário/configuração temporária após iniciar a chamada. Fechar aba desmonta o controlador; Start que terminar depois tem sua sessão fechada. Shutdown do backend continua fechando todas as sessões.

## Autenticação por chave

Config de conexão aceita `authMethod`, `privateKeyPath` e `passphrase`, além da senha. Campos são mutuamente exclusivos e validados no backend; perfis não recebem segredos nem conteúdo de chave. `internal/sshengine/auth.go` lê a chave local e constrói o método SSH; parsing ocorre dentro do trabalhador da sessão, sem bloquear o manager. O arquivo tem limite de 1 MiB; o custo bcrypt OpenSSH também é limitado. Uma tentativa cancelada durante parsing não abre conexão depois. Ver [CREDENTIALS.md](CREDENTIALS.md) para limites e desenho do cofre independente do motor.

## Identidades gerenciadas e cofre — atualização autorizada

O cadastro usa confirmação de chave na primeira conexão, ligada à mesma sessão, sem autenticação antes do aceite. Chave divergente bloqueia. Caminho known_hosts externo é opcional e mantém modo estrito. Perfis antigos preservam o caminho; limpar em opções avançadas e reabrir aba ativa o gerenciamento. Ver [HOST_TRUST.md](HOST_TRUST.md).

Cofre inicial implementado com senha mestra própria, sem admin, Argon2id/AES-256-GCM, gravação criptografada explícita, bloqueio manual e após 5 minutos sem uso. Perfis não contêm segredos; resolução ocorre no backend com vínculo ao destino. Ver uso, limites e gates pendentes em [CREDENTIALS.md](CREDENTIALS.md). Estas decisões substituem as restrições históricas de senha apenas/arquivo obrigatório/cofre futuro.
