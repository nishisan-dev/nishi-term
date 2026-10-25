# Área de trabalho — etapa 3A

Implementada em 24/10/2026 após aprovação do usuário: estrutura visual, perfis/grupos e quatro abas. Mantidos Go, Wails v2, React, xterm.js e motor SSH independente.

## Como usar

1. Compile com `./scripts/build-windows.sh` no Linux e copie `build/bin/NishiTerm.exe` para o Windows com WebView2 disponível.
2. Prepare um arquivo known_hosts com identidades verificadas por canal confiável.
3. Use o botão + de Conexões para cadastrar nome, grupo, host, porta, usuário e caminho completo do known_hosts. O grupo é opcional; grupos são derivados dos perfis, sem cadastro separado.
4. Selecione a conexão na árvore ou use Ctrl+K para buscar por nome, grupo, host ou usuário. Abrir aba não inicia a conexão.
5. Escolha senha ou chave SSH na aba. Para chave, informe caminho local e passphrase se houver. Segredos não são salvos; cada nova conexão exige autenticação.
6. Alterne entre até quatro abas. Abas ocultas pausam leitura; o indicador de pausa informa que o estado será atualizado ao selecionar. O comando remoto pode bloquear ao preencher os buffers de saída.
7. Desconecte ou feche a aba. Fechar durante conexão/sessão exige confirmação. Abas encerradas ocupam um dos quatro lugares até serem fechadas.

Editar ou excluir perfil não modifica abas já abertas: cada uma preserva os metadados usados ao abrir. Não há reconexão automática nem restauração de sessões ao reiniciar.

## Dados locais

Windows: `%APPDATA%\NishiTerm\profiles.json`. Linux: diretório retornado por `os.UserConfigDir()`, subpasta `NishiTerm`. JSON versão 1, limite de 200 perfis e 2 MiB. Contém somente ID, nome, grupo, host, porta, usuário e caminho known_hosts. Esses metadados não são criptografados. Senhas, saída de terminal e chaves não são persistidas.

Arquivos inválidos/incompatíveis são preservados e a interface apresenta erro. Corrija ou restaure uma cópia válida antes de tentar novamente. A escrita usa temporário e substituição; ainda falta validar falhas de energia e ACLs no Windows. Edições simultâneas em várias instâncias não têm bloqueio entre processos.

## Validação desta entrega

- Testes Go com race detector: motor e armazenamento passaram.
- Go vet do código interno e do alvo Windows: passaram.
- Nove testes de browser passaram: Unicode fragmentado/ACK, Start tardio, pausa/retomada de abas, entrada isolada, CRUD/busca/cópia de perfil, limite de abas/teclado, falha de autenticação, ausência de bridge, erro de gravação com nova tentativa e colagem excessiva.
- Build frontend e compilação Windows pelo Linux passaram.
- [Capturas verificadas](evidence/product-ui/README.md): browser com bridge simulado, sem conexão SSH real.

As evidências Windows anteriores validam a PoC anterior e o motor. A nova UI ainda precisa de ensaio nativo com persistência, múltiplas abas, DPI/teclado, suspensão/retomada e carga com abas ocultas. O script `scripts/windows-lab/live-test.mjs` foi preservado como histórico da UI antiga; atualizar preparação de perfis e seletores antes de reutilizá-lo.

## Próximo recorte

Consolidar o uso diário desta área de trabalho e validar a nova integração Windows antes de distribuição. Projetar autenticação por chave/agent e proteção de credenciais como etapa 3B própria. SFTP, métricas e cofre ainda não foram implementados; não há dados fictícios desses recursos na interface do produto.
