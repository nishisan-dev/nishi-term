# 2FA opcional do cofre

## Ativar

Abra o cofre e escolha **Configurar 2FA**. Confirme a senha mestra, escaneie o QR code no Google Authenticator ou outro aplicativo TOTP, e informe um código para ativar. Há chave manual para cadastro sem câmera. A configuração expira em cinco minutos e é descartada ao cancelar ou bloquear o cofre. Nada é ativado apenas por gerar o QR.

Após confirmar, o aplicativo mostra **dez códigos de recuperação**, somente uma vez. Guarde-os fora do cofre. Cada código permite um desbloqueio junto da senha mestra. A confirmação de ativação consome o código TOTP: aguarde o próximo código do autenticador antes de desbloquear novamente.

## Usar e recuperar

O código é pedido no desbloqueio do cofre, na conexão com credencial salva e no acesso ao gerenciador de chaves/grupos. Um cofre já desbloqueado continua autorizando operações até bloquear; o 2FA não muda a autenticação dos servidores SSH nem encerra sessões abertas.

Para desativar, confirme novamente a senha mestra e um código novo ou um código de recuperação. Se perdeu o celular, desbloqueie com recuperação e, em até cinco minutos naquela mesma sessão, repita esse código na desativação. Isso permite remover o autenticador mesmo após consumir o último código; o código usado nunca autoriza outro desbloqueio. O prazo de recuperação não é estendido por outras operações. Depois de desativar, configure novamente para obter nova semente e novos códigos.

Sem o autenticador e sem códigos de recuperação não há bypass pelo aplicativo. Códigos de recuperação não substituem uma senha mestra esquecida. Mantenha o relógio do computador e do celular corretos; é aceita uma janela de um intervalo anterior e um posterior.

## Protocolo e limites de segurança

TOTP conforme [RFC 6238](https://www.rfc-editor.org/rfc/rfc6238), HMAC-SHA-1, seis dígitos, período de 30 segundos e semente aleatória de 160 bits. Cadastro usa o [formato otpauth documentado pelo Google Authenticator](https://github.com/google/google-authenticator/wiki/Key-Uri-Format). QR gerado localmente com `qrcode` 1.5.4, sem serviço externo.

**É um segundo fator de autorização dentro do aplicativo, não um segundo fator criptográfico independente para o arquivo.** A semente TOTP fica cifrada junto das credenciais com a chave derivada da senha mestra. Quem obtiver o arquivo e descobrir a senha mestra pode decifrá-lo por ferramentas próprias, contornando o aplicativo. TOTP não protege contra malware, alteração do programa ou controle total da conta local. Proteção offline independente exigiria outra arquitetura, por exemplo hardware ou serviço externo, e não é prometida nesta entrega.

A semente só é entregue à UI durante o cadastro; não há API para recuperá-la depois. Não gravar QR, semente, senha ou códigos de recuperação em logs, localStorage, telemetria ou capturas reais. A captura pública de teste usa uma semente fictícia conhecida e não deve ser cadastrada para uso real.

Códigos de recuperação têm 128 bits aleatórios cada; somente hashes SHA-256 com separação de domínio são persistidos. Contador de último TOTP usado e consumo de recuperação são salvos atomicamente antes de liberar o cofre. Se essa gravação falha, o desbloqueio falha fechado. Há limitação de tentativas de um segundo por processo; não é defesa contra acesso offline ou reinício do processo. Restaurar cópia antiga do cofre também restaura o estado de códigos usados daquela cópia; não há servidor externo que detecte rollback.

## Compatibilidade

Cofres sem 2FA continuam no formato v1. Ativar usa envelope v2 com conteúdo cifrado e dados autenticados distintos, impedindo remover a exigência simplesmente alterando a versão do cabeçalho. Versões antigas do aplicativo recusam v2 sem sobrescrever. Desativar volta ao envelope v1, preservando credenciais e associações; versões sem suporte a chaves gerenciadas ainda podem ser incompatíveis com essas entradas.

O desbloqueio 2FA precisa escrever no cofre para registrar o uso do código. Arquivo somente leitura, bloqueio concorrente ou mudança externa impedem o desbloqueio. Permissões e política de armazenamento existentes continuam, sem exigir administrador.

## Validação

Testes cobrem vetores SHA-1 do RFC 6238, tolerância de relógio, expiração, replay, cancelamento de cadastro, token incorreto, senha incorreta, recuperação persistida após reinício, falha ao persistir consumo, remoção autenticada, preservação das credenciais e tentativa de downgrade v2 → v1. A suíte Chromium cobre QR local, ativação, limpeza dos campos, recuperação, remoção e os diálogos de conexão/grupos. Validação manual com aplicativo autenticador em celular e UI nativa permanece pendente; testes automatizados não substituem esse ensaio.
