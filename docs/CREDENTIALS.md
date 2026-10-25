# Credenciais: chave SSH e cofre autenticado

## Implementado nesta etapa

Cada conexão permite escolher senha ou arquivo de chave privada, com passphrase opcional. A leitura e a assinatura acontecem no backend Go; o conteúdo da chave não passa pelo React. O arquivo não é copiado para os perfis. Senha/passphrase são efêmeras, removidas dos campos e referências temporárias após envio. Não há fallback automático entre métodos nem uso de agent nesta etapa.

São aceitos os formatos de chave privada suportados por `golang.org/x/crypto/ssh`, incluindo OpenSSH Ed25519 com e sem passphrase. PuTTY PPK e dispositivos FIDO não são suportados por este fluxo. Arquivo regular de até 1 MiB; para OpenSSH criptografado, bcrypt de 1 a 64 rounds (padrão OpenSSH: 16). Limitar tamanho não basta para limitar o custo de uma chave malformada. Arquivos acima desse custo são recusados com erro genérico; não diminuir a proteção de uma chave existente só para contornar o limite.

A verificação known_hosts continua estrita, antes de oferecer a chave ao servidor. O prazo de 15 s de conexão começa após a leitura/decodificação local. Cancelamento durante decodificação impede a conexão posterior; o parser não é interrompível, mas o custo OpenSSH é limitado. Usar caminhos locais; arquivos em rede podem bloquear I/O. Go/JS não garantem zeroização completa da memória gerenciada.

## Requisito confirmado do cofre

O usuário exige um cofre de senhas que peça autenticação para desbloquear e funcione sem privilégios administrativos. O cofre ainda não está implementado. Autenticação por chave não substitui esse requisito.

Desenho proposto: cofre local criptografado e bloqueado ao iniciar, desbloqueado por senha mestra explícita. Esse caminho é independente de admin e pode funcionar em Windows/Linux. Integração futura com Windows Hello/PIN pode ser alternativa após validar APIs, disponibilidade e consentimento em usuário comum; DPAPI/Credential Manager isoladamente não satisfazem o requisito de nova autenticação.

Antes de implementar armazenamento de segredos:

- Definir formato versionado, derivação de chave resistente a ataques offline (Argon2id com parâmetros limitados/medidos) e criptografia autenticada com sal/nonces aleatórios. Rever desenho e migração antes de gravar dados reais.
- Separar o serviço de cofre do motor SSH. Perfis guardam somente referência opaca; backend obtém o segredo, sem API de listar segredos em texto claro para o renderizador.
- Desbloqueio explícito, bloqueio manual e automático após inatividade; novo bloqueio ao reiniciar. Bloquear impede novas autenticações, sem derrubar sessões SSH já estabelecidas. Definir comportamento em suspensão/retomada.
- Não salvar senha mestra nem chave de desbloqueio. Sem recuperação implícita: perda da senha mestra pode tornar o cofre inacessível; explicar isso antes da criação.
- Gravação transacional, backup/recuperação, limites de tamanho/KDF, tratamento de corrupção e uso simultâneo. Nunca substituir o cofre por um vazio após erro.

## Ameaças e aceitação do cofre

Proteger contra leitura do arquivo em repouso e uso do app bloqueado. Não prometer proteção contra malware com controle da sessão do usuário, captura de teclado, administrador comprometido ou memória de processo desbloqueado. A CSP e o bridge precisam de revisão porque XSS pode abusar de credenciais disponíveis no backend.

Testar: senha mestra correta/incorreta, adulteração/truncamento, migração, reinício bloqueado, limite de tentativas e custo KDF, inatividade/suspensão, cancelamento concorrente com conexão, falha de gravação sem perda, ausência de canários em arquivos/logs/bridge e funcionamento real como usuário comum Windows/Linux. Não marcar o cofre como protegido na interface até esses critérios passarem.
