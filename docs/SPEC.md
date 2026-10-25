# NishiTerm — especificação inicial

Status: base técnica definida pelo pedido de 24/10/2026; mockup recebido e analisado; requisitos completos da conversa ainda pendentes de recuperação. Esta especificação não presume que descreve todo o produto discutido.

## Origem e lacunas

Pedido atual: Go, Wails v2 estável, React, xterm.js, execução Windows sem administrador, compilação Linux → Windows e motor SSH independente da interface. Prioridades: confiabilidade, credenciais, backpressure e sessões.

Referência: conversa `6add2281-30b4-83e9-b102-17cd272a9f67` (Interfaces gráficas em Go). A leitura retornou cinco turnos recentes, respostas como marcadores internos e nenhum anexo. Confirma preocupação com beta, compilação cruzada e solicitação de mockup, mas não fornece a discussão completa. Posteriormente, o usuário enviou o mockup, agora preservado em `docs/references/nishiterm-mockup.jpg` e analisado em [UI_REFERENCE.md](UI_REFERENCE.md). A imagem mostra abas, conexões agrupadas, busca, cofre, SFTP e métricas; seus comportamentos completos e inclusão no MVP ainda precisam ser especificados. A exportação da conversa permanece pendente.

## Objetivo e escopo desta entrega

Validar uma sessão SSH interativa em janela nativa com PTY remoto, teclado, redimensionamento e saída binária íntegra. Não executar shell local nem depender de ssh.exe, serviço, driver ou elevação no cliente. A PoC inicial evoluiu para a etapa de produto aprovada abaixo.

| ID | Requisito | Critério de aceitação |
|---|---|---|
| F01 | Conectar a host, porta e usuário explicitamente informados | Sessão autenticada e shell PTY xterm-256color |
| F02 | Autenticação efêmera por senha ou chave | Senha limpa do formulário ao enviar; sem persistência, telemetria ou logs de conteúdo |
| F03 | Verificação estrita de host | Identidade gerenciada: desconhecida exige confirmação explícita; alterada bloqueia. Arquivo externo opcional: modo estrito, incluindo revogação; sem aceite automático |
| F04 | Terminal interativo | ANSI, Unicode fragmentado, Ctrl-C e resize; saída entregue como bytes ao xterm |
| F05 | Backpressure ponta a ponta | Fila de saída limitada; próximo lote somente após callback de processamento do xterm; sem descarte silencioso |
| F06 | Gerenciar sessões | ID opaco por tentativa, cancelamento durante conexão e execução, fechamento idempotente; máximo de 16 sessões ativas/conectando no motor; abas desconectadas ou já finalizadas não ocupam vagas |
| F07 | Erros e término explícitos | Estado conectando/aberto/drenando/fechado/falhou; erro público sem payload de credencial; última saída drenada antes do término |
| F08 | Compilar Windows a partir de Linux | Produzir executável Windows amd64 com Wails v2 estável; testar execução separadamente em Windows |
| F09 | Sem privilégios administrativos | Executável portable, dados WebView em diretório do usuário, nenhuma escrita em Program Files/HKLM nem instalador elevado |
| F10 | Motor desacoplado | Testes de SSH/sessões rodam sem Wails, React ou bibliotecas gráficas |

## Limites deliberados

Versão atual: senha ou arquivo de chave privada (passphrase opcional), identidade gerenciada conforme HOST_TRUST.md ou arquivo externo confiável, sem reconexão automática, retomada, agent forwarding, transferência de arquivos, plugins; armazenamento de segredos somente pelo cofre criptografado opt-in. Nova conexão é nova sessão; não repetir comandos automaticamente após falhas. A existência de uma chave coletada por ssh-keyscan não comprova sua autenticidade.

Metas iniciais para validação, não medições já obtidas: saída em blocos de até 16 KiB, oito blocos na fila e um bloco pendente de confirmação por sessão; entrada em oito blocos de até 4 KiB; scrollback de 2.000 linhas. Considerar separadamente buffers SSH, serialização base64 (+33%), WebView e renderizador. Conexão/handshake/abertura de PTY com prazo total de 15 s; keepalive a cada 10 s, resposta em até 5 s; fechar transporte para desbloquear operações. Poll de saída com espera máxima de 200 ms. Cancelamento deve liberar recursos em até 2 s nos testes locais.

