# Escolta Marítima Autônoma - Infraestrutura Blockchain com Drones

Sistema distribuído e imutável para coordenação de missões de escolta e monitoramento marítimo via drones autônomos. A aplicação utiliza **CometBFT** como motor de consenso (Byzantine Fault Tolerance) e a interface **ABCI** escrita em Go para gerenciar um *Ledger* criptográfico, validar assinaturas digitais (ED25519) e assegurar a transparência de créditos e laudos de missões.

## Estrutura de Diretórios

```
.
├── docker-compose.yml
├── README.md
├── broker/
│   ├── Dockerfile
│   ├── go.mod
│   ├── go.sum
│   ├── main.go          # Bootstrap: sobe servidor ABCI (26658), API HTTP (8080) e TCP (1053)
│   ├── abci.go           # Aplicação ABCI: CheckTx (validação) e FinalizeBlock (execução)
│   ├── ledger.go         # Histórico imutável de créditos e derivação de saldo
│   ├── api.go            # Endpoints de transparência: /saldos, /extrato, /missoes, /auditoria
│   ├── despacho.go       # Reserva otimista local + submissão de BlocoDespacho ao consenso
│   ├── drone.go          # Registro, heartbeat, ACEITE/CONCLUSAO e emissão do laudo
│   ├── sensor.go         # Recepção de requisições e submissão de BlocoTransacao assinado
│   ├── requisicao.go     # Struct Requisicao, fila de prioridade (heap) e aging
│   └── protocolo.go      # Struct Mensagem e parser do protocolo TCP
├── drone/
│   ├── Dockerfile
│   ├── go.mod
│   └── main.go           # Conecta ao broker, heartbeat, executa missão e gera rota GPS
├── sensor-manual/
│   ├── Dockerfile
│   ├── go.mod
│   └── main.go           # Terminal interativo: registro, missão e transferência de créditos
├── teste/
│   ├── hacker.go         # Tenta forjar transferência de créditos (teste de fraude)
│   └── hackerL.go        # Tenta injetar laudo falso de drone fantasma (teste de fraude)
└── minha-rede/
    ├── node0/config/     # Configuração e chaves do nó CometBFT 0 (genesis, validador)
    ├── node1/config/     # Configuração e chaves do nó CometBFT 1
    ├── node2/config/     # Configuração e chaves do nó CometBFT 2
    └── node3/config/     # Configuração e chaves do nó CometBFT 3
```
## Pacotes e Dependências

A aplicação de estado (Broker/ABCI) expande a biblioteca padrão do Go incorporando pacotes de consenso e criptografia:

| Pacote | Uso |
|--------|-----|
| `net` / `http` | Sockets TCP e chamadas REST à RPC do CometBFT |
| `github.com/cometbft/...` | Interface ABCI (`abcitypes`) e servidor de sockets do Comet |
| `crypto/ed25519` | Geração de chaves e verificação de assinaturas em transações |
| `sync` | Mutex (`RWMutex`) para sincronização local (filas e mapas) |
| `container/heap` | Fila de prioridade de missões por criticidade |
| `encoding/json` / `hex` | Serialização de transações para o mempool |
| `math/rand/v2` | Geração de coordenadas aleatórias em edge (Drone) |

---

## Protocolo de Comunicação

O sistema opera em duas camadas de protocolo. A camada **TCP Interna** (Broker ↔ Clientes) e a **Camada de Consenso** (Broker ↔ CometBFT).

### 1. Mensagens TCP (Clientes)
Formato base: `TIPO;ID;ACAO;PAYLOAD\n`

* **Sensor → Broker:** `SENSOR;Navio_A;bloqueio_rota;alta`
* **Drone → Broker:** `DRONE;drone-1;ACEITE;`
* **Broker → Drone:** `BROKER;broker-1;MISSAO;Navio_A-171543200`

### 2. Blocos de Consenso (JSON via RPC CometBFT)
As transações trafegam empacotadas na struct `PacoteBase`. Tipos mapeados no `abci.go`:

* **`REGISTRO`**: Cadastra chaves públicas e inicializa saldo (Gênese).
* **`TRANSACAO`**: Solicitação de missão assinada, debita créditos da empresa.
* **`TRANSFERENCIA`**: Envio de fundos P2P assinado pela chave da empresa origem.
* **`LAUDO`**: Finalização de missão gerada e assinada pelas chaves do Drone.
* **`DESPACHO` / `LIBERACAO`**: Controle interno distribuído de disponibilidade de frota.

---

## Como Executar

Utilize as diretrizes abaixo para configurar a rede Docker interna e iniciar a malha do consenso e os Brokers.

### 1. Criar a Rede Docker
O ecossistema depende de uma rede estática fechada para o P2P da blockchain:
```bash
docker network create pbl-net
```

### 2. Subir o Nó do CometBFT (Blockchain)
Inicia o nó da blockchain conectado à interface ABCI do broker, com mapeamento de peers para P2P em laboratório:
```bash
docker run --name node0 --user "$(id -u):$(id -g)" --network pbl-net \
  -p 26657:26657 -p 26656:26656 -v ./minha-rede/node0:/cometbft \
  cometbft/cometbft:v0.38.x node \
  --proxy_app=tcp://broker-app-1:26658 \
  --rpc.laddr=tcp://0.0.0.0:26657 \
  --p2p.laddr=tcp://0.0.0.0:26656 \
  --p2p.persistent_peers="05bae741b457ad946138ec071bc29a1eed8c3ddf@172.16.201.2:26656,dd6a278b150e6287027d6597ed424f0b371b301f@172.16.201.4:26656,04e4459859e8a0ed42327557eeb3d61613b048a4@172.16.201.1:26656,94e47f538fb70bcdbfcc27e33718eb3448aa721a@172.16.201.5:26656"
```

