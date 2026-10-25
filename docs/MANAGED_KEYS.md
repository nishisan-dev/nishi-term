# Chaves gerenciadas e conexões por grupo

Implementação autorizada em 25/10/2026. O botão **Chaves SSH** na lateral abre o gerenciador protegido pelo desbloqueio explícito do cofre. Não requer administrador local.

## Gerar e associar

1. Crie/desbloqueie o cofre, dê um nome à chave e escolha **Gerar Ed25519**.
2. A chave privada OpenSSH fica dentro do arquivo criptografado do cofre. Não é gravada em arquivo PEM temporário, enviada ao renderer ou retornada pela API. A tela apresenta somente nome, chave pública e fingerprint SHA-256.
3. Associe a um host ou grupo, conferindo o destino e a confirmação. Associar ao host substitui sua credencial específica, inclusive senha previamente salva. Associar ao grupo define um padrão para membros atuais e futuros **sem credencial própria**.
4. Credencial específica do host sempre tem prioridade. Se ficou inválida após alterar endereço/porta/usuário, o aplicativo recusa seu uso; não muda silenciosamente para a chave do grupo.
5. Alterar o nome/grupo dos perfis não renomeia a associação antiga de grupo. Associe a chave novamente ao novo grupo quando necessário. Remover a credencial específica do host permite que ele volte a usar o padrão do grupo.

A associação é local: não instala a chave no destino nem comprova que o servidor a aceita. Para autenticar com a associação, selecione **Credencial do cofre** no host ou use **Conectar grupo**. Ed25519 é o único algoritmo gerado nesta etapa; servidores que não o suportam continuam podendo usar os métodos anteriores.

## Instalar chave pública

1. Conecte normalmente ao servidor, por exemplo com sua senha atual, mantendo a verificação da identidade SSH.
2. Selecione a aba, abra **Chaves SSH**, escolha a chave e **Instalar chave na sessão atual**.
3. Confira usuário, endereço, porta e fingerprint e confirme. O backend valida que a sessão corresponde ao perfil atual.
4. A operação abre um canal na conexão SSH autenticada e executa um script fixo no usuário remoto. Somente a chave pública canônica é interpolada; nomes, caminhos, comentários e senhas vindos da UI não entram no comando.
5. Para servidores com shell POSIX e utilitários padrão (mkdir, chmod, wc, awk, grep), cria `.ssh` se necessário, ajusta permissões 700/600 e acrescenta em `$HOME/.ssh/authorized_keys`. Não usa sudo, não substitui o arquivo e não remove chaves existentes. Recusa links simbólicos, arquivo acima de 1 MiB e conflito com a mesma chave restrita/comentada. Uma chave pública simples já presente não é duplicada.
6. Mantém a senha/credencial atual. Depois de instalar, associe a chave ao host ou grupo e teste uma nova conexão **antes de encerrar o acesso original**. `authorized_keys` personalizado pelo sshd, contas sem shell, appliances e servidores Windows remotos não são suportados por este instalador inicial.

Há um bloqueio cooperativo por diretório remoto para instalações concorrentes, uma operação remota por sessão (compartilhada com a navegação SFTP), prazo de 15 s e saída descartada. Não há repetição automática. Se houver falha/timeout, o resultado remoto pode ser parcial: verificar antes de repetir. Interromper a sessão cancela a operação; cancelar durante abertura travada de canal pode fechar o transporte, como no SFTP. A instalação não é uma transação com a associação local, e nunca altera automaticamente o método de login.

## Conectar grupo

- O botão de cada grupo abre a conferência de cobertura. Se necessário, solicita uma vez a senha mestra.
- Habilita a ação somente quando todos os perfis do grupo têm credencial específica válida ou chave padrão aplicável. A cobertura é revalidada antes de iniciar.
- Cobertura significa existência e vínculo da credencial: não testa senha, existência de arquivo de chave externa, reachability, nem instalação da chave pública no destino.
- Ignora perfis já ativos/conectando na área de trabalho e verifica as vagas disponíveis. O lote precisa caber em 16 sessões; não cria fila ilimitada nem aumenta os buffers. O backend continua recusando excesso em caso de corrida com outras conexões; resultados/falhas aparecem por aba, sem rollback ou reconexão automática.
- Cada aba usa autenticação independente e mantém a confirmação de host desconhecido. Selecionar a aba permite responder à confirmação; não há confiança automática em grupo.
- Abas ocultas continuam com saída pausada e buffers limitados. O estado observado pode precisar de atualização ao selecionar uma aba encerrada remotamente.

## Persistência e compatibilidade

As novas entradas (identidade, associação de host e associação de grupo) são criptografadas junto às credenciais. Continuam os limites de 200 registros totais e 2 MiB do cofre, bloqueio por inatividade, detecção de alteração externa e escrita atômica. Nome de chave e associação também não são persistidos em claro. O login entrega bytes privados somente internamente ao motor; o campo é não exportado e não existe na serialização JSON.

Cofres anteriores são lidos preservando as credenciais existentes. Depois de salvar novas categorias de entrada, versões antigas do aplicativo não conseguem abri-las e falham sem sobrescrever o arquivo. Não usar um executável antigo para editar esse cofre. Exportação privada, revogação automática remota, remoção de identidades referenciadas e backup/restauração na UI são etapas futuras. Não há promessa de zeroização completa de segredos em Go/JS.

## Evidências

Testes Go com detector de concorrência: geração, persistência criptografada, bloqueio/desbloqueio, ausência de material privado no JSON, prioridade host/grupo, destino alterado, login por chave do cofre em SSH efêmero e instalação pelo canal SSH. Testes POSIX verificam preservação de chaves, idempotência, recusa de symlinks e preservação de restrições. A execução do script remoto é validada em fixture Linux; no Windows esses testes de shell local são pulados, pois o aplicativo Windows usa o shell do servidor remoto.

Testes Chromium com bridge simulado: confirmação de associação/instalação, cobertura incompleta, desbloqueio, conexão automática do grupo, prevenção de duplicatas e capacidade insuficiente. Capturas `key-manager-fixture.png` e `group-connect-fixture.png` em `docs/evidence/product-ui` são dados fictícios. Ensaio da UI nativa e instalação em OpenSSH externo permanecem pendentes. Nenhum host real do usuário foi acessado nesta implementação.
