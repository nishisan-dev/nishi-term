# Integração contínua

O workflow `.github/workflows/ci.yml` executa em push de main, pull requests e acionamento manual. Permissão somente de leitura, actions fixadas por SHA, limite de duração e cancelamento de execução obsoleta. Não usa secrets nem acessa o laboratório privado.

1. Go: testes com race detector e vet, no Ubuntu 22.04 e Windows.
2. React: instalação pelo lockfile, build TypeScript/Vite e Playwright/Chromium.
3. Após ambos passarem: Windows amd64 compilado no Linux e build nativo Linux amd64/GTK3/WebKitGTK 4.1, mais vet do adaptador.

Artifacts ficam 14 dias na execução do GitHub Actions: `NishiTerm-windows-amd64` e `NishiTerm-linux-amd64`. Cada um contém tar.gz com binário, LICENSE, NOTICE, README e SHA256SUMS. São builds de desenvolvimento sem assinatura, não releases. Compilar Windows não valida sua interface em execução; testes Go no runner Windows validam somente os pacotes internos.

## Linux para teste local

O build é feito no Ubuntu 22.04. Instale os runtimes GTK3/WebKitGTK 4.1 da sua distribuição. Em Ubuntu: `sudo apt install libgtk-3-0 libwebkit2gtk-4.1-0` (versões mais novas podem usar nomes com sufixo t64). O aplicativo deve ser executado como usuário comum numa sessão gráfica, sem sudo.

Baixe o artifact, extraia o ZIP do GitHub e depois o tar.gz. Na pasta extraída, confira `sha256sum -c SHA256SUMS` e execute `./NishiTerm`. O tar preserva o bit executável. Outras distribuições/versões de bibliotecas ainda não foram certificadas.

Para compilar localmente: Go/Node conforme README, `libgtk-3-dev libwebkit2gtk-4.1-dev pkg-config`, depois `./scripts/build-linux.sh`. Saída em `build/bin/NishiTerm`. A compilação não instala bibliotecas no sistema automaticamente.
