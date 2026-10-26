# Perfis, grupos e RTT opcional

O cadastro/edição define o usuário e o método de autenticação:

- **Pedir senha sempre**: padrão para perfis antigos; solicita senha a cada conexão.
- Perfis de **cofre**: ao abrir o host, tentam conectar com a associação específica ou a chave do grupo. Solicitam desbloqueio e, quando habilitado, 2FA. Não contornam a verificação de identidade. Ausência de credencial é um erro explícito, sem fallback silencioso.
- **Chave do cofre — escolher chave**: abre o desbloqueio com senha mestra e 2FA quando habilitado, e lista as chaves geradas por nome e fingerprint. A escolha fica pendente até **Salvar conexão**. Salvar cria/substitui a associação específica do host; não altera o padrão do grupo nem instala a chave pública no servidor.
- **Arquivo de chave**: salva somente o caminho local; a passphrase permanece efêmera.
- **Credencial já associada (host ou grupo)**: conserva a resolução existente, inclusive senhas salvas e chaves herdadas do grupo. **Consultar ou trocar chave do cofre** permite conferir nome/fingerprint da chave efetiva após desbloquear. **Manter associação atual**, ou confirmar a mesma chave com o destino inalterado, preserva a herança do grupo e não grava associação específica. Se host/porta/usuário mudou, a tela exige confirmar explicitamente a associação ao novo destino, mesmo quando a chave é a mesma.

O seletor na aba permite uma alteração temporária. Editar o perfil aplica-se a novas abas; abas existentes mantêm sua cópia. Perfis nunca contêm senha, passphrase ou material privado. Campos novos são opcionais no formato v1; versões antigas do aplicativo podem recusar perfis com esses campos e preservam o arquivo.

A escolha de chave não persiste seu ID no perfil: usa a associação criptografada existente no cofre. Editar apenas os metadados mantém essa associação. Se o destino mudar, uma associação antiga continua falhando fechado até ser explicitamente substituída. O seletor informa ausência de chaves e permite nova tentativa em falhas de leitura; retornar ao cadastro antes de confirmar não muda o método.

Metadados do perfil e associação criptografada são arquivos independentes. Se a primeira gravação funcionar e a associação falhar, o editor permanece aberto e informa explicitamente que o perfil foi salvo e a chave não foi associada; a credencial anterior permanece. Tentar salvar novamente reutiliza o ID retornado, sem duplicar o perfil. O aplicativo revalida o desbloqueio antes de salvar, inclusive quando o cofre bloqueou durante a edição. Cancelar/fechar depois de uma falha parcial não desfaz o perfil já salvo.

Para conectar um grupo, use o botão direito no nome do grupo (ou Shift+F10 com foco no grupo). O menu mantém as verificações de cobertura completa de credenciais e de capacidade para 16 sessões. Clique esquerdo continua expandindo/recolhendo.

## RTT informativo

Enquanto o aplicativo está aberto, o backend mede ICMP a cada 10 segundos. A interface consulta apenas o cache. Apenas respostas válidas mostram um texto discreto em milissegundos; falha, timeout, bloqueio de ICMP ou permissão indisponível removem o texto. Isso não indica disponibilidade nem altera o estado SSH.

O monitor deduplica hosts, admite até 200 destinos e 32 medições concorrentes, com orçamento de 1 segundo por destino incluindo DNS. Ciclos não se sobrepõem. Samples expiram após 20 segundos; remoções são refletidas no próximo ciclo. Para nomes com múltiplos endereços usa somente um, preferindo IPv4. RTT exclui tempo de DNS. Não há armazenamento histórico, telemetria ou comandos remotos.

Linux usa sockets ICMP datagram sem raw sockets; depende da política `ping_group_range` existente. Windows usa `IcmpSendEcho` / `Icmp6SendEcho2` do sistema, sem CGO. Não solicita elevação nem altera políticas do sistema. Não há fallback de varredura UDP ou tentativa em portas SSH. Fechar o aplicativo cancela o monitor; no Windows a chamada síncrona termina dentro do timeout.

## Validação

Testes automatizados cobrem deduplicação, expiração, remoção após falha, limites/cancelamento, eco IPv4/IPv6 em loopback, persistência/validação de preferências, menu com teclado, RTT opcional e abertura de perfil com chave/cofre. Testes Chromium usam bridge simulado e não constituem ensaio de GUI nativa. A política de ICMP pode impedir o recurso em determinados ambientes; nesses casos a ausência do rótulo é esperada. Validação manual em Windows com conta comum permanece necessária.
