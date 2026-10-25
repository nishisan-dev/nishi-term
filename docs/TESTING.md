# Estado público da validação

A etapa atual implementa perfis, busca e até quatro abas, com motor SSH independente.

Testes Go com race detector, vet interno/Windows, nove testes Playwright e compilação Windows a partir de Linux passaram na preparação desta versão. Os testes Playwright usam bridge simulado; suas capturas estão em [evidence/product-ui](evidence/product-ui/README.md).

A PoC anterior foi ensaiada no Windows com usuário comum e OpenSSH isolado; o motor também passou por carga de 64 MiB, pausa de 30 s, 100 ciclos, quatro sessões e falha de rede. Os relatórios brutos, imagens nativas e roteiros específicos da infraestrutura original ficam locais e não integram este repositório público.

A interface atual ainda precisa de validação nativa própria no Windows. Também faltam longa duração, suspensão/retomada e matriz DPI/teclados/WebView2 ausente. Ver README.md para reproduzir testes locais e compilação. Testes TestLab são opt-in e exigem configuração privada de uma fixture descartável; não são necessários à suíte padrão.

## CI e chave SSH

Adicionados testes de autenticação real em loopback com chave Ed25519 aberta/criptografada, passphrase incorreta, chave recusada, host desconhecido/trocado/revogado antes de autenticar, arquivos inválidos/grandes e limite de custo bcrypt OpenSSH. Passaram com race detector, junto da suíte anterior. Vet interno e vet Windows passaram; crossbuild local passou. A suíte frontend passou com dez testes, incluindo limpeza de passphrase e ausência de persistência.

O primeiro CI confirmou as suítes Go Linux/Windows e Chromium. O resultado definitivo dos builds e da revisão com chave deve ser consultado no [GitHub Actions](https://github.com/nishisan-dev/nishi-term/actions/workflows/ci.yml). Não confundir um build com aprovação visual nativa. Guia em [CI.md](CI.md).
