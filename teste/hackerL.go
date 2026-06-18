package main

import (
	"crypto/ed25519"
	cryptorand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type TipoBloco string

const BlocoLaudo TipoBloco = "LAUDO"

type PacoteBase struct {
	Tipo TipoBloco       `json:"tipo"`
	Data json.RawMessage `json:"data"`
}

type PayloadLaudo struct {
	RequisicaoID string `json:"requisicao_id"`
	DroneID      string `json:"drone_id"`
	Log          string `json:"log"`
	Rota         string `json:"rota"`
	Timestamp    string `json:"timestamp"`
	ChavePublica string `json:"chave_publica"`
	Assinatura   string `json:"assinatura"`
}

// main executa a simulação de injeção de laudo fraudulento na rede.
func main() {
	fmt.Println("INICIANDO ATAQUE DE INFRAESTRUTURA (INJEÇÃO DE LAUDO)")

	// Geração de chave criptográfica não autorizada para assinatura do pacote.
	pubKeyHacker, privKeyHacker, _ := ed25519.GenerateKey(cryptorand.Reader)

	alvoReqID := "Navio_B-123456789"
	droneFalso := "DRONE_FANTASMA_007"
	timestampAtual := fmt.Sprintf("%d", time.Now().Unix())

	fmt.Printf("Alvo: %s | Drone Atacante: %s\n", alvoReqID, droneFalso)

	// Estruturação da mensagem bruta e aplicação da assinatura com a chave do atacante.
	mensagemBruta := fmt.Sprintf("%s:%s:%s", alvoReqID, droneFalso, timestampAtual)
	assinaturaForjada := ed25519.Sign(privKeyHacker, []byte(mensagemBruta))

	txPayload := PayloadLaudo{
		RequisicaoID: alvoReqID,
		DroneID:      droneFalso,
		Log:          "FRAUDE: Poluição ignorada no relatório do invasor.",
		Rota:         "Lat: 0.00, Lng: 0.00",
		Timestamp:    timestampAtual,
		ChavePublica: hex.EncodeToString(pubKeyHacker),
		Assinatura:   hex.EncodeToString(assinaturaForjada),
	}

	txBytes, _ := json.Marshal(txPayload)
	pacoteBytes, _ := json.Marshal(PacoteBase{Tipo: BlocoLaudo, Data: txBytes})
	txHex := fmt.Sprintf("0x%s", hex.EncodeToString(pacoteBytes))

	ipDoNo := "172.16.201.1:26657"
	url := fmt.Sprintf("http://%s/broadcast_tx_commit?tx=%s", ipDoNo, txHex)

	fmt.Println("Submetendo laudo forjado diretamente ao consenso do nó...")
	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("Erro de conexão: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	respostaString := string(body)

	// Verificação do código de resposta do CheckTx para validar a rejeição.
	if strings.Contains(respostaString, `"code":3`) || strings.Contains(respostaString, `"code": 3`) {
		fmt.Println("SUCESSO: A blockchain identificou a fraude e rejeitou o laudo.")
		fmt.Printf("Motivo do bloqueio: %s\n", respostaString)
	} else if strings.Contains(respostaString, `"code":0`) {
		fmt.Println("FALHA DE SEGURANÇA: A rede validou o laudo de origem desconhecida.")
	} else {
		fmt.Printf("Resposta inesperada do nó: %s\n", respostaString)
	}
}
