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

func main() {
	fmt.Println(" INICIANDO ATAQUE DE INFRAESTRUTURA (INJEÇÃO DE LAUDO) ")

	// 1. O Hacker gera uma chave descartável para assinar o pacote falso
	pubKeyHacker, privKeyHacker, _ := ed25519.GenerateKey(cryptorand.Reader)

	alvoReqID := "Navio_B-123456789" // ID fictício
	droneFalso := "DRONE_FANTASMA_007"
	timestampAtual := fmt.Sprintf("%d", time.Now().Unix())

	fmt.Printf("Alvo: %s | Drone Atacante: %s\n", alvoReqID, droneFalso)

	// 2. Monta a string bruta e assina com a chave hacker
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

	// Endereço padrão do PC 1 no laboratório
	ipDoNo := "172.16.201.1:26657"
	url := fmt.Sprintf("http://%s/broadcast_tx_commit?tx=%s", ipDoNo, txHex)

	fmt.Println("Forçando envio do Laudo falso diretamente ao consenso do nó...")
	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("Erro de conexão: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	respostaString := string(body)

	// Avalia a resposta baseada nos códigos de rejeição do CheckTx
	if strings.Contains(respostaString, `"code":3`) || strings.Contains(respostaString, `"code": 3`) {
		fmt.Println("SUCESSO: A blockchain (Broker) identificou a fraude e rejeitou o laudo!")
		fmt.Printf("Motivo do bloqueio: %s\n", respostaString)
	} else if strings.Contains(respostaString, `"code":0`) {
		fmt.Println("FALHA DE SEGURANÇA: A rede engoliu o laudo do Drone Fantasma!")
	} else {
		fmt.Printf("Resposta inesperada do nó: %s\n", respostaString)
	}
}
