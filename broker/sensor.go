package main

import (
	"bufio"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
)

var (
	chavePublica ed25519.PublicKey
	chavePrivada ed25519.PrivateKey
)

func init() {
	var err error
	chavePublica, chavePrivada, err = ed25519.GenerateKey(rand.Reader)
	if err != nil {
		log.Fatalf("Erro ao gerar chaves criptográficas: %v", err)
	}
	log.Printf("[SENSOR] Chaves geradas com sucesso! PubKey: %x\n", chavePublica)
}

// definirPrioridade relaciona o nome da criticidade com um valor numérico de prioridade.
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

// custoPorCriticidade atrela valores de cobrança às requisições com base na criticidade.
func custoPorCriticidade(criticidade string) int {
	switch criticidade {
	case "alta":
		return 3
	case "media":
		return 2
	default:
		return 1
	}
}

// adicionarRequisicao assina os dados do sensor e remete a requisição à blockchain.
func adicionarRequisicao(m Mensagem) {
	valorCusto := custoPorCriticidade(m.Payload)

	mensagemBruta := fmt.Sprintf("%s:%d:%s", m.ID, valorCusto, m.Acao)
	assinaturaBytes := ed25519.Sign(chavePrivada, []byte(mensagemBruta))

	txPayload := PayloadTransacao{
		Empresa:      m.ID,
		Valor:        valorCusto,
		Criticidade:  m.Payload,
		Acao:         m.Acao,
		ChavePublica: hex.EncodeToString(chavePublica),
		Assinatura:   hex.EncodeToString(assinaturaBytes),
	}
	txBytes, _ := json.Marshal(txPayload)

	pacote := PacoteBase{Tipo: BlocoTransacao, Data: txBytes}
	pacoteBytes, _ := json.Marshal(pacote)

	txHex := fmt.Sprintf("0x%s", hex.EncodeToString(pacoteBytes))

	cometURL := os.Getenv("COMET_URL")
	if cometURL == "" {
		cometURL = "localhost:26657"
	}

	url := fmt.Sprintf("http://%s/broadcast_tx_commit?tx=%s", cometURL, txHex)

	resp, err := http.Get(url)
	if err != nil {
		log.Printf("[SENSOR] Erro ao enviar para a Blockchain: %v\n", err)
		return
	}
	defer resp.Body.Close()

	log.Printf("[SENSOR] Pedido assinado de %s enviado ao Consenso BFT!\n", m.ID)
}

// handleSensor executa a lógica de comunicação de entrada do respectivo sensor conectado.
func handleSensor(m Mensagem, conn net.Conn) {
	adicionarRequisicao(m)
	reader := bufio.NewReader(conn)

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

		adicionarRequisicao(mensagem)
	}
}
