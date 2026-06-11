package main

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
)

// função para definir a prioridade da requisição com base na criticidade
func definirPrioridade(criticidade string) int {
	switch criticidade {
	case "alta":
		return PrioridadeAlta
	case "media":
		return PrioridadeMedia
	case "baixa":
		return PrioridadeNormal
	default:
		return 0
	}
}

// função para adicionar uma nova requisição à fila de prioridade
func adicionarRequisicao(m Mensagem) {
	txPayload := PayloadTransacao{
		Empresa:     m.ID,
		Valor:       1,
		Criticidade: m.Payload,
		Acao:        m.Acao,
	}
	txBytes, _ := json.Marshal(txPayload)
	pacote := PacoteBase{Tipo: BlocoTransacao, Data: txBytes}
	pacoteBytes, _ := json.Marshal(pacote)

	txHex := fmt.Sprintf("0x%s", hex.EncodeToString(pacoteBytes))

	// Busca o parceiro BFT via variável de ambiente (Docker DNS)
	cometURL := os.Getenv("COMET_URL")
	if cometURL == "" {
		cometURL = "localhost:26657" // Fallback seguro
	}

	url := fmt.Sprintf("http://%s/broadcast_tx_commit?tx=%s", cometURL, txHex)

	// Envia e confia no ABCI para colocar na fila depois
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("[SENSOR] ❌ Erro ao enviar para a Blockchain: %v\n", err)
		return
	}
	defer resp.Body.Close()

	log.Printf("[SENSOR] 🚀 Pedido de %s enviado ao Consenso BFT!\n", m.ID)
}

// função para lidar com conexões de sensores, ler as mensagens e adicionar as requisições à fila de prioridade
func handleSensor(m Mensagem, conn net.Conn) {

	adicionarRequisicao(m) // adiciona a requisição recebida à fila de prioridade
	reader := bufio.NewReader(conn)

	// fica escutando o sensor para receber novas requisições
	for {
		linha, err := reader.ReadString('\n')
		if err != nil {
			conn.Close()
			log.Println("[SERVIDOR]: Erro ao receber dados do sensor:", err)
			return
		}

		mensagem, err := ParseMensagem(linha)
		if err != nil {
			log.Printf("Mensagem do sensor inválida: %v", err)
			continue
		}

		adicionarRequisicao(mensagem) // adiciona a nova requisição recebida à fila de prioridade
	}
}
