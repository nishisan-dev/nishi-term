# Integração contínua

O workflow `.github/workflows/ci.yml` executa em push de main, pull requests e acionamento manual. Testes e builds têm permissão somente de leitura; somente o job de publicação recebe `contents: write` e `actions: read`. Actions fixadas por SHA, limite de duração e cancelamento de execução obsoleta. Usa apenas o token temporário do GitHub Actions, sem secrets pessoais nem acesso ao laboratório privado.

1. Go: testes com race detector e vet, no Ubuntu 22.04 e Windows.
2. React: instalação pelo lockfile, build TypeScript/Vite e Playwright/Chromium.
3. Após ambos passarem: Windows amd64 compilado no Linux e build nativo Linux amd64/GTK3/WebKitGTK 4.1, mais vet do adaptador.

Artifacts ficam 14 dias na execução do GitHub Actions: `NishiTerm-windows-amd64` e `NishiTerm-linux-amd64`. Cada um contém tar.gz com binário, LICENSE, NOTICE, README e SHA256SUMS. São builds de desenvolvimento sem assinatura. Após todos os gates, os mesmos pacotes são publicados em GitHub Releases, conforme a política abaixo. Compilar Windows não valida sua interface em execução; testes Go no runner Windows validam somente os pacotes internos.

## Linux para teste local

O build é feito no Ubuntu 22.04. Instale os runtimes GTK3/WebKitGTK 4.1 da sua distribuição. Em Ubuntu: `sudo apt install libgtk-3-0 libwebkit2gtk-4.1-0` (versões mais novas podem usar nomes com sufixo t64). O aplicativo deve ser executado como usuário comum numa sessão gráfica, sem sudo.

Baixe o artifact, extraia o ZIP do GitHub e depois o tar.gz. Na pasta extraída, confira `sha256sum -c SHA256SUMS` e execute `./NishiTerm`. O tar preserva o bit executável. Outras distribuições/versões de bibliotecas ainda não foram certificadas.

Para compilar localmente: Go/Node conforme README, `libgtk-3-dev libwebkit2gtk-4.1-dev pkg-config`, depois `./scripts/build-linux.sh`. Saída em `build/bin/NishiTerm`. A compilação não instala bibliotecas no sistema automaticamente.

## Releases e links permanentes

Push em `main` ou acionamento manual sobre `main` publica automaticamente uma release após **todos** os testes e os dois builds passarem. PRs e outras branches nunca publicam. O job baixa somente os artifacts da própria execução, confere os checksums internos e gera um ZIP adicional para Windows. Mantém os dois tar.gz para compatibilidade com consumidores existentes e publica `SHA256SUMS` dos três arquivos distribuídos.

Cada release recebe uma tag única `build-<run_number>-<run_attempt>`, vinculada ao SHA testado, e é marcada como `latest`. Reexecutar o workflow gera outra tag, sem sobrescrever releases anteriores. Antes de publicar, consulta `main` e ignora execuções cujo commit já ficou para trás. O cancelamento por branch também evita que execuções obsoletas continuem normalmente; não há transação entre atualização de branch e publicação, portanto um push simultâneo pode deixar o build anterior em latest até o próximo CI passar.

A CLI cria um rascunho, envia todos os assets e só então publica, preservando o latest anterior se o upload falhar. Um rascunho residual de uma falha pode ser removido manualmente. Não há rotação automática das releases publicadas; elas preservam o histórico para baixar versões anteriores. O nome `latest` significa último build aprovado disponível, não certificação de produto estável. Para permitir o endpoint latest do GitHub, as releases não usam a flag `prerelease`, mas título/notas/README deixam explícito que são builds de desenvolvimento.

Links estáveis:

- [Windows amd64 ZIP](https://github.com/nishisan-dev/nishi-term/releases/latest/download/NishiTerm-windows-amd64.zip)
- [Linux amd64 tar.gz](https://github.com/nishisan-dev/nishi-term/releases/latest/download/NishiTerm-linux-amd64.tar.gz)
- [SHA256SUMS](https://github.com/nishisan-dev/nishi-term/releases/latest/download/SHA256SUMS)

No Linux, com os três pacotes e o manifesto na mesma pasta, execute `sha256sum --check SHA256SUMS`. Para conferir somente o pacote baixado, compare seu `sha256sum <arquivo>` com a linha correspondente. No Windows use `Get-FileHash .\NishiTerm-windows-amd64.zip -Algorithm SHA256` e compare com o manifesto. Após extrair, o `SHA256SUMS` interno corresponde ao executável.

Só são publicados Windows/Linux amd64, os alvos hoje compilados e testados. Outras arquiteturas exigem matriz e gates próprios. Os checksums detectam corrupção; não substituem assinatura digital. Distribuir para avaliação não elimina os gates de validação nativa, WebView2, DPI/teclado e revisão do cofre documentados no projeto.
