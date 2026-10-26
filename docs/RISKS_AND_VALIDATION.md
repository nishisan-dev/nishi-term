# Riscos e plano de validação

| Risco | Mitigação | Validação / gate |
|---|---|---|
| Conversa incompleta e mockup conceitual | Mockup recebido; distinguir elementos visuais de comportamentos definidos | Obter exportação da conversa e fechar operações/aceitação antes do MVP |
| Linux → Windows / dependências nativas | Wails v2 e transporte Go; script de build | Gerar PE amd64 no Linux; executar em Windows 10/11 sem admin |
| WebView2 ausente/política corporativa | Não instalar automaticamente; avaliar Fixed Version | VM sem runtime, offline e com usuário padrão; erro compreensível; validar alternativa privada antes de release |
| MITM/host key trocada | known_hosts estrito | Chave correta aceita; desconhecida, divergente e revogada recusadas antes de autenticação |
| Vazamento de segredo | Sem persistência/logs; errors genéricos | Busca em artefatos/logs com senha-canário; inspeção manual bridge e storage; considerar dumps como risco residual |
| Saída intensa/UI lenta | Fila e ACK limitados | Produtor envia ≥64 MiB; pausar consumo por 30 s; memória estabiliza e retoma sem perda; Ctrl-C p95 <250 ms em LAN (meta a medir) |
| EOF antes da renderização | Drenar fila e confirmar último frame | Saída final permanece íntegra em exit 0 e não zero |
| Rede interrompida | Cancelamento fecha socket; keepalive limitado | TCP blackhole: sessão termina em até 15 s aproximadamente; fechamento manual <2 s |
| Conexão/handshake travado | Prazo total de 15 s | Servidor aceita TCP e não fala SSH; cancelar e verificar liberação |
| Goroutines/sockets órfãos | Manager proprietário e cleanup | 100 ciclos conectar/cancelar/fechar; contagem retorna próximo ao baseline; race detector |
| UTF-8 e ANSI fragmentados | Bytes/base64, parser incremental do xterm | Emoji e acentos partidos entre frames, cores, vim/top, colagem, Ctrl-C, resize |
| Entrada congestionada | Fila limitada, rejeição explícita | Servidor não lê; fila satura sem memória crescente; UI interrompe sessão com aviso |
| Concorrência e eventos antigos | IDs, geração frontend, mutex/limites | Poll concorrente, ACK inválido/repetido, fechamento durante dial/read/write/resize |
| Sequências de terminal maliciosas | Sem HTML/links/clipboard automático | OSC 52, hyperlinks e texto HTML não executam ações nativas |
| Mudanças de dependências | Versões exatas/lock | Revisar avisos com govulncheck/npm audit; achados são gate antes de distribuição |

## Matriz manual ainda necessária

Windows 10 e 11 amd64, usuário padrão, DPI 100/150/200%, teclado PT-BR/US, IME, WebView2 presente/ausente; Linux de desenvolvimento. OpenSSH real com senha permitida em ambiente isolado: login/logout, auth inválida, resize, Ctrl-C em `yes`, sessão longa, perda de rede e encerramento da janela. Nunca executar carga em servidor de produção.

## Evidências

Resultados desta execução serão registrados em docs/TESTING.md. Testes locais do motor não comprovam renderização, latência interativa, consumo global de memória nem compatibilidade Windows.

## Riscos adicionais identificados no mockup (fases futuras)

| Risco | Validação proposta |
|---|---|
| Abas ocultas acumulam saída | Várias sessões com saída contínua, alternância de abas, memória estável e fechamento sem vazamento |
| SFTP ou métricas degradam interatividade | Transferência/coleta simultâneas ao terminal; medir latência e verificar cancelamento e isolamento de falhas |
| Métricas incompatíveis com roteadores/appliances | Testar capacidades ausentes, timeout, dados antigos e ausência de privilégios; mostrar indisponibilidade |
| Caminho SFTP diverge do shell | Navegação independente e diretórios com caracteres especiais; nenhuma dedução pelo prompt |
| Rótulos induzem falsa confiança | Cofre, cifra e status só indicam estado efetivo; erro e desconexão removem indicadores obsoletos |
| Layout denso oculta terminal/foco | Comparar com UI_REFERENCE.md em diferentes tamanhos/DPI; teclado, contraste e painéis recolhidos |

## Observação do ensaio OpenSSH real

A fixture anuncia múltiplos tipos de chave. Com apenas a chave Ed25519 no arquivo de confiança, o cliente recusou a sessão porque outra chave foi negociada; com todas as chaves obtidas diretamente do servidor pelo canal administrativo, o teste passou. A PoC não seleciona algoritmos em função das entradas do known_hosts. Validar esse caso de UX antes do MVP; jamais resolver trocas de chave com aceite automático.

## Etapa 3A — próximos gates

- Abas ocultas: browser confirmou pausa/retomada e isolamento. Repetir saída intensa e alternância no WebView2 real. Status oculto é o último observado; buffers cheios podem bloquear produtores remotos.
- Persistência: Go verifica CRUD, concorrência no processo e rejeição de arquivo inválido sem sobrescrita. Validar ACLs, falha de escrita e substituição no Windows. Evitar editar perfis com várias instâncias: ainda não há bloqueio entre processos.
- Ciclo de vida: testes cobrem Start tardio e isolamento. Repetir fechar janela/reabrir e suspender/retomar no Windows.
- Layout: capturas verificadas em browser 1280×800 e 800×540 com bridge simulado; DPI, IME e teclado PT-BR ainda pendentes.
- Metadados são texto simples. Credenciais continuam efêmeras; cofre/chaves exigem etapa própria.

## Identidades gerenciadas e cofre — atualização autorizada

O cadastro usa confirmação de chave na primeira conexão, ligada à mesma sessão, sem autenticação antes do aceite. Chave divergente bloqueia. Caminho known_hosts externo é opcional e mantém modo estrito. Perfis antigos preservam o caminho; limpar em opções avançadas e reabrir aba ativa o gerenciamento. Ver [HOST_TRUST.md](HOST_TRUST.md).

Cofre inicial implementado com senha mestra própria, sem admin, Argon2id/AES-256-GCM, gravação criptografada explícita, bloqueio manual e após 5 minutos sem uso. Perfis não contêm segredos; resolução ocorre no backend com vínculo ao destino. Ver uso, limites e gates pendentes em [CREDENTIALS.md](CREDENTIALS.md). Estas decisões substituem as restrições históricas de senha apenas/arquivo obrigatório/cofre futuro.

- RTT ICMP não representa disponibilidade: firewalls e política local podem impedir resposta. Sem fallback/elevacão; ocultar texto em falhas. Validar loopback v4/v6 nas duas plataformas, cancelamento, concorrência, expiração e independência do estado SSH.
- Preferência de cofre inicia conexão ao abrir: preservar confirmação de identidade, desbloqueio/2FA, limite de sessões e cancelamento. Testar perfis antigos, caminho de chave e cópias de perfis em abas abertas. Ver CONNECTION_WORKFLOW.md.

- Observabilidade: canal SSH bloqueado não pode ser cancelado na abertura sem encerrar transporte. Retenção máxima de um worker por sessão, sem novas tentativas concorrentes; UI retorna em 3 s. Testar bloqueio de abertura/exec e terminal/SFTP responsivos. Rede agregada pode contar interfaces virtuais em duplicidade; aviso explícito. Dados ausentes/reset não viram zeros. Histórico limitado e sem persistência. Ver OBSERVABILITY.md.
