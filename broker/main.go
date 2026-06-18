package main

import (
	"bufio"
	"container/heap"
	"log"
	"net"
	"os"
	"sync"

	"github.com/cometbft/cometbft/abci/server"
)

var rwmu sync.RWMutex                              // mutex para proteger acesso a mapas e fila
var mapaDrones = make(map[string]*Drone)           // drones conectados e estado
var mapaRequisicoes = make(map[string]*Requisicao) // mapa de requisições registradas
var filaRequisicoes FilaRequisicoes                // heap de prioridade
var brokerID string                                // hostname do broker atual

// função para lidar com conexões de sensores e drones
func handleConnection(conn net.Conn) {

	reader := bufio.NewReader(conn)
	linha, err := reader.ReadString('\n')
	if err != nil {
		conn.Close()
		log.Printf("Erro ao ler mensagem inicial: %v", err)
		return
	}
	mensagem, err := ParseMensagem(linha)
	if err != nil {
		conn.Close()
		log.Printf("Mensagem inicial inválida: %v", err)
		return
	}

	// TAREFA 2: Removemos o tratamento de BROKER. Brokers agora falam via CometBFT.
	switch mensagem.Tipo {
	case "SENSOR":
		handleSensor(mensagem, conn)
	case "DRONE":
		handleDrone(mensagem, conn)
	default:
		log.Printf("Tipo de conexão não suportado: %s", mensagem.Tipo)
		conn.Close()
	}
}

func main() {
	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalf("Erro ao obter hostname: %v", err)
	}
	brokerID = hostname

	heap.Init(&filaRequisicoes) // inicializa a fila de requisições

	// Inicia goroutines de suporte
	go verificarHeartbeat()
	go iniciarAging(&filaRequisicoes)

	log.Printf("[BROKER %s]: Iniciando Broker com Consenso BFT...", brokerID)

	// TAREFA 2: Instanciar a aplicação ABCI e o Ledger
	meuLedger := NovoLedger()
	app := NovaDronesApp(meuLedger)

	// Sobe a API HTTP de transparência/auditoria (/saldos, /missoes, /auditoria, /extrato)
	iniciarAPI(meuLedger)
	log.Println("[API] Servidor de transparência ativo na porta 8080")

	// Inicia o servidor ABCI na porta 26658 (Porta padrão que o CometBFT procura)
	// server.NewSocketServer inicia o servidor e o método Start() roda em background
	srv := server.NewSocketServer("tcp://0.0.0.0:26658", app)
	if err := srv.Start(); err != nil {
		log.Fatalf("Erro ao iniciar servidor ABCI: %v", err)
	}
	log.Println("[ABCI] Servidor ABCI ativo na porta 26658 (aguardando CometBFT)")

	// Inicia o servidor TCP para clientes (Sensores e Drones)
	ln, err := net.Listen("tcp", ":1053")
	if err != nil {
		log.Fatalf("Erro ao iniciar servidor TCP: %v", err)
	}
	defer ln.Close()
	log.Printf("[BROKER %s]: Servidor TCP para Clientes iniciado na porta 1053", brokerID)

	// Loop principal para aceitar conexões de Sensores e Drones
	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("Erro ao aceitar conexão: %v", err)
			continue
		}
		go handleConnection(conn)
	}
}
