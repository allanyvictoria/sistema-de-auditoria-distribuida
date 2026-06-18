package main

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"
)

// Drone define a estrutura e o estado de conexão de um drone.
type Drone struct {
	ID              string
	Conn            net.Conn
	Disponivel      bool
	RequisicaoAtual string
	UltimoHeartbeat time.Time
}

// submeterLiberacao submete um bloco de liberação à rede de consenso.
func submeterLiberacao(reqID, motivo string) {
	payload := PayloadLiberacao{
		RequisicaoID: reqID,
		Motivo:       motivo,
	}
	payloadBytes, _ := json.Marshal(payload)

	pacote := PacoteBase{Tipo: BlocoLiberacao, Data: payloadBytes}
	pacoteBytes, _ := json.Marshal(pacote)

	txHex := "0x" + hex.EncodeToString(pacoteBytes)

	cometURL := os.Getenv("COMET_URL")
	if cometURL == "" {
		cometURL = "localhost:26657"
	}

	url := fmt.Sprintf("http://%s/broadcast_tx_commit?tx=%s", cometURL, txHex)

	go func() {
		resp, err := http.Get(url)
		if err != nil {
			fmt.Printf("[BROKER] Erro ao submeter liberação via consenso: %v\n", err)
			return
		}
		defer resp.Body.Close()
	}()
}

// handleDrone processa as conexões e o ciclo de vida dos drones.
func handleDrone(m Mensagem, conn net.Conn) {
	rwmu.Lock()

	reqAnterior := m.Payload

	novoDrone := &Drone{
		ID:              m.ID,
		Conn:            conn,
		Disponivel:      true,
		RequisicaoAtual: "",
		UltimoHeartbeat: time.Now(),
	}
	mapaDrones[m.ID] = novoDrone
	fmt.Printf("[BROKER] Novo drone conectado: %s - TOTAL: %d\n", m.ID, len(mapaDrones))

	// Solicita liberação de missão presa em casos de reconexão do drone
	if reqAnterior != "" {
		if req, existe := mapaRequisicoes[reqAnterior]; existe && req.Status == "em atendimento" {
			fmt.Printf("[REDE] Drone %s reconectou com a missão %s pendente. Solicitando liberação via consenso...\n", m.ID, reqAnterior)
			submeterLiberacao(reqAnterior, "broker_caiu_drone_reconectou")
		}
	}

	despacharDrone()
	rwmu.Unlock()

	reader := bufio.NewReader(conn)
	for {
		linha, err := reader.ReadString('\n')
		if err != nil {
			conn.Close()
			return
		}
		mensagem, err := ParseMensagem(linha)
		if err != nil {
			continue
		}

		switch mensagem.Acao {
		case "HEARTBEAT":
			rwmu.Lock()
			if drone, existe := mapaDrones[mensagem.ID]; existe {
				drone.UltimoHeartbeat = time.Now()
			}
			rwmu.Unlock()

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

		case "CONCLUSAO":
			rwmu.Lock()
			drone, existe := mapaDrones[m.ID]
			if existe {
				rotaDoDrone := mensagem.Payload
				reqID := drone.RequisicaoAtual

				logConclusao := fmt.Sprintf("Escolta %s finalizada", reqID)

				drone.Disponivel = true
				drone.RequisicaoAtual = ""

				laudoPayload := PayloadLaudo{
					RequisicaoID: reqID,
					DroneID:      drone.ID,
					Log:          logConclusao,
					Rota:         rotaDoDrone,
					Timestamp:    time.Now().Format(time.RFC3339),
				}

				laudoBytes, _ := json.Marshal(laudoPayload)
				pacote := PacoteBase{Tipo: BlocoLaudo, Data: laudoBytes}
				pacoteBytes, _ := json.Marshal(pacote)
				txHex := fmt.Sprintf("0x%s", hex.EncodeToString(pacoteBytes))

				cometURL := os.Getenv("COMET_URL")
				if cometURL == "" {
					cometURL = "localhost:26657"
				}

				url := fmt.Sprintf("http://%s/broadcast_tx_commit?tx=%s", cometURL, txHex)

				go func() {
					resp, err := http.Get(url)
					if err == nil {
						defer resp.Body.Close()
					}
				}()
			}
			despacharDrone()
			rwmu.Unlock()
		}
	}
}

// verificarHeartbeat monitora o tempo de inatividade dos drones e encerra conexões expiradas.
func verificarHeartbeat() {
	for {
		time.Sleep(10 * time.Second)
		rwmu.Lock()
		for _, drone := range mapaDrones {
			if time.Since(drone.UltimoHeartbeat) > 20*time.Second {
				drone.Conn.Close()
				reqPendente := drone.RequisicaoAtual

				delete(mapaDrones, drone.ID)

				if reqPendente != "" {
					req := mapaRequisicoes[reqPendente]
					if req != nil {
						switch req.Status {
						case "em atendimento":
							fmt.Printf("[HEARTBEAT] Drone %s desconectado durante a missão %s. Submetendo liberação...\n", drone.ID, reqPendente)
							submeterLiberacao(reqPendente, "drone_caiu")
						case "reservado":
							req.Status = "pendente"
							req.DroneID = ""
						}
					}
				} else {
					fmt.Printf("[HEARTBEAT] Drone %s desconectado por inatividade.\n", drone.ID)
				}
			}
		}
		despacharDrone()
		rwmu.Unlock()
	}
}
