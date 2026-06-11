# Estreito de Ormuz — Infraestrutura Distribuída de Drones

Sistema distribuído para coordenação de uma frota de drones autônomos de monitoramento marítimo, desenvolvido em Go e containerizado com Docker. Brokers independentes gerenciam setores do estreito, compartilham uma frota de drones e continuam operando mesmo com falhas parciais, sem nenhum ponto central de controle.

---

## Estrutura de Diretórios

```
.
├── docker-compose.yml
├── Teste.go                        # Suite de testes TCP externos
├── broker/
│   ├── Dockerfile
│   ├── main.go                     # Inicialização do servidor TCP, goroutines de heartbeat e aging
│   ├── broker.go                   # Protocolo inter-broker: RESERVAR_E_DESPACHAR, CONCLUSAO_REMOTA
│   ├── despacho.go                 # Lógica de despacho local e remoto, heap de prioridade
│   ├── drone.go                    # Registro de drones, heartbeat, verificação de timeout
│   ├── sensor.go                   # Recepção de requisições dos sensores, inserção na fila
│   ├── requisicao.go               # Struct Requisicao, FilaRequisicoes (heap), aging de prioridade
│   ├── protocolo.go                # Struct Mensagem, ParseMensagem
│   └── go.mod
├── drone/
│   ├── Dockerfile
│   ├── main.go                     # Conecta ao broker, envia heartbeat, executa missões
│   └── go.mod
├── sensor/
│   ├── Dockerfile
│   ├── main.go                     # Gera requisições aleatórias autonomamente
│   └── go.mod
└── sensor-manual/
    ├── Dockerfile
    ├── main.go                     # Interface de terminal para injetar requisições manualmente
    └── go.mod
```

---

## Pacotes e Dependências

O projeto utiliza **apenas a biblioteca padrão do Go**, sem frameworks externos:

| Pacote | Uso |
|--------|-----|
| `net` | Sockets TCP (brokers, drones, sensores) |
| `sync` | `Mutex` para proteção de mapas e fila compartilhados |
| `container/heap` | Fila de prioridade das requisições |
| `bufio` | Leitura de mensagens linha a linha via TCP |
| `time` | Timestamps, heartbeat, aging, timeout de conexão |
| `fmt` / `log` | Saída e logging |
| `math/rand` | Geração aleatória de tipos e criticidades no sensor |
| `strings` | Parsing de mensagens e variáveis de ambiente |
| `strconv` | Conversão do intervalo de envio do sensor |
| `os` | Hostname (ID do container), variáveis de ambiente |

---

## Protocolo de Comunicação

Todas as mensagens seguem o formato:

```
TIPO;ID;ACAO;PAYLOAD\n
```

| Campo | Descrição |
|-------|-----------|
| `TIPO` | Origem da mensagem: `SENSOR`, `DRONE`, `BROKER` |
| `ID` | Identificador do remetente (hostname do container) |
| `ACAO` | Ação ou estado: `REGISTRO`, `MISSAO`, `HEARTBEAT`, `RESERVAR_E_DESPACHAR`, etc. |
| `PAYLOAD` | Dado adicional (criticidade, droneID, reqID, etc.) |

### Mensagens por componente

**Sensor → Broker** (requisição de monitoramento):
```
SENSOR;sensor-setor1-deriva;bloqueio_rota;alta
```

**Drone → Broker** (registro ao conectar):
```
DRONE;drone-setor1;REGISTRO;
```

**Drone → Broker** (heartbeat periódico, a cada 10s):
```
DRONE;drone-setor1;HEARTBEAT;
```

**Drone → Broker** (aceite e conclusão de missão):
```
DRONE;drone-setor1;ACEITE;
DRONE;drone-setor1;CONCLUSAO;
```

**Broker → Drone** (despacho de missão):
```
BROKER;broker1;MISSAO;bloqueio_rota
```

