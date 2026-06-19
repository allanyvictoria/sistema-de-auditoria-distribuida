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

type Drone struct {
	ID              string
	Conn            net.Conn
	Disponivel      bool
	RequisicaoAtual string
	UltimoHeartbeat time.Time
}

// submeterLiberacao envia um BlocoLiberacao ao consenso.
// Roda I/O de rede em uma goroutine para NÃO travar o rwmu do broker.
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

// handleDrone gerencia a comunicação contínua com um drone específico após o registro inicial.
func handleDrone(m Mensagem, conn net.Conn) {
	rwmu.Lock()

	// O Payload da mensagem inicial de REGISTRO agora contém a Chave Pública do drone
	chavePublicaDrone := m.Payload

	novoDrone := &Drone{
		ID:              m.ID,
		Conn:            conn,
		Disponivel:      true,
		RequisicaoAtual: "",
		UltimoHeartbeat: time.Now(),
	}
	mapaDrones[m.ID] = novoDrone

	// Regista a chave pública do drone no mapa global que a ABCI consulta.
	// Sem isto, o CheckTx da blockchain rejeitará o laudo no final da missão.
	mapaChavesDrones[m.ID] = chavePublicaDrone

	fmt.Printf("[BROKER] Novo drone conectado e homologado: %s - TOTAL: %d\n", m.ID, len(mapaDrones))

	despacharDrone()
	rwmu.Unlock()

	reader := bufio.NewReader(conn)
	for {
		linha, err := reader.ReadString('\n')
		if err != nil {
			rwmu.Lock()
			delete(mapaDrones, m.ID) // Remove o drone do mapa se a conexão cair
			rwmu.Unlock()
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
			drone, existe := mapaDrones[mensagem.ID]

			if existe {
				laudoJSONRecebido := mensagem.Payload

				// Atualiza o estado local do drone
				drone.Disponivel = true
				drone.RequisicaoAtual = ""

				fmt.Printf("[BROKER] Drone %s concluiu a missão! Encaminhando laudo assinado ao consenso...\n", drone.ID)

				// Envelopa o JSON recebido diretamente no PacoteBase (BlocoLaudo).
				pacote := PacoteBase{
					Tipo: BlocoLaudo,
					Data: []byte(laudoJSONRecebido),
				}

				pacoteBytes, err := json.Marshal(pacote)
				if err != nil {
					fmt.Printf("[BROKER] Erro ao serializar pacote de laudo: %v\n", err)
					rwmu.Unlock()
					continue
				}

				txHex := fmt.Sprintf("0x%s", hex.EncodeToString(pacoteBytes))

				cometURL := os.Getenv("COMET_URL")
				if cometURL == "" {
					cometURL = "localhost:26657"
				}

				url := fmt.Sprintf("http://%s/broadcast_tx_commit?tx=%s", cometURL, txHex)

				// Envio assíncrono para o consenso para evitar o bloqueio do Broker
				go func() {
					resp, err := http.Get(url)
					if err != nil {
						fmt.Printf("[BROKER] Erro ao submeter laudo assinado via consenso: %v\n", err)
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

// funcao para verificar periodicamente os heartbeats dos drones e lidar com desconexões
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
							fmt.Printf("[HEARTBEAT] Drone %s caiu durante a missão %s! Submetendo liberação...\n", drone.ID, reqPendente)
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
