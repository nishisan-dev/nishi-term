# Instruções permanentes — NishiTerm

- Leia docs/SPEC.md, docs/ARCHITECTURE.md, docs/ROADMAP.md, docs/RISKS_AND_VALIDATION.md e docs/UI_REFERENCE.md antes de mudar comportamento. Documentos em português; código pode usar nomes em inglês.
- Stack obrigatória: Go, Wails v2 estável, React, xterm.js. Não migrar para Wails v3/beta, Electron ou shell externo sem decisão explícita do usuário.
- Windows deve funcionar como usuário comum. Compilação Linux → Windows é requisito; não introduza dependência nativa Windows/CGO sem provar esse caminho.
- Motor SSH em internal/sshengine, sem Wails/React; UI é adaptador. Sessões possuem contexto, dono e fechamento idempotente. Nunca reconectar/reexecutar comandos automaticamente.
- Toda fila deve ter limite em bytes/itens e política explícita. ACK de saída só após callback do xterm. Não usar eventos sem backpressure, buffers ilimitados ou goroutine por tecla.
- Verifique host keys estritamente. Nunca use InsecureIgnoreHostKey, aceite automático ou fallback para algoritmos inseguros. Testes usam chaves locais efêmeras.
- Não persistir/logar senha, chave, passphrase, terminal ou DTO de autenticação. Não usar localStorage para segredos. Não prometer zeroização em Go/JS.
- Não transformar conteúdo remoto em HTML nem habilitar clipboard/links automáticos. Preserve bytes de terminal e UTF-8 fragmentado.
- Para mudanças no motor: go test -race ./internal/..., go vet ./internal/... e GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go vet ./... (após gerar frontend), testes reais do handshake local e casos de cancelamento/backpressure. Para frontend: npm ci, npm run build e npm test (Playwright). Para integração: compilar Windows pelo script. Registrar limitações do ambiente, sem apresentar testes planejados como executados.
- Sem acesso automático a hosts reais ou uso de credenciais do usuário em testes. Use servidor efêmero em loopback.
- Dependências com versões estáveis exatas e lockfiles; upgrades revisados. Não commitar binários, node_modules, credenciais ou known_hosts do usuário.
- Escopo inicial limitado à documentação e PoC SSH. O mockup está em docs/references/nishiterm-mockup.jpg, com análise em docs/UI_REFERENCE.md; a conversa ChatGPT permanece incompleta. Usar a imagem como direção visual, respeitar as fases e não apresentar capacidades ou dados simulados como reais.
- A fase 2 de validação foi autorizada: teste do executável normal no Windows, carga/falhas/cancelamento/100 ciclos e isolamento de até quatro sessões no motor. Esse gate foi seguido pela autorização da etapa de produto abaixo. Testes `TestLab*` são opt-in, usam somente a fixture OpenSSH descartável e configuração privada indicada por `NISHITERM_LAB_CONFIG`; nunca usar hosts de produção. Separar evidências do motor, da interface normal e da interface instrumentada.

- Etapa de produto 1–3 autorizada: estrutura visual do mockup, perfis/grupos com CRUD e busca Ctrl+K, até quatro abas independentes. Cofre, chaves/agent, SFTP e métricas continuam fora desta entrega.
- Persistir somente DTO de metadados em internal/profiles; nunca reutilizar DTO de autenticação. Arquivos inválidos devem ser preservados; erros permitem nova tentativa.
- Cada aba tem TerminalSession próprio e cópia dos metadados ao abrir. Editar/excluir perfil não altera sessões abertas. Confirmar fechamento durante conexão/sessão; nunca reconectar automaticamente.
- Aba oculta pausa Poll/processamento, retendo no máximo um frame recebido. Status é o último observado. Preservar ACK após callback, limites e cancelamento ao alternar/fechar abas.
- Capturas em docs/evidence/product-ui usam bridge simulado, não SSH real nem Windows. A nova UI requer ensaio nativo próprio antes de release.
- Autorização seguinte: CI GitHub, build Windows/Linux e autenticação por chave SSH com passphrase. Cofre é requisito de produto: desbloqueio com autenticação explícita do usuário sem admin; proteção automática da conta do sistema não equivale a desbloqueio autenticado. Não salvar segredos até implementar e validar esse fluxo. SFTP/métricas continuam fora de escopo.

- Autorizado: identidades gerenciadas com confirmação explícita na mesma conexão, token por sessão e bloqueio de chave divergente; known_hosts externo opcional/estrito. Nunca aceitar mudança de chave automaticamente. Ver docs/HOST_TRUST.md.
- Cofre inicial autorizado: somente persistência criptografada em internal/vault, senha mestra explícita sem admin, bloqueio manual/automático e sem API de ler segredos no renderer. A regra anterior de não persistir segredos passa a permitir exclusivamente esse cofre validado; perfis/logs/storage do browser continuam sem segredos. Ver docs/CREDENTIALS.md.

- Ampliação autorizada: 16 sessões ativas/conectando, sem contar abas desconectadas ou workers finalizados. Preservar todos os limites por sessão e a saída final; cancelamento só libera capacidade após encerrar o worker. Testes de carga `TestSixteen*` em loopback e CI Linux/Windows são gate antes de disponibilizar novo build. Ver docs/SESSION_CAPACITY.md. Esta regra substitui os limites históricos de quatro abas/sessões acima.

- Autorização de 25/10/2026: avançar autonomamente em funcionalidades do mockup para revisão pela manhã. Primeiro incremento: organização de abas e SFTP somente leitura, documentado em docs/SFTP.md. Substitui a exclusão histórica de SFTP acima; transferências e métricas exigem implementação real com limites, sem controles simulados. Não usar hosts reais automaticamente.

- Autorizado em 25/10/2026: chaves Ed25519 geradas no cofre, associações por host/grupo, instalação pública explícita em sessão autenticada e conexão de grupo com cobertura completa. Material privado nunca deve atravessar o bridge ou ser escrito em PEM temporário. Host específico prevalece sobre grupo; destino alterado falha fechado. Instalação exige confirmação na UI e mantém credencial atual. Respeitar 16 sessões e host trust individual. Ver docs/MANAGED_KEYS.md.

- 2FA opcional do cofre autorizado: TOTP RFC 6238, QR local, confirmação antes de ativar e recuperação de uso único. Exigir senha mestra e fator no backend antes de liberar entradas; persistir consumo antes do desbloqueio. Não enviar semente fora do cadastro nem registrar QR/códigos. Documentar que TOTP local não é segundo segredo criptográfico offline. Formato v2 deve falhar fechado em downgrade/erro de escrita. Ver docs/VAULT_2FA.md.
