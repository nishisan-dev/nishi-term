# Arquivos remotos — primeira etapa

Autorizada em 25/10/2026 como avanço em direção ao mockup. O botão **SFTP** abre um painel abaixo do terminal da sessão ativa. A listagem é real e somente leitura: nome, tamanho em bytes, data e permissões. Pastas e links podem ser abertos; links para arquivos retornam erro de diretório. Há caminho editável, diretório superior, filtro local, atualização e cancelamento. Nomes remotos são texto React, nunca HTML.

## Transporte e limites

O motor mantém a referência ao cliente SSH já autenticado e abre um canal SFTP temporário para cada listagem. Não pede outra senha, não aceita outra host key e não abre conexão independente. Requer uma sessão com shell aberto; servidores exclusivamente SFTP ainda não são suportados. Não acompanha `cd` digitado no terminal.

- Uma operação por sessão, sem fila de requisições concorrentes.
- Prazo total de 15 segundos, incluindo abertura do canal.
- Até 1 MiB recebido por operação e até 2.000 itens retornados. Diretórios maiores falham explicitamente; não são apresentados como listagem completa.
- Biblioteca `github.com/pkg/sftp` fixada em v1.13.10, pacote individual limitado por ela a 256 KiB. O limite de bytes também limita a acumulação antes de verificar a quantidade de entradas.
- Cancelamento fecha somente o canal SFTP depois de sua abertura. Se o servidor travar na abertura do canal, o transporte SSH inteiro precisa ser fechado, pois essa API SSH não recebe contexto. A interface passa a mostrar a desconexão.
- Desconectar/fechar a aba cancela leituras; ocultar o painel ou alternar abas cancela sua leitura pendente. Ao reabrir, a listagem começa novamente no diretório inicial remoto.
- Sem upload, download, exclusão, edição ou execução de comandos. Não há conteúdo de arquivo enviado ao renderer.

A implementação usa as APIs documentadas do [pkg/sftp](https://pkg.go.dev/github.com/pkg/sftp@v1.13.10). O motor continua independente de Wails e React.

## Testes e revisão

Testes com servidor SSH/SFTP efêmero em loopback cobrem navegação, metadados, pasta vazia, caminho inexistente, servidor sem SFTP, cancelamento de subsystem travado, recusa de operação concorrente, diretório acima do limite e terminal ainda responsivo após essas operações. Toda a suíte inclui a carga de 16 sessões anterior.

Testes Chromium usam bridge simulado, incluindo nome parecido com HTML que permanece texto, filtro, erro, cancelamento e ajuste do terminal ao fechar o painel. Capturas em `docs/evidence/product-ui/sftp*-fixture.png` são dados fictícios; não comprovam execução nativa. Teste manual com OpenSSH real e interface Windows permanece pendente.

Próxima etapa: transferências com seleção nativa de arquivo, progresso, fila limitada, cancelamento, política explícita de sobrescrita e arquivos temporários. Não implementar transferência carregando o arquivo inteiro em memória ou passando conteúdo pelo JavaScript.
