# Roadmap por gates

| Fase | Entrega | Gate para avançar |
|---|---|---|
| 0 — especificação | SPEC, arquitetura, instruções, riscos | Mockup recebido e registrado; recuperar conversa completa e revisar decisões em aberto |
| 1 — PoC SSH (esta entrega) | Motor isolado, terminal mínimo, fluxo limitado, cancelamento, teste SSH local e EXE Windows | Testes automatizados, build e ensaio manual do executável normal em Windows com SSH real realizados; ver TESTING.md |
| 2 — robustez | Testes com OpenSSH, falhas de rede, carga prolongada, múltiplas sessões, métricas sem conteúdo | Carga 64 MiB/pausa 30 s, rede, cancelamento, 100 ciclos e quatro sessões validados no motor em Windows; longa duração e medição ponta a ponta pendentes (TESTING.md) |
| 3A — área de trabalho (implementada) | Estrutura visual, perfis/grupos, busca Ctrl+K e quatro abas | Testes locais e crossbuild passaram; ensaio da nova UI no Windows pendente |
| 3B — credenciais (futura) | Chaves/agent, cofre Windows e políticas de confiança | Modelo de ameaças revisto; segredos protegidos; UX de host key validada |
| 4 — MVP | Escopo funcional reconciliado com conversa e mockup; avaliar SFTP e métricas; acessibilidade, atalhos e estados | Aceitação funcional e visual; sem expansão especulativa de escopo |
| 5 — distribuição | Portable, política WebView2, assinatura e pipeline reprodutível | Execução sem admin em máquina limpa e ambiente corporativo; rollback documentado |

Não tratar fase 1 como produto pronto. Não implementar fases posteriores antes de avaliar os resultados. SFTP e métricas aparecem no mockup e são candidatos à fase 4, após definição das operações e plataformas suportadas. Túneis, broadcast, sincronização e plugins continuam sem requisito confirmado.
