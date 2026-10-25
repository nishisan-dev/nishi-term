# Capacidade de sessões

A partir de 24/10/2026, o motor admite 16 sessões ativas ou conectando. A admissão é atômica, inclusive em chamadas simultâneas. Aguardando confirmação de identidade também conta. Cancelar não libera a vaga enquanto o worker ainda termina; entrega final de saída em andamento continua ativa. Resultados já finalizados não contam, mesmo com saída aguardando leitura.

Abas desconectadas não ocupam vagas; o rodapé distingue sessões e abas. Não há limite fixo de abas inativas, mas cada terminal retém seu histórico e consome memória. O motor permanece a autoridade para recusar a 17ª conexão. Não há reconexão automática.

Mantidos por sessão: saída 8 × 16 KiB mais frame pendente de ACK e buffers de leitura/biblioteca, entrada 8 × 4 KiB, fila frontend 32 KiB e scrollback de 2.000 linhas. ACK somente após processamento pelo xterm. O limite separado de 256 registros retidos impede crescimento indefinido em clientes que nunca consomem/fecham resultados; ao atingir esse limite, novas tentativas são recusadas explicitamente, sem descartar saída final.

## Validação local

Evidência: [log bruto](evidence/session-capacity/linux-loopback.txt). Execução com `go test -race -run TestSixteen -count=1 -v ./internal/sshengine`, cliente e servidor SSH criptografado no mesmo processo, em loopback Linux. Não representa OpenSSH remoto, consumo da WebView ou desempenho de rede real.

- 32 admissões simultâneas: exatamente 16 aceitas.
- 16 handshakes aguardando consentimento: 17º recusado, sem enviar credenciais.
- Sessões terminadas/com falha: deixam de ocupar vagas sem perder saída final.
- 16 × 4 MiB = 64 MiB íntegros, verificados por SHA-256, em 5,668 s incluindo pausa de 3 s.
- Consumo pausado: oito blocos por fila; heap de 52.106.288 para 52.116.304 bytes entre amostras, 39 descritores e 307 goroutines estáveis. Esses números incluem cliente, servidor e detector de concorrência.
- Uma sessão congestionada e 15 interativas: 300 respostas, p95 de 1,689 ms; encerramento da congestionada em 0,479 ms, substituição aceita, demais sessões continuam respondendo.
- Quatro ondas de 16 conexões: 64 ciclos; goroutines 2 → 2 e descritores 6 → 6 após limpeza.
- Interface Chromium com bridge simulado: 17 abas permitidas, 16 conexões, recusa da 17ª e reaproveitamento após desconectar sem fechar a aba. Não substitui ensaio nativo.

Uma repetição local da suíte encontrou a espera de três segundos do teste de chave criptografada excedida durante compilação concorrente. O caso isolado passou; o teste agora permite 20 segundos para derivação criptográfica instrumentada e falha imediatamente em estado inesperado. Prazos do produto não foram alterados.

## Gate de build e limites da evidência

O CI executa os testes de carga com detector de concorrência em Linux e Windows; guarda `go-tests-<sistema>` em JSON. Os builds dependem desses testes e da suíte React/Chromium. O roteiro opt-in OpenSSH também foi ampliado para 16, mas não foi executado nesta validação.

Ainda pendentes: 16 terminais na interface nativa com medição ponta a ponta, uso prolongado, combinações com máquinas lentas e rede real. Latências locais não são promessa de desempenho do produto.
