package main

import (
	"bufio"
	"crypto/ed25519"
	cryptorand "crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math/rand/v2"
	"net"
	"os"
	"strings"
	"time"
)

<<<<<<< HEAD
// obterBrokerAddr recupera o endereço do broker da variável de ambiente ou aplica o valor padrão.
=======
// Estrutura que o Broker ABCI agora exige para o Laudo
type PayloadLaudo struct {
	RequisicaoID string `json:"requisicao_id"`
	DroneID      string `json:"drone_id"`
	Log          string `json:"log"`
	Rota         string `json:"rota"`
	Timestamp    string `json:"timestamp"`
	ChavePublica string `json:"chave_publica"`
	Assinatura   string `json:"assinatura"`
}

var (
	chavePublicaDrone ed25519.PublicKey
	chavePrivadaDrone ed25519.PrivateKey
)

func init() {
	// O Drone agora gera sua própria identidade secreta ao ser ligado!
	var err error
	chavePublicaDrone, chavePrivadaDrone, err = ed25519.GenerateKey(cryptorand.Reader)
	if err != nil {
		log.Fatalf("Erro ao gerar chaves do drone: %v", err)
	}
}

// Tenta obter o endereço do broker a partir da variável de ambiente, ou usa o padrão
>>>>>>> 39cd027 (Atualizados e comentados)
func obterBrokerAddr() string {
	addr := os.Getenv("BROKER_ADDR")
	if addr == "" {
		addr = "broker:1883"
	}
	fmt.Println("Endereço do broker:", addr)
	return addr
}

// conectarBroker tenta estabelecer uma conexão TCP com o broker principal ou seus alternativos listados.
func conectarBroker() (net.Conn, string, error) {
	addrPrincipal := obterBrokerAddr()

	// Tentativa de conexão com o broker principal.
	conn, err := net.DialTimeout("tcp", addrPrincipal, 5*time.Second)
	if err == nil {
		return conn, addrPrincipal, nil
	}

	// Iteração e tentativa de conexão com os brokers alternativos.
	brokersStr := os.Getenv("BROKERS_ADDR")
	if brokersStr != "" {
		for _, broker := range strings.Split(brokersStr, ",") {
			broker = strings.TrimSpace(broker)
			if broker == "" {
				continue
			}

			addrAlternativo := broker
			if !strings.Contains(addrAlternativo, ":") {
				addrAlternativo = fmt.Sprintf("%s:1053", broker)
			}

			if addrAlternativo == addrPrincipal {
				continue
			}
			conn, err := net.DialTimeout("tcp", addrAlternativo, 5*time.Second)
			if err == nil {
				return conn, addrAlternativo, nil
			}
		}
	}
	return nil, "", fmt.Errorf("nenhum broker disponível")
}

// heartbeat transmite pacotes periódicos para confirmar a disponibilidade do cliente na rede.
func heartbeat(conn net.Conn, id string) {
	for {
		mensagem := fmt.Sprintf("DRONE;%s;HEARTBEAT;%s\n", id, "")
		_, err := conn.Write([]byte(mensagem))
		if err != nil {
<<<<<<< HEAD
			// Interrupção silenciosa da rotina caso a conexão de rede já tenha sido encerrada.
=======
>>>>>>> 39cd027 (Atualizados e comentados)
			if strings.Contains(err.Error(), "use of closed network connection") {
				return
			}
			log.Printf("Erro ao enviar heartbeat: %v", err)
			return
		}
		time.Sleep(10 * time.Second)
	}
}

