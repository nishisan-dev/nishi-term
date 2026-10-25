# Identidade do servidor sem configurar known_hosts

Novos perfis usam identidades gerenciadas pelo NishiTerm. O cadastro básico pede nome, grupo, host, porta e usuário. O arquivo known_hosts permanece opcional em **Opções avançadas de identidade**.

## Primeira conexão

O servidor apresenta sua chave durante o handshake. O aplicativo mostra destino/porta, algoritmo e impressão digital SHA-256 e aguarda **Confiar e continuar** ou **Cancelar conexão**. Confira a impressão digital por outro canal confiável: a primeira apresentação sozinha não prova autenticidade. Nenhuma senha ou assinatura de login é enviada ao servidor antes da aprovação.

A confirmação vale somente para aquela tentativa, com token aleatório ligado à sessão e à chave apresentada. A conexão fica aberta enquanto o usuário decide; não se faz um scan seguido de nova conexão. A espera expira em 120 segundos. Cancelar/fechar a aba não grava confiança. Após aprovação, a identidade é salva e o handshake continua na mesma conexão, com novo prazo de 15 segundos.

## Conexões seguintes e mudanças

A chave é comparada com a identidade salva para o host/porta. Divergência bloqueia a conexão, sem botão de ignorar/substituir. Uma mudança legítima exige verificar com o administrador antes de atualizar o registro; ainda não há interface de rotação/remoção. Aliases diferentes são identidades separadas. O gerenciamento fixa uma chave exata por destino; mudança do algoritmo negociado também pode bloquear e precisa de investigação.

Dados em `os.UserConfigDir()/NishiTerm/trusted-hosts.json`, versão 1, até 1.000 destinos/2 MiB. São chaves públicas, não segredos. Arquivo inválido é preservado e falha fechado. Gravação em temporário + Sync + Rename; exclusão entre processos por arquivo `.lock`. Se o processo morrer durante gravação, um lock residual exige recuperação manual após confirmar que nenhum NishiTerm está gravando. Nunca removê-lo automaticamente nem substituir arquivo inválido por vazio.

## Perfis existentes

Perfis com caminho externo continuam usando esse arquivo em modo estrito, sem diálogo de aceite. Para usar o novo gerenciamento, edite o perfil e deixe **Arquivo known_hosts (opcional)** vazio; feche e reabra abas já abertas para carregar a mudança. Não importamos ou modificamos automaticamente `~/.ssh/known_hosts`.