### 3. Subir o Broker (App ABCI + Servidor TCP)
Substitua `SEU_USUARIO` pela sua imagem no Docker Hub:
```bash
docker run --name broker-app-1 --network pbl-net \
  -p 1053:1053 -p 8080:8080 \
  -e COMET_URL=node0:26657 \
  allanyvictoria/brokert3:v1
```

### 4. Verificar ID do Persistent Peer (Auditoria de Nó)
Se necessário para inclusão na rede estendida:
```bash
docker run --rm --user "$(id -u):$(id -g)" -v ./minha-rede/node0:/cometbft \
  cometbft/cometbft:v0.38.x show-node-id
```

---

## Como Usar

### API de Transparência (Porta 8080)
O Broker provê rotas HTTP para auditoria dos dados aprovados em consenso:
* `GET /saldos` — Snapshot do cache de saldos antigos/atuais de todos os navios.
* `GET /extrato` — Exporta o `Ledger` completo (todo o histórico de débitos e transferências).
* `GET /saldos/recalcular` — Rederiva todos os saldos somando o Ledger do zero, provando a consistência dos fundos.
* `GET /auditoria` — Proxy direto para acesso granular ao último bloco consolidado pelo CometBFT.

### Terminal do Sensor (Navio)
O terminal interativo roda no terminal local. Ele emite um par de chaves ED25519 randômico temporário por sessão:
```text
Chave criptográfica gerada para esta sessão.
Digite o nome da sua Companhia (ex: Navio_A): Navio_A

--- TERMINAL DO SENSOR: Navio_A ---
1. Emitir Créditos Iniciais
2. Solicitar Missão
3. Transferir Créditos para outra Empresa
4. Sair
```

Ao solicitar missão, a aplicação assina o pedido matematicamente e submete o bloco `TRANSACAO` ao ABCI. Se aprovada pelo `CheckTx` (saldo verificado e assinatura válida), entra no mempool.

### Drones
Os drones rodam autonomamente. Ao conectar e entrar na rede do broker, emitem laudos contendo criptografia validada atestando a conclusão da escolta de forma inalterável no Ledger.

---

## Arquitetura & Fluxo BFT

```text
  [Navio_A (Sensor)] ──────── (1) Assina Tx (ED25519) ───────┐
                                                             ▼
                                                ┌────────────────────────┐
  [API HTTP:8080] ◀── (4) Deriva Ledger ────────┤        Broker          │
    (Saldos/Extrato)                            │ (ABCI App + TCP 1053)  │
                                                └────┬───────────────▲───┘
                                                     │               │
                                              (2) Proxy App       (3) FinalizeBlock
                                                 CheckTx             │
                                                     ▼               │
                                                ┌────────────────────────┐
  [Drone Edge GPS] ── (5) Assina Laudo ────────▶│  Nó CometBFT (26657)   │
                                                │  (Consenso P2P, Rede)  │
                                                └────────────────────────┘
```

## Auditoria e Testes de Segurança (Pasta `teste`)

A pasta `teste` contém scripts autônomos desenvolvidos para simular ataques diretos à infraestrutura de consenso. Estes testes validam a robustez do `CheckTx` na interface ABCI, garantindo que o sistema rejeita transações baseadas em chaves criptográficas divergentes, forjadas ou não homologadas.

### 1. Teste de Fraude Financeira (`hacker.go`)
Simula uma tentativa de roubo de créditos. O script forja uma requisição de transferência de fundos utilizando o nome de uma empresa legítima (vítima), porém assina matematicamente o pacote utilizando uma chave privada gerada aleatoriamente pelo atacante.
* **Objetivo:** Verificar se a rede recusa a transação ao confrontar a assinatura com a `PublicKey` original vinculada à empresa.
* **Como executar:**
  ```bash
  go run teste/hacker.go
  ```

### 2. Teste de Injeção de Laudo Falso (`hackerL.go`)
Simula um ataque cibernético à infraestrutura de drones. O script tenta submeter um bloco tipo `LAUDO` (conclusão de missão) forjando a identidade de um drone inexistente ou não homologado, gerando dados de rota e status de poluição corrompidos.
* **Objetivo:** Assegurar que o sistema processe unicamente laudos emitidos por drones cujas chaves públicas e identidades foram validadas e registradas na memória do Broker.
* **Como executar:**
  ```bash
  go run teste/hackerL.go
  ```
1. **Assinatura:** O Sensor/Navio gera uma transação, anexa o timestamp e assina com a chave privada.
2. **CheckTx:** A interface ABCI atua como porteira do mempool. Verifica saldos na memória, valida a `PublicKey` no dicionário da rede e confere o *hash* da assinatura matemática.
3. **Consenso & FinalizeBlock:** O CometBFT sincroniza com os peers. Ao gerar o bloco, o `FinalizeBlock` aplica o débito no `Ledger`, insere na fila e aciona despachos.
4. **Resiliência de Rede:** Submissões assíncronas do Broker de liberação de drones contam com tolerância a *rollbacks* na fila local (via goroutines) se o endpoint HTTP do CometBFT demorar a responder.