// receberMissao processa as mensagens vindas do broker e gerencia o fluxo de execução das missões.
func receberMissao(conn net.Conn, id string) {
	emMissao := false
	reader := bufio.NewReader(conn)
<<<<<<< HEAD
	
=======

>>>>>>> 39cd027 (Atualizados e comentados)
	for {
		mensagem, err := reader.ReadString('\n')
		if err != nil {
			log.Printf("[DRONE %s] Erro de leitura. Conexão perdida com o servidor.", id)
			return
		}
		mensagem = strings.TrimSpace(mensagem)
		log.Printf("[DRONE %s] Mensagem recebida: %s", id, mensagem)

		if strings.Contains(mensagem, "MISSAO") && !emMissao {
			emMissao = true
<<<<<<< HEAD
			log.Printf("[DRONE %s] Iniciando missão.", id)

			// Envio da confirmação de aceite.
			conn.Write([]byte(fmt.Sprintf("DRONE;%s;ACEITE;\n", id)))

			// Simulação do tempo de execução do trajeto.
			time.Sleep(10 * time.Second)

			// Geração autônoma de coordenadas para a rota percorrida.
=======

			partes := strings.Split(mensagem, ";")
			reqID := "ID_DESCONHECIDO"
			if len(partes) >= 4 {
				reqID = partes[3]
			}

			log.Printf("[DRONE %s] Iniciando missão: %s!", id, reqID)
			conn.Write([]byte(fmt.Sprintf("DRONE;%s;ACEITE;\n", id)))

			// Simula o tempo de voo
			time.Sleep(10 * time.Second)

			// O PRÓPRIO DRONE GERA SEU GPS
>>>>>>> 39cd027 (Atualizados e comentados)
			latOrigem := -10.0 + (rand.Float64() * 20.0)
			lonOrigem := -40.0 + (rand.Float64() * 20.0)
			latDestino := latOrigem + (rand.Float64() * 2.0)
			lonDestino := lonOrigem + (rand.Float64() * 2.0)
			rotaDinamica := fmt.Sprintf("Lat: %.4f, Lon: %.4f -> Lat: %.4f, Lon: %.4f", latOrigem, lonOrigem, latDestino, lonDestino)

<<<<<<< HEAD
			// Transmissão do status de conclusão juntamente com os dados da rota.
			conn.Write([]byte(fmt.Sprintf("DRONE;%s;CONCLUSAO;%s\n", id, rotaDinamica)))

			emMissao = false
			log.Printf("[DRONE %s] Missão concluída. Rota enviada: %s", id, rotaDinamica)
=======
			// O Drone assina digitalmente o Laudo de Conclusão
			timestampAtual := fmt.Sprintf("%d", time.Now().Unix())
			mensagemBruta := fmt.Sprintf("%s:%s:%s", reqID, id, timestampAtual)
			assinaturaBytes := ed25519.Sign(chavePrivadaDrone, []byte(mensagemBruta))

			laudoPayload := PayloadLaudo{
				RequisicaoID: reqID,
				DroneID:      id,
				Log:          "Missão concluída sem anomalias ambientais.",
				Rota:         rotaDinamica,
				Timestamp:    timestampAtual,
				ChavePublica: hex.EncodeToString(chavePublicaDrone),
				Assinatura:   hex.EncodeToString(assinaturaBytes),
			}

			// Transforma o Laudo em JSON para enviar pela rede
			laudoJSON, _ := json.Marshal(laudoPayload)

			// Manda o JSON assinado para o Broker
			conn.Write([]byte(fmt.Sprintf("DRONE;%s;CONCLUSAO;%s\n", id, string(laudoJSON))))

			emMissao = false
			log.Printf("[DRONE %s] Missão concluída e assinada enviada!", id)
>>>>>>> 39cd027 (Atualizados e comentados)
		}
	}
}

func main() {
	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalf("Erro ao obter hostname: %v", err)
	}

	for {
		conn, addrConectado, err := conectarBroker()
		if err != nil {
			log.Printf("Falha na conexão com brokers. Nova tentativa em 5 segundos.")
			time.Sleep(5 * time.Second)
			continue
		} else {
<<<<<<< HEAD
			log.Printf("[DRONE %s] Conectado ao broker (%s) com sucesso.", hostname, addrConectado)
=======
			log.Printf("[DRONE %s] Conectado ao broker (%s)! Chave Pública: %x...", hostname, addrConectado, chavePublicaDrone[:5])
>>>>>>> 39cd027 (Atualizados e comentados)
		}

		// O DRONE AGORA MANDA A CHAVE PÚBLICA DELE NO REGISTRO
		chavePubHex := hex.EncodeToString(chavePublicaDrone)
		mensagem := fmt.Sprintf("DRONE;%s;REGISTRO;%s\n", hostname, chavePubHex)

		_, err = conn.Write([]byte(mensagem))
		if err != nil {
			log.Printf("Erro ao enviar registro: %v", err)
		}

<<<<<<< HEAD
		go heartbeat(conn, id)

		receberMissao(conn, id)
=======
		go heartbeat(conn, hostname)
		receberMissao(conn, hostname)
>>>>>>> 39cd027 (Atualizados e comentados)

		conn.Close()
		log.Printf("Conexão perdida. Iniciando rotina de reconexão.")
	}
}