**Broker → Broker** (reserva e despacho atômico de drone remoto):
```
BROKER;broker1;RESERVAR_E_DESPACHAR;req-xyz/bloqueio_rota/broker1
→ BROKER;broker2;DESPACHO_OK;
→ BROKER;broker2;DESPACHO_NEGADO;
```

**Broker → Broker** (notificação de conclusão de missão remota):
```
BROKER;broker2;CONCLUSAO_REMOTA;req-xyz
```

---

## Como Executar

### Pré-requisitos

- [Docker](https://www.docker.com/)
- [Docker Compose](https://docs.docker.com/compose/)
- [Go 1.22+](https://golang.org/) — apenas para rodar os testes externos (`Teste.go`)

### Opção A — Tudo em uma máquina (docker-compose)

Constrói as imagens e sobe brokers, drones e sensores automaticamente:

```bash
docker compose up --build
```

Para derrubar o ambiente:

```bash
docker compose down -v
```

### Opção B — Máquinas distintas no laboratório

Cada serviço pode rodar em uma máquina diferente. Substitua os IPs pelos endereços reais da rede do lab:

**Máquina 1 — broker1 (ex: 172.16.201.8)**
```bash
# Broker
docker run -d --name broker1 \
  -e BROKERS_ADDR=172.16.201.4,172.16.201.7 \
  -p 1053:1053 \
  allanyvictoria/broker-setor:v1

# Drone do setor 1
docker run -d --name drone-setor1 \
  -e BROKER_ADDR=172.16.201.8:1053 \
  -e BROKERS_ADDR=172.16.201.4,172.16.201.7 \
  allanyvictoria/drone:v1

# Sensores do setor 1
docker run -d --name sensor-setor1-deriva \
  -e BROKER_ADDR=172.16.201.8:1053 \
  -e INTERVALO=5 \
  allanyvictoria/sensor-setor:v1

docker run -d --name sensor-setor1-bloqueio \
  -e BROKER_ADDR=172.16.201.8:1053 \
  -e INTERVALO=5 \
  allanyvictoria/sensor-setor:v1
```

**Máquina 2 — broker2 (ex: 172.16.201.4)**
```bash
docker run -d --name broker2 \
  -e BROKERS_ADDR=172.16.201.8,172.16.201.7 \
  -p 1053:1053 \
  allanyvictoria/broker-setor:v1

docker run -d --name drone-setor2 \
  -e BROKER_ADDR=172.16.201.4:1053 \
  -e BROKERS_ADDR=172.16.201.8,172.16.201.7 \
  allanyvictoria/drone:v1

docker run -d --name sensor-setor2-objeto \
  -e BROKER_ADDR=172.16.201.4:1053 \
  -e INTERVALO=5 \
  allanyvictoria/sensor-setor:v1

docker run -d --name sensor-setor2-congestionamento \
  -e BROKER_ADDR=172.16.201.4:1053 \
  -e INTERVALO=5 \
  allanyvictoria/sensor-setor:v1
```

**Máquina 3 — broker3 (ex: 172.16.201.7)**
```bash
docker run -d --name broker3 \
  -e BROKERS_ADDR=172.16.201.8,172.16.201.4 \
  -p 1053:1053 \
  allanyvictoria/broker-setor:v1

docker run -d --name drone-setor3 \
  -e BROKER_ADDR=172.16.201.7:1053 \
  -e BROKERS_ADDR=172.16.201.8,172.16.201.4 \
  allanyvictoria/drone:v1

docker run -d --name sensor-setor3-inspecao \
  -e BROKER_ADDR=172.16.201.7:1053 \
  -e INTERVALO=5 \
  allanyvictoria/sensor-setor:v1

docker run -d --name sensor-setor3-risco \
  -e BROKER_ADDR=172.16.201.7:1053 \
  -e INTERVALO=5 \
  allanyvictoria/sensor-setor:v1
```

### Sensor manual

Para injetar requisições manualmente, informando setor, tipo e criticidade:

```bash
# No docker-compose
docker compose --profile manual run --rm sensor-manual

# Em máquinas distintas (passa os IPs dos brokers como argumento)
docker run -it --rm allanyvictoria/sensor-manual:v1 \
  -ip 172.16.201.8,172.16.201.4,172.16.201.7
```

### Comandos úteis

```bash
# Ver logs em tempo real
docker logs -f broker1

# Parar um broker para testar tolerância a falha
docker stop broker2

# Parar um drone para testar replanejamento
docker stop drone-setor1

# Ver todos os containers rodando
docker ps

# Parar e remover todos os containers
docker stop $(docker ps -q)
docker rm $(docker ps -aq)
```

---

## Como Usar

### Sensor automático

Cada sensor gera requisições aleatórias a cada `INTERVALO` segundos (padrão: 5s). O tipo de ocorrência e a criticidade são sorteados a cada envio:

```
[SENSOR bloqueio_rota] Criticidade: alta | Horário: 2026-05-09 14:32:11
```

### Sensor manual

Ao iniciar, exibe um menu interativo:

```
=== SETOR ===
  [1] Setor 1 (broker1)
  [2] Setor 2 (broker2)
  [3] Setor 3 (broker3)

=== TIPO DE SENSOR ===
  [1] bloqueio_rota
  [2] deriva
  ...

=== CRITICIDADE ===
  [1] baixa
  [2] media
  [3] alta
```

### Drone

O drone conecta ao broker do seu setor, registra-se e aguarda missões. Ao receber `MISSAO`, confirma com `ACEITE`, simula a execução (5s) e responde com `CONCLUSAO`:

```
[DRONE drone-setor1] Conectado ao broker (broker1:1053) com sucesso!
[DRONE drone-setor1] Mensagem recebida: BROKER;broker1;MISSAO;bloqueio_rota
[DRONE drone-setor1] Iniciando missão!
[DRONE drone-setor1] Missão concluída!
```

Se o broker cair, o drone tenta reconectar automaticamente nos brokers alternativos definidos em `BROKERS_ADDR` a cada 5s.

### Broker

Exibe no terminal as requisições recebidas, o estado da fila e os despachos:

```
[BROKER broker1]: Servidor iniciado na porta 1053
[BROKER] Nova requisição recebida: bloqueio_rota criticidade alta
[FILA] ENTROU: Req sensor-setor1-1234 | Tipo: bloqueio_rota | Prioridade: 3 | Tamanho atual: 1
[BROKER] Drone drone-setor1 despachado remotamente com sucesso!
[BROKER-1] Sucesso! Drone despachado. 0 requisições restantes na fila.
```

---

## Arquitetura

```
        SETOR 1                  SETOR 2                  SETOR 3
  ┌──────────────────┐    ┌──────────────────┐    ┌──────────────────┐
  │  sensor-deriva   │    │  sensor-objeto   │    │ sensor-inspecao  │
  │  sensor-bloqueio │    │  sensor-congest. │    │ sensor-risco     │
  └────────┬─────────┘    └────────┬─────────┘    └────────┬─────────┘
           │ TCP                   │ TCP                    │ TCP
           ▼                       ▼                        ▼
  ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
  │    broker1      │◀──▶│    broker2      │◀──▶│    broker3      │
  │  porta 1053     │    │  porta 1053     │    │  porta 1053     │
  └────────▲────────┘    └────────▲────────┘    └────────▲────────┘
           │ TCP                   │ TCP                    │ TCP
  ┌────────┴────────┐    ┌────────┴────────┐    ┌────────┴────────┐
  │  drone-setor1   │    │  drone-setor2   │    │  drone-setor3   │
  └─────────────────┘    └─────────────────┘    └─────────────────┘

  Frota compartilhada: qualquer broker pode requisitar drone de outro setor
```

Cada broker gerencia seu setor de forma autônoma. Quando não há drone local disponível, o broker consulta os demais via operação atômica `RESERVAR_E_DESPACHAR` — reserva e despacho ocorrem dentro de um único lock no broker remoto, eliminando race conditions. A conclusão de missões remotas é notificada de volta ao broker de origem via `CONCLUSAO_REMOTA`.

---

## Fila de Prioridade e Aging

As requisições entram numa `container/heap` ordenada por prioridade e, dentro da mesma prioridade, por timestamp (FIFO):

| Criticidade | Prioridade |
|-------------|------------|
| `alta` | 3 |
| `media` | 2 |
| `baixa` | 1 |

O **aging** evita que requisições de baixa prioridade fiquem esperando indefinidamente. A cada 10s, requisições pendentes há mais de 30s têm a prioridade elevada em um nível. Quando há alteração, a heap é reorganizada e o broker tenta despachar novamente.

---

## Concorrência e Tolerância a Falhas

- `sync.RWMutex` protege `mapaDrones`, `mapaRequisicoes` e `filaRequisicoes` contra acesso concorrente
- Cada conexão TCP (sensor, drone, broker remoto) roda em goroutine dedicada
- O lock é liberado antes de chamadas de rede inter-broker e readquirido logo após, evitando bloqueio do sistema durante consultas remotas
- **Operação atômica:** `RESERVAR_E_DESPACHAR` executa reserva e despacho dentro de um único lock no broker remoto, eliminando a janela de race condition entre reservar e despachar
- **Heartbeat:** o drone envia `HEARTBEAT` a cada 10s; o broker verifica a cada 10s e remove drones sem sinal há mais de 20s
- **Requeue:** se um drone cai durante uma missão, a requisição volta à fila com status `pendente` e é redespachada automaticamente
- **Sem SPOF:** se um broker cair, os demais continuam operando independentemente; drones tentam reconectar nos brokers alternativos listados em `BROKERS_ADDR`; sensores reconectam ao próprio broker até ele voltar

---

## Testes

O arquivo `Teste.go` permite testar o sistema externamente via TCP, simulando cenários reais de uso e falhas, sem necessidade de acessar os containers manualmente.

> **Pré-requisito para todos os testes:** Go 1.22+ instalado na máquina onde o teste é executado.
>
> **Pré-requisito adicional para testes de falha** (`drone_cai`, `migracao`, `missao_remota`): Docker CLI instalado e socket acessível. Execute esses testes na mesma máquina que roda os containers envolvidos.

### TESTE 1 — Disponibilidade

Verifica conectividade TCP em cada broker. Útil para confirmar que todos estão no ar antes dos outros testes.

```bash
go run Teste.go disponivel 172.16.201.8:1053 172.16.201.4:1053 172.16.201.7:1053
```

### TESTE 2 — Concorrência

N workers conectam e disparam requisições simultaneamente, testando a fila de prioridade sob carga.

```bash
go run Teste.go concorrencia 172.16.201.8:1053 20
```

### TESTE 3 — Despacho Atômico (inter-broker)

Testa o protocolo `RESERVAR_E_DESPACHAR` diretamente entre brokers, verificando que a operação atômica funciona sem race condition.

```bash
go run Teste.go despacho 172.16.201.4:1053 broker1 3
```

### TESTE 4 — Drone cai durante missão

Envia uma requisição via sensor, aguarda o despacho, mata o container do drone no meio da missão e verifica se o heartbeat detecta a queda e a requisição volta à fila.

```bash
# Execute na máquina que roda broker1 e drone-setor1
go run Teste.go drone_cai 172.16.201.8:1053 broker1 drone-setor1
```

### TESTE 5 — Migração de drone

Derruba o broker do setor e verifica se o drone migra para um broker alternativo e continua atendendo missões.

```bash
# Execute na máquina que roda broker1 e drone-setor1
go run Teste.go migracao 172.16.201.8:1053 172.16.201.4:1053 172.16.201.7:1053 broker1 drone-setor1
```

### TESTE 6 — Missão remota

Para o drone local de um setor e verifica se o broker busca drone em outro setor automaticamente.

```bash
# Execute na máquina que roda broker1 e drone-setor1
go run Teste.go missao_remota broker1 drone-setor1 172.16.201.4:1053
```