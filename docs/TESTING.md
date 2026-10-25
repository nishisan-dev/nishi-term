# Estado público da validação

A etapa atual implementa perfis, busca e até quatro abas, com motor SSH independente.

Testes Go com race detector, vet interno/Windows, nove testes Playwright e compilação Windows a partir de Linux passaram na preparação desta versão. Os testes Playwright usam bridge simulado; suas capturas estão em [evidence/product-ui](evidence/product-ui/README.md).

A PoC anterior foi ensaiada no Windows com usuário comum e OpenSSH isolado; o motor também passou por carga de 64 MiB, pausa de 30 s, 100 ciclos, quatro sessões e falha de rede. Os relatórios brutos, imagens nativas e roteiros específicos da infraestrutura original ficam locais e não integram este repositório público.

A interface atual ainda precisa de validação nativa própria no Windows. Também faltam longa duração, suspensão/retomada e matriz DPI/teclados/WebView2 ausente. Ver README.md para reproduzir testes locais e compilação. Testes TestLab são opt-in e exigem configuração privada de uma fixture descartável; não são necessários à suíte padrão.

## CI e chave SSH

Adicionados testes de autenticação real em loopback com chave Ed25519 aberta/criptografada, passphrase incorreta, chave recusada, host desconhecido/trocado/revogado antes de autenticar, arquivos inválidos/grandes e limite de custo bcrypt OpenSSH. Passaram com race detector, junto da suíte anterior. Vet interno e vet Windows passaram; crossbuild local passou. A suíte frontend passou com dez testes, incluindo limpeza de passphrase e ausência de persistência.

O primeiro CI passou integralmente: suítes Go Linux/Windows, Chromium e builds Linux/Windows. O resultado definitivo dos builds e da revisão com chave deve ser consultado no [GitHub Actions](https://github.com/nishisan-dev/nishi-term/actions/workflows/ci.yml). Não confundir um build com aprovação visual nativa. Guia em [CI.md](CI.md).

## Identidades gerenciadas e cofre — atualização autorizada

O cadastro usa confirmação de chave na primeira conexão, ligada à mesma sessão, sem autenticação antes do aceite. Chave divergente bloqueia. Caminho known_hosts externo é opcional e mantém modo estrito. Perfis antigos preservam o caminho; limpar em opções avançadas e reabrir aba ativa o gerenciamento. Ver [HOST_TRUST.md](HOST_TRUST.md).

Cofre inicial implementado com senha mestra própria, sem admin, Argon2id/AES-256-GCM, gravação criptografada explícita, bloqueio manual e após 5 minutos sem uso. Perfis não contêm segredos; resolução ocorre no backend com vínculo ao destino. Ver uso, limites e gates pendentes em [CREDENTIALS.md](CREDENTIALS.md). Estas decisões substituem as restrições históricas de senha apenas/arquivo obrigatório/cofre futuro.

Nesta etapa, a suíte de interface passou com 12 testes. As suítes internas passaram com race detector, incluindo hosttrust, vault e credentials. Vet interno/Windows e crossbuild local passaram. Capturas de confirmação/cofre são fixtures de browser; a execução nativa deste novo fluxo ainda requer ensaio do usuário.

## Feedback ao abrir e conectar

A interface mostra preparação do terminal e progresso de conexão antes de inicialização/reset e chamada Start. Um teste com Start pendente verifica que o indicador já existe ao entrar no bridge, que cancelar continua disponível e que uma resposta tardia é encerrada sem Poll. O formulário tem controles de mesma altura e ações alinhadas; a animação respeita redução de movimento. Capturas usam bridge simulado. O relato do usuário motivou a mudança; não foi medido ganho de latência de rede.
