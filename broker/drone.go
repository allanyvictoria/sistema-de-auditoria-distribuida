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
	"time"
)

// O struct do drone representa cada drone conectado ao broker
type Drone struct {
	ID              string
	Conn            net.Conn
	Disponivel      bool
	RequisicaoAtual string // Guarda a requisição atual do drone, se tiver
	UltimoHeartbeat time.Time
}

// submeterLaudoToBlockchain envia o laudo da missão para o consenso do CometBFT
func submeterLaudoToBlockchain(droneID string) {
	payload := PayloadLaudo{
		DroneID: droneID,
		Log:     "Missão concluída com sucesso",
	}
	payloadBytes, _ := json.Marshal(payload)

	pacote := PacoteBase{
		Tipo: BlocoLaudo,
		Data: payloadBytes,
	}
	pacoteBytes, _ := json.Marshal(pacote)

	// Converte para Hexadecimal prefixado com 0x para o CometBFT
	txHex := "0x" + hex.EncodeToString(pacoteBytes)
	url := fmt.Sprintf("http://localhost:26657/broadcast_tx_commit?tx=%s", txHex)

	resp, err := http.Get(url)
	if err != nil {
		log.Printf("[BLOCKCHAIN] Erro ao submeter laudo via RPC: %v", err)
		return
	}
	defer resp.Body.Close()
	log.Printf("[BLOCKCHAIN] Laudo do drone %s enviado ao CometBFT para consenso!", droneID)
}

// função para lidar com mensagens recebidas de drones
func handleDrone(m Mensagem, conn net.Conn) {

	rwmu.Lock()
	// Registra o novo drone no mapa de drones conectados
	novoDrone := &Drone{
		ID:              m.ID,
		Conn:            conn,
		Disponivel:      true,
		RequisicaoAtual: "",
		UltimoHeartbeat: time.Now(),
	}
	mapaDrones[m.ID] = novoDrone

	fmt.Printf("[BROKER] Novo drone conectado: %s - TOTAL: %d\n", m.ID, len(mapaDrones))
	despacharDrone()

	rwmu.Unlock()

	reader := bufio.NewReader(conn)
	// fica escutando o drone para receber mensagens de heartbeat, aceite e conclusão
	for {

		linha, err := reader.ReadString('\n')
		if err != nil {
			conn.Close()
			log.Println("[SERVIDOR]: Erro ao escutar drone:", err)
			return
		}
		mensagem, err := ParseMensagem(linha)
		if err != nil {
			log.Printf("Mensagem inválida recebida: %v", err)
			return
		}

		// Trata as ações específicas para mensagens de drones
		switch mensagem.Acao {

		// Atualiza o timestamp do último heartbeat recebido para monitorar a conexão do drone
		case "HEARTBEAT":
			rwmu.Lock()
			if drone, existe := mapaDrones[mensagem.ID]; existe {
				drone.UltimoHeartbeat = time.Now()
			}
			rwmu.Unlock()

		// Quando o drone aceita uma missão, marca ele como indisponível e atualiza a requisição associada
		case "ACEITE":
			rwmu.Lock()
			if drone, existe := mapaDrones[mensagem.ID]; existe {
				req := mapaRequisicoes[drone.RequisicaoAtual]
				if req != nil {
					req.Status = "em atendimento"
				}
				drone.Disponivel = false
			}
			rwmu.Unlock()

		// Quando o drone conclui uma missão libera drone, atualiza requisição e gera laudo na blockchain
		case "CONCLUSAO":
			rwmu.Lock()
			drone, existe := mapaDrones[m.ID]
			if existe {
				drone.Disponivel = true
				drone.RequisicaoAtual = ""

				// 1. Monta o Laudo
				laudoPayload := PayloadLaudo{
					DroneID: drone.ID,
					Log:     "Missão concluída com sucesso no Estreito de Ormuz",
				}
				laudoBytes, _ := json.Marshal(laudoPayload)
				pacote := PacoteBase{Tipo: BlocoLaudo, Data: laudoBytes}
				pacoteBytes, _ := json.Marshal(pacote)

				txHex := fmt.Sprintf("0x%s", hex.EncodeToString(pacoteBytes))

				// 2. Resolve o endereço de rede do Docker
				cometURL := os.Getenv("COMET_URL")
				if cometURL == "" {
					cometURL = "localhost:26657"
				}

				url := fmt.Sprintf("http://%s/broadcast_tx_commit?tx=%s", cometURL, txHex)

				// 3. Dispara a submissão em Goroutine para não prender o Mutex
				go func() {
					resp, err := http.Get(url)
					if err != nil {
						log.Printf("[BLOCKCHAIN] ❌ Erro ao submeter laudo via RPC: %v\n", err)
						return
					}
					defer resp.Body.Close()
				}()
			}
			despacharDrone()
			rwmu.Unlock()
		}

	}
}

// função para monitorar periodicamente os drones e detectar quedas
func verificarHeartbeat() {
	for {
		time.Sleep(10 * time.Second)
		rwmu.Lock()
		for _, drone := range mapaDrones {
			if time.Since(drone.UltimoHeartbeat) > 20*time.Second {
				log.Printf("Drone %s desconectado por inatividade (timeout)", drone.ID)
				drone.Conn.Close()

				if drone.RequisicaoAtual != "" {
					reqID := drone.RequisicaoAtual
					req := mapaRequisicoes[reqID]
					if req != nil {
						req.Status = "pendente"
						req.DroneID = ""
						filaRequisicoes.Push(req)
						log.Printf("[BROKER] Requisição %s voltou para a fila devido à queda do drone", req.ID)
					}
				}
				delete(mapaDrones, drone.ID)
			}
		}
		despacharDrone()
		rwmu.Unlock()
	}
}
