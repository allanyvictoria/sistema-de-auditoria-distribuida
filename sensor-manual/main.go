### `main.go`

```go
package main

import (
	"bufio"
	"crypto/ed25519"
	cryptorand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type TipoBloco string

const (
	BlocoRegistro      TipoBloco = "REGISTRO"
	BlocoEmissao       TipoBloco = "EMISSAO"
	BlocoTransacao     TipoBloco = "TRANSACAO"
	BlocoTransferencia TipoBloco = "TRANSFERENCIA"
)

type PacoteBase struct {
	Tipo TipoBloco       `json:"tipo"`
	Data json.RawMessage `json:"data"`
}

type PayloadRegistro struct {
	Empresa string `json:"empresa"`
}

type PayloadEmissao struct {
	Empresa string `json:"empresa"`
	Valor   int    `json:"valor"`
}

type PayloadTransacao struct {
	Empresa      string `json:"empresa"`
	Valor        int    `json:"valor"`
	Criticidade  string `json:"criticidade"`
	Acao         string `json:"acao"`
	Timestamp    string `json:"timestamp"`
	ChavePublica string `json:"chave_publica"`
	Assinatura   string `json:"add_assinatura"`
}

type PayloadTransferencia struct {
	Origem       string `json:"origem"`
	Destino      string `json:"destino"`
	Valor        int    `json:"valor"`
	ChavePublica string `json:"chave_publica"`
	Assinatura   string `json:"assinatura"`
}

var (
	chavePublica ed25519.PublicKey
	chavePrivada ed25519.PrivateKey
	minhaEmpresa string
)

func init() {
	rand.Seed(time.Now().UnixNano())

	var err error
	chavePublica, chavePrivada, err = ed25519.GenerateKey(cryptorand.Reader)
	if err != nil {
		log.Fatalf("Erro ao gerar chaves: %v", err)
	}
}

// enviarParaCometBFT transmite o pacote estruturado ao nó do ecossistema CometBFT via requisição HTTP.
func enviarParaCometBFT(pacote PacoteBase) {
	pacoteBytes, _ := json.Marshal(pacote)
	txHex := fmt.Sprintf("0x%s", hex.EncodeToString(pacoteBytes))
	url := fmt.Sprintf("http://127.0.0.1:26657/broadcast_tx_commit?tx=%s", txHex)

	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("Erro de conexão com o CometBFT: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	
	// Validação do código de retorno obtido do processo CheckTx.
	if strings.Contains(string(body), `"code":0`) {
		fmt.Println("Transação aceita e registrada no bloco.")
	} else {
		fmt.Printf("Transação rejeitada pelo nó. Resposta: %s\n", string(body))
	}
}

// registrarEmpresa encaminha a requisição de inicialização e registro da respectiva companhia à rede.
func registrarEmpresa() {
	txPayload := PayloadRegistro{
		Empresa: minhaEmpresa,
	}
	txBytes, _ := json.Marshal(txPayload)

	fmt.Println("Enviando pedido de registro para a blockchain...")
	enviarParaCometBFT(PacoteBase{Tipo: BlocoRegistro, Data: txBytes})
}

// transferirCreditos realiza os procedimentos de coleta de parâmetros e assinatura para a transferência de fundos.
func transferirCreditos(reader *bufio.Reader) {
	fmt.Print("Empresa de destino (ex: Navio_B): ")
	destino, _ := reader.ReadString('\n')
	destino = strings.TrimSpace(destino)

	fmt.Print("Valor a transferir: ")
	valorStr, _ := reader.ReadString('\n')
	valorStr = strings.TrimSpace(valorStr)

	valor, err := strconv.Atoi(valorStr)
	if err != nil || valor <= 0 {
		fmt.Println("Valor inválido.")
		return
	}

	// Processamento da assinatura digital contendo os parâmetros de origem, destino e valor.
	mensagemBruta := fmt.Sprintf("%s:%s:%d", minhaEmpresa, destino, valor)
	assinaturaBytes := ed25519.Sign(chavePrivada, []byte(mensagemBruta))

	txPayload := PayloadTransferencia{
		Origem:       minhaEmpresa,
		Destino:      destino,
		Valor:        valor,
		ChavePublica: hex.EncodeToString(chavePublica),
		Assinatura:   hex.EncodeToString(assinaturaBytes),
	}
	txBytes, _ := json.Marshal(txPayload)

	fmt.Printf("Enviando transferência de %d créditos para %s...\n", valor, destino)
	enviarParaCometBFT(PacoteBase{Tipo: BlocoTransferencia, Data: txBytes})
}

// custoPorCriticidade calcula o valor de débito operacional com base no grau de criticidade.
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

// solicitarMissao gera e assina dinamicamente um pedido de processo de missão simulado por sensores.
func solicitarMissao() {
	tipos := []string{
		"deriva",
		"bloqueio_rota",
		"objeto_nao_identificado",
		"congestionamento",
		"inspecao_visual",
		"risco_ambiental",
	}
	criticidades := []string{"baixa", "media", "alta"}
	tipoSensor := tipos[rand.Intn(len(tipos))]
	nivelcriticidade := criticidades[rand.Intn(len(criticidades))]

	acao := tipoSensor
	criticidade := nivelcriticidade
	valorCusto := custoPorCriticidade(criticidade)

	// Geração de carimbo de data/hora em escala de nanossegundos.
	timestampAtual := fmt.Sprintf("%d", time.Now().UnixNano())

	// Vinculação criptográfica dos dados da mensagem com a inclusão do timestamp.
	mensagemBruta := fmt.Sprintf("%s:%d:%s:%s", minhaEmpresa, valorCusto, acao, timestampAtual)
	assinaturaBytes := ed25519.Sign(chavePrivada, []byte(mensagemBruta))

	txPayload := PayloadTransacao{
		Empresa:      minhaEmpresa,
		Valor:        valorCusto,
		Criticidade:  criticidade,
		Acao:         acao,
		Timestamp:    timestampAtual,
		ChavePublica: hex.EncodeToString(chavePublica),
		Assinatura:   hex.EncodeToString(assinaturaBytes),
	}
	txBytes, _ := json.Marshal(txPayload)

	fmt.Println("Assinando e enviando pedido de missão...")
	enviarParaCometBFT(PacoteBase{Tipo: BlocoTransacao, Data: txBytes})
}

// menu gerencia a interface de linha de comando e o fluxo de chamadas do sistema.
func menu() {
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Digite o nome da sua Companhia (ex: Navio_A): ")
	nome, _ := reader.ReadString('\n')
	minhaEmpresa = strings.TrimSpace(nome)

	for {
		fmt.Printf("\n--- TERMINAL DO SENSOR: %s ---\n", minhaEmpresa)
		fmt.Println("1. Emitir Créditos Iniciais")
		fmt.Println("2. Solicitar Missão")
		fmt.Println("3. Transferir Créditos para outra Empresa")
		fmt.Println("4. Sair")
		fmt.Print("Escolha uma opção: ")

		opcao, _ := reader.ReadString('\n')
		opcao = strings.TrimSpace(opcao)

		switch opcao {
		case "1":
			registrarEmpresa()
		case "2":
			solicitarMissao()
		case "3":
			transferirCreditos(reader)
		case "4":
			fmt.Println("Encerrando terminal.")
			return
		default:
			fmt.Println("Opção inválida.")
		}
	}
}

func main() {
	fmt.Println("Par de chaves criptográficas estabelecido para a sessão atual.")
	menu()
}

```
