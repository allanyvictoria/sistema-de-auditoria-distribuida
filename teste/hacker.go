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
)

type TipoBloco string

const BlocoTransferencia TipoBloco = "TRANSFERENCIA"

type PacoteBase struct {
	Tipo TipoBloco       `json:"tipo"`
	Data json.RawMessage `json:"data"`
}

type PayloadTransferencia struct {
	Origem       string `json:"origem"`
	Destino      string `json:"destino"`
	Valor        int    `json:"valor"`
	ChavePublica string `json:"chave_publica"`
	Assinatura   string `json:"assinatura"`
}

// main executa a simulação de fraude financeira na rede.
func main() {
	fmt.Println("INICIANDO SCRIPT DE TESTE DE VULNERABILIDADE")

	// Geração de par de chaves próprio, distinto das chaves da vítima.
	chavePublicaHacker, chavePrivadaHacker, _ := ed25519.GenerateKey(cryptorand.Reader)

	vitima := "Navio_B"
	hacker := "Pirata_1"
	valorRoubado := 50

	fmt.Printf("Alvo: %s | Tentativa de transferência: %d créditos\n", vitima, valorRoubado)

	// Montagem do payload forjando a autorização da vítima.
	mensagemBruta := fmt.Sprintf("%s:%s:%d", vitima, hacker, valorRoubado)

	// Assinatura do payload utilizando a chave privada não correspondente à origem.
	assinaturaForjada := ed25519.Sign(chavePrivadaHacker, []byte(mensagemBruta))

	txPayload := PayloadTransferencia{
		Origem:       vitima,
		Destino:      hacker,
		Valor:        valorRoubado,
		ChavePublica: hex.EncodeToString(chavePublicaHacker),
		Assinatura:   hex.EncodeToString(assinaturaForjada),
	}

	txBytes, _ := json.Marshal(txPayload)
	pacoteBytes, _ := json.Marshal(PacoteBase{Tipo: BlocoTransferencia, Data: txBytes})
	txHex := fmt.Sprintf("0x%s", hex.EncodeToString(pacoteBytes))

	ipDoNo := "172.16.201.1:26657"
	url := fmt.Sprintf("http://%s/broadcast_tx_commit?tx=%s", ipDoNo, txHex)

	fmt.Println("Submetendo transação fraudulenta à blockchain...")
	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("Erro de conexão: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// Avaliação do retorno do mempool sobre a validação da transação.
	if strings.Contains(string(body), `"code":0`) {
		fmt.Println("FALHA DE SEGURANÇA: A blockchain processou a transação fraudulenta (código 0).")
	} else {
		fmt.Println("SUCESSO: A rede identificou a divergência criptográfica e rejeitou a transação.")
		fmt.Printf("Motivo da rejeição: %s\n", string(body))
	}
}
