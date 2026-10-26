# Observabilidade SSH — primeiro incremento

Painel **Métricas** na sessão conectada, inspirado no mockup: CPU, memória, disco `/`, recepção/transmissão de rede e uptime. Recolhível, inicialmente fechado. Em janelas estreitas abre sobre a lateral do terminal; em janelas largas reserva espaço próprio. Não cria novas conexões nem usa credenciais adicionais.

## Coleta e semântica

Somente servidores Linux com shell POSIX, `uname`, `cat`, `df -Pk /` e `/proc` acessíveis. Nenhum agente, arquivo remoto ou sudo. Um comando fixo de leitura roda em canal `exec` separado, sem PTY e sem passar pelo terminal interativo. Não interpola dados da interface. Servidores sem suporte/permissões apresentam indisponibilidade.

- CPU: diferença entre contadores agregados de `/proc/stat`, com idle+iowait como tempo ocioso e sem duplicar guest/guest_nice; normalizada em 0–100% para o conjunto de CPUs. iowait é uma estimativa do kernel e pode recuar; amostras incoerentes não geram taxa.
- Memória: `MemTotal - MemAvailable`; cache recuperável não é tratado integralmente como memória usada. Requer MemAvailable.
- Disco: bytes usados/totais de `/`, convertidos de blocos de 1024 bytes. Percentual é usado/total, não a coluna Capacity de df (que considera blocos reservados).
- Rede: soma das diferenças RX/TX de interfaces, excluindo `lo`. Interfaces virtuais/bridges podem contabilizar tráfego mais de uma vez; não é medição de utilização do link. Unidade B/s com múltiplos binários, não bits/s. Alteração de interfaces ou recuo de contadores reinicia a taxa.
- Uptime: `/proc/uptime`. Reinício do servidor, pausa longa ou contadores incoerentes reiniciam CPU/rede. A primeira amostra mostra “—” nessas taxas, não zero.

Referência: [documentação oficial de /proc](https://www.kernel.org/doc/html/v6.12/filesystems/proc.html). O parser e os cálculos ficam em `internal/observability`, sem dependência gráfica ou SSH.

## Limites e ciclo de vida

Uma coleta a cada 5 s somente quando a aba está visível, conectada e com painel aberto. Backend recusa sobreposição e início em menos de 4 s por sessão. Cada operação tem orçamento de 3 s e até 128 KiB de stdout. Stderr não é retido. Histórico em memória do painel: até 61 pontos, janela de 5 min. Nenhum histórico em disco ou telemetria. Fechar painel apaga seu histórico; aba oculta preserva somente a janela e pausa a coleta.

UI consulta por RPC; não há stream/eventos ou fila de trabalhos. Fechar painel, ocultar aba ou encerrar sessão cancela a coleta. Gráficos têm lacunas em falhas/pausas; valores atuais ficam indisponíveis, com mensagem explícita. Pontos oferecem hora/valor ao passar o mouse.

A API SSH não permite cancelar uma abertura de canal sem fechar o transporte. Para não derrubar o terminal por uma falha de métricas, um pedido de abertura bloqueado retém **um único worker** por sessão até o servidor responder ou a sessão SSH encerrar. A chamada da UI retorna no timeout e novas coletas são recusadas enquanto esse worker existir. Uma abertura tardia é fechada sem executar o comando. Não há goroutines adicionais a cada tentativa. Depois de abrir o canal, o cancelamento fecha apenas esse canal; o transporte continua compartilhado com terminal/SFTP, com os limites originais preservados.

## Validação

Testes de parsing/cálculo incluem primeira amostra, reset, pausa longa, dados incompletos, NaN, overflow e resposta excessiva. Fixture SSH em loopback testa resposta válida/inválida/excessiva, bloqueio de exec/abertura, timeout, cancelamento, recusa de concorrência, encerramento e eco do terminal durante/depois da coleta; SFTP também é exercitado durante coleta travada. Execução local Linux verifica o comando fixo em `/proc` real, sem acessar hosts do usuário.

Testes de interface usam dados simulados declarados e cobrem atualização, falha, pausa ao ocultar/recolher e layout. Builds Linux/Windows e testes do transporte nas duas plataformas não substituem ensaio nativo com servidores reais, DPI ou medição de latência sob carga prolongada. A lista de processos, SO/distribuição detalhados, alertas e histórico persistente ficam fora deste incremento.
