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

var rwmu sync.RWMutex
var mapaDrones = make(map[string]*Drone)
var mapaRequisicoes = make(map[string]*Requisicao)
var filaRequisicoes FilaRequisicoes
var brokerID string

// handleConnection gerencia as conexões de entrada com os clientes TCP.
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

	heap.Init(&filaRequisicoes)

	go verificarHeartbeat()
	go iniciarAging(&filaRequisicoes)

	log.Printf("[BROKER %s]: Iniciando Broker com Consenso BFT...", brokerID)

	meuLedger := NovoLedger()
	app := NovaDronesApp(meuLedger)

	iniciarAPI(meuLedger)
	log.Println("[API] Servidor de transparência ativo na porta 8080")

	srv := server.NewSocketServer("tcp://0.0.0.0:26658", app)
	if err := srv.Start(); err != nil {
		log.Fatalf("Erro ao iniciar servidor ABCI: %v", err)
	}
	log.Println("[ABCI] Servidor ABCI ativo na porta 26658 (aguardando CometBFT)")

	ln, err := net.Listen("tcp", ":1053")
	if err != nil {
		log.Fatalf("Erro ao iniciar servidor TCP: %v", err)
	}
	defer ln.Close()
	log.Printf("[BROKER %s]: Servidor TCP para Clientes iniciado na porta 1053", brokerID)

	for {
		conn, err := ln.Accept()
		if err != nil {
			log.Printf("Erro ao aceitar conexão: %v", err)
			continue
		}
		go handleConnection(conn)
	}
}
