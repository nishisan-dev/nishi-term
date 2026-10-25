# Contribuindo com NishiTerm

Leia AGENTS.md e os documentos de arquitetura e requisitos antes de alterar comportamento. Abra uma issue para discutir mudanças de escopo. Envie mudanças pequenas com descrição do problema, solução e validação.

## Licença e autoria

Contribuições intencionalmente enviadas para inclusão seguem Apache-2.0, salvo declaração explícita em contrário, conforme a seção 5 da licença. Você mantém a autoria de suas contribuições; não exigimos transferência de copyright. Preserve LICENSE, NOTICE e avisos de terceiros. Não envie material que você não tenha direito de contribuir.

## Validação

- Motor/armazenamento: `go test -race ./internal/...` e `go vet ./internal/...`.
- Interface: `npm ci --prefix frontend`, `npm run build --prefix frontend` e `npm test --prefix frontend`.
- Integração: `./scripts/build-windows.sh`; compilação não substitui execução Windows.

Não inclua credenciais, known_hosts reais, dados de clientes ou logs de terminal. Use fixtures efêmeras. Relatórios operacionais e scripts específicos do laboratório original ficam locais; os testes normais não precisam deles.
