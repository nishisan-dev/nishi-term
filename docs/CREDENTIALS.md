# Credenciais e cofre autenticado — versão inicial

## Uso

Sem cofre, senha e passphrase são temporárias. Para salvar, crie o cofre na barra lateral com uma **senha mestra própria**, forte e exclusiva (mínimo 12 caracteres na interface). Ela não é a senha do Windows/Linux; não exige administrador. Confirme a senha na criação. Se a esquecer, não existe recuperação das credenciais criptografadas.

Na aba, escolha senha ou chave SSH, preencha e clique explicitamente em **Salvar credencial no cofre**. Depois selecione **Usar credencial do cofre** e conecte. O conteúdo do segredo não retorna ao React. A referência é o ID do perfil; destino, porta e usuário são conferidos antes de usar. Alterar o destino exige salvar nova credencial; abas com cópia antiga do perfil precisam ser reabertas. Excluir perfil não apaga silenciosamente o segredo: remova a credencial pela aba antes de excluir o perfil.

Bloqueie na barra lateral com **Bloquear agora**. O cofre também bloqueia após 5 minutos sem salvar/usar/remover credenciais e ao reiniciar/encerrar o aplicativo. Consultar status não renova o prazo. Bloqueio impede novas solicitações de credenciais; sessões/tentativas que já receberam credenciais continuam com seu ciclo de vida. O prazo é conferido pelo relógio de parede antes de cada uso, inclusive após suspensão; a limpeza física em memória pode ocorrer somente após o processo retomar. Mudança manual do relógio é risco residual.

## Formato e criptografia

`os.UserConfigDir()/NishiTerm/vault.json` contém envelope JSON versão 1 com sal aleatório de 16 bytes e ciphertext. Derivação: Argon2id, três iterações, 64 MiB, paralelismo 1 e chave de 32 bytes. Parâmetros fixos por versão, não controláveis pelo arquivo. Criptografia: AES-256-GCM via `cipher.NewGCMWithRandomNonce`; nonce de 96 bits aleatório gerado/prependido pela biblioteca e dados associados identificando formato/KDF. Até 200 credenciais/2 MiB, métodos senha ou chave com caminho e passphrase. O arquivo de chave privada não é copiado para o cofre; somente seu caminho e eventual passphrase.

Chave derivada e credenciais ficam somente na memória do backend enquanto desbloqueado. Nenhum log de conteúdo ou persistência da senha mestra. Limpeza de referências/buffers reduz retenção, mas Go/JS não garantem zeroização completa. Tentativas de desbloqueio são serializadas e têm intervalo mínimo de um segundo por processo; isso não impede ataque offline contra uma cópia do arquivo.

A escrita usa temporário criptografado, Sync, Close e Rename; nunca há temporário em texto claro. `.lock` exclui escritores concorrentes, e o hash da versão lida impede sobrescrever alterações de outra instância. Falhas preservam o cofre anterior; corrupção/adulteração não cria cofre vazio. Locks residuais após queda exigem recuperação manual com todos os processos fechados. POSIX cria pasta 0700/arquivo 0600; Windows usa diretório do usuário e ACLs herdadas. Backups devem copiar o envelope criptografado com a aplicação fechada. Não existe rotação de senha mestra, migração ou recuperação automática nesta versão.

## Chave SSH

A leitura e assinatura ficam no backend. Formatos suportados por `golang.org/x/crypto/ssh`, incluindo OpenSSH Ed25519 aberta/criptografada; sem PuTTY PPK, agent ou FIDO neste fluxo. Arquivo regular de até 1 MiB; OpenSSH bcrypt limitado a 1–64 rounds. Arquivos acima do limite são recusados; não reduzir a proteção de uma chave existente apenas para contornar o limite. Usar caminhos locais; I/O de rede pode bloquear. Cancelar durante parsing impede a conexão posterior, mas o parser não é interrompível.

A identidade do servidor é conferida antes de enviar autenticação; ver [HOST_TRUST.md](HOST_TRUST.md).

## Alcance e validação

Implementação inicial, ainda sem revisão criptográfica independente ou ensaio visual completo Windows/Linux. Não protege contra malware que controla a sessão do usuário, captura de teclado, administrador comprometido, XSS no bridge ou memória de processo desbloqueado. Windows Hello/PIN e integração com keyring não foram implementados; DPAPI sozinho não substituiria a autenticação explícita exigida.

Testes automatizados cobrem criptografia sem canários em texto claro, senha incorreta, adulteração, reinício bloqueado, expiração, exclusão, escritor concorrente, arquivo inválido preservado, vinculação ao destino e fluxo de interface. Permanecem gates de distribuição: suspensão real, queda de energia/recuperação, ACLs Windows, auditoria do bridge e do desenho criptográfico.

Referências: [Argon2id em x/crypto](https://pkg.go.dev/golang.org/x/crypto/argon2) e [GCM com nonce aleatório](https://pkg.go.dev/crypto/cipher#NewGCMWithRandomNonce).

## Desbloqueio ao conectar

Ao escolher autenticação pelo cofre e conectar, a interface verifica seu estado. Se trancado, pede a senha mestra no próprio fluxo e continua a tentativa somente após desbloquear. Cancelar não inicia SSH; senha incorreta mantém o diálogo, limpa o campo e permite nova tentativa. Não cria cofre nem salva credenciais automaticamente. Uma corrida com bloqueio posterior ainda é recusada pelo backend.

## Chaves gerenciadas e grupos

Gerador Ed25519 protegido pelo cofre, chave privada somente no backend, associação por host ou grupo com prioridade do host e instalação pública confirmada em sessão autenticada. Conectar grupo exige cobertura completa e capacidade disponível. Limites, compatibilidade do cofre e validação: [MANAGED_KEYS.md](MANAGED_KEYS.md).
