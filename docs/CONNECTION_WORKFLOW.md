# Perfis, grupos e RTT opcional

O cadastro/edição define o usuário e o método de autenticação:

- **Pedir senha sempre**: padrão para perfis antigos; solicita senha a cada conexão.
- **Credencial do cofre**: ao abrir o host, tenta conectar com a associação específica ou a chave do grupo. Solicita desbloqueio e, quando habilitado, 2FA. Não contorna a verificação de identidade. Ausência de credencial é um erro explícito, sem fallback silencioso.
- **Chave privada**: salva somente o caminho local; a passphrase permanece efêmera. Chaves geradas dentro do cofre usam a opção de cofre.

O seletor na aba permite uma alteração temporária. Editar o perfil aplica-se a novas abas; abas existentes mantêm sua cópia. Perfis nunca contêm senha, passphrase ou material privado. Campos novos são opcionais no formato v1; versões antigas do aplicativo podem recusar perfis com esses campos e preservam o arquivo.

Para conectar um grupo, use o botão direito no nome do grupo (ou Shift+F10 com foco no grupo). O menu mantém as verificações de cobertura completa de credenciais e de capacidade para 16 sessões. Clique esquerdo continua expandindo/recolhendo.

## RTT informativo

Enquanto o aplicativo está aberto, o backend mede ICMP a cada 10 segundos. A interface consulta apenas o cache. Apenas respostas válidas mostram um texto discreto em milissegundos; falha, timeout, bloqueio de ICMP ou permissão indisponível removem o texto. Isso não indica disponibilidade nem altera o estado SSH.

O monitor deduplica hosts, admite até 200 destinos e 32 medições concorrentes, com orçamento de 1 segundo por destino incluindo DNS. Ciclos não se sobrepõem. Samples expiram após 20 segundos; remoções são refletidas no próximo ciclo. Para nomes com múltiplos endereços usa somente um, preferindo IPv4. RTT exclui tempo de DNS. Não há armazenamento histórico, telemetria ou comandos remotos.

Linux usa sockets ICMP datagram sem raw sockets; depende da política `ping_group_range` existente. Windows usa `IcmpSendEcho` / `Icmp6SendEcho2` do sistema, sem CGO. Não solicita elevação nem altera políticas do sistema. Não há fallback de varredura UDP ou tentativa em portas SSH. Fechar o aplicativo cancela o monitor; no Windows a chamada síncrona termina dentro do timeout.

## Validação

Testes automatizados cobrem deduplicação, expiração, remoção após falha, limites/cancelamento, eco IPv4/IPv6 em loopback, persistência/validação de preferências, menu com teclado, RTT opcional e abertura de perfil com chave/cofre. Testes Chromium usam bridge simulado e não constituem ensaio de GUI nativa. A política de ICMP pode impedir o recurso em determinados ambientes; nesses casos a ausência do rótulo é esperada. Validação manual em Windows com conta comum permanece necessária.