Segurança: dados remotos são texto/bytes de terminal, nunca HTML; sem abertura automática de links ou acesso remoto ao clipboard. CSP sem scripts remotos. Não manter senha em storage. Go/JS usam memória gerenciada: limpeza de referências não garante zeroização. Cofre nativo e chaves privadas exigem desenho e validação próprios antes do MVP.

## Windows

Alvo inicial: Windows 10/11 amd64 com WebView2 compatível disponível. Compilar um EXE não valida execução, políticas corporativas, SmartScreen, DPI ou teclado. Máquinas sem WebView2 e sem permissão de instalar exigem avaliar distribuição Fixed Version privada; tamanho, atualizações e permissões são gate de release. A PoC deve falhar claramente se runtime faltar, sem tentar instalar/elevar. ARM64 e outras plataformas não são critérios desta fase.

## Referência visual recebida

Adotar a hierarquia e direção visual descritas em [UI_REFERENCE.md](UI_REFERENCE.md), com requisitos V01–V07 e fases próprias. O mockup orienta a evolução da UI; a entrega inicial F01–F10 foi ampliada por F11–F15 abaixo. Indicadores de proteção, métricas e serviços ativos devem refletir capacidades implementadas e dados reais, nunca valores ilustrativos da imagem.

## Etapa de produto aprovada — 24/10/2026

| ID | Requisito | Critério de aceitação |
|---|---|---|
| F11 | Perfis persistentes | CRUD de até 200 perfis: ID, nome/grupo/host/porta/usuário/caminho known_hosts; sem segredos |
| F12 | Grupos e busca | Grupos derivados dos perfis; Ctrl+K busca nome/grupo/host/usuário ignorando caixa e acentos |
| F13 | Quatro abas | Terminal e ciclo de vida próprios; abrir não conecta; editar/excluir perfil preserva cópia usada pela aba |
| F14 | Fechamento e fluxo | Confirmar fechamento durante conexão/sessão; limpar Start tardio; aba oculta pausa leitura com no máximo um frame recebido |
| F15 | Estrutura visual | Tema escuro, árvore, busca, abas e status; terminal dominante; layout verificado em browser 1280×800 e 800×540 |

Grupos vazios não são persistidos. Abas encerradas ocupam limite até fechar. Não restaurar sessões nem senhas ao reiniciar. Status oculto é o último observado; a pausa pode bloquear a produção remota quando os buffers SSH enchem. Cofre, chaves, SFTP e métricas ficam fora desta aprovação. Guia: [PRODUCT_WORKSPACE.md](PRODUCT_WORKSPACE.md).

## Etapa 3B — autorização de CI e chave SSH

Autenticação por arquivo de chave privada com passphrase opcional autorizada após a etapa 3A. Escolha explícita na aba, sem fallback, agent ou persistência de credenciais. Known_hosts continua estrito. Testes automatizados Linux/Windows e builds Windows via Linux + Linux nativo no GitHub Actions. Detalhes em [CI.md](CI.md) e [CREDENTIALS.md](CREDENTIALS.md).

Requisito permanente: cofre de senhas com autenticação explícita para desbloquear, utilizável sem administrador. Implementação inicial disponível; proteger dados pela conta do sistema sem novo desbloqueio não atende sozinho ao requisito.

## Identidades gerenciadas e cofre — atualização autorizada

O cadastro usa confirmação de chave na primeira conexão, ligada à mesma sessão, sem autenticação antes do aceite. Chave divergente bloqueia. Caminho known_hosts externo é opcional e mantém modo estrito. Perfis antigos preservam o caminho; limpar em opções avançadas e reabrir aba ativa o gerenciamento. Ver [HOST_TRUST.md](HOST_TRUST.md).

Cofre inicial implementado com senha mestra própria, sem admin, Argon2id/AES-256-GCM, gravação criptografada explícita, bloqueio manual e após 5 minutos sem uso. Perfis não contêm segredos; resolução ocorre no backend com vínculo ao destino. Ver uso, limites e gates pendentes em [CREDENTIALS.md](CREDENTIALS.md). Estas decisões substituem as restrições históricas de senha apenas/arquivo obrigatório/cofre futuro.

## Incremento autorizado em 25/10/2026

Organizador de abas com busca, reordenação sem reconectar e fechamento de grupos confirmado. Navegador SFTP somente leitura sob o terminal, caminho independente do shell, filtro e cancelamento. Até 2.000 entradas, 1 MiB por resposta e 15 segundos por operação; ver [SFTP.md](SFTP.md). Transferências e métricas permanecem pendentes.
