# Revisão da manhã — 25/10/2026

## Pronto para revisar

1. Conecte um perfil e abra **SFTP**. Navegue por pastas, use o caminho e o filtro, atualize e feche o painel. A altura do terminal se ajusta ao espaço disponível.
2. Abra várias abas e clique **Abas (N)**. Localize por nome/grupo/host, mova com as setas e selecione uma aba. Reordenar preserva sessões e histórico.
3. No organizador, escolha **Fechar grupo**. Cancele a confirmação para preservar tudo; confirme para encerrar as abas daquele grupo.
4. Confira os fluxos em janela compacta. Capturas de referência estão em `docs/evidence/product-ui/sftp-fixture.png`, `sftp-compact-fixture.png` e `tab-organizer-fixture.png` (bridge simulado).

## Decisões desta entrega

SFTP começa como navegador somente leitura, com cancelamento e limites explícitos. Os detalhes estão em [SFTP.md](SFTP.md). Não há botões simulados de transferência ou gráficos com dados fictícios. As proteções do cofre, host keys e limites das 16 sessões permanecem.

## Próximas entregas propostas

- Upload/download por streaming, progresso e cancelamento; confirmação de sobrescrita e limpeza de arquivos parciais.
- Painel de métricas reais, inicialmente Linux, com coleta opcional, frequência limitada e indicação clara de indisponibilidade. Roteadores e outros sistemas precisam de adaptadores próprios.
- Troca de senha mestra e exportação/importação criptografada do cofre, com backup e validação antes de substituir dados.
- Ensaio nativo prolongado em Linux e Windows, incluindo 16 terminais e navegação SFTP simultânea.
