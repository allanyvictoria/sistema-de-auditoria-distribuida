package main

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"time"
)

// submeterDespacho envia um BlocoDespacho ao CometBFT atribuindo reqID ao droneID.
// Executada em goroutine separada para evitar bloqueios de I/O na thread principal.
func submeterDespacho(reqID, droneID string) bool {
	payload := PayloadDespacho{
		RequisicaoID: reqID,
		DroneID:      droneID,
		BrokerID:     brokerID,
	}
	payloadBytes, _ := json.Marshal(payload)

	pacote := PacoteBase{Tipo: BlocoDespacho, Data: payloadBytes}
	pacoteBytes, _ := json.Marshal(pacote)

	txHex := "0x" + hex.EncodeToString(pacoteBytes)

	cometURL := os.Getenv("COMET_URL")
	if cometURL == "" {
		fmt.Println("[BROKER] ERRO CRÍTICO: COMET_URL não configurada no ambiente!")
		return false
	}

	url := fmt.Sprintf("http://%s/broadcast_tx_commit?tx=%s", cometURL, txHex)

	resp, err := http.Get(url)
	if err != nil {
		fmt.Printf("[BROKER] Falha de comunicação com a rede CometBFT (%s). Aplicando Rollback...\n", cometURL)
		return false
	}
	defer resp.Body.Close()

	return true
}

// despacharDrone verifica a fila de requisições e atribui drones disponíveis
// aos pedidos pendentes. Submete o despacho ao consenso em background.
func despacharDrone() bool {
	type par struct {
		reqID   string
		droneID string
	}
	var paraSubmeter []par

	for _, req := range filaRequisicoes {
		if req.Status != "pendente" {
			continue
		}

		for _, drone := range mapaDrones {
			if !drone.Disponivel {
				continue
			}

			// Reserva do drone e atualização de status locais
			drone.Disponivel = false
			drone.RequisicaoAtual = req.ID
			req.Status = "reservado"

			paraSubmeter = append(paraSubmeter, par{reqID: req.ID, droneID: drone.ID})
			break
		}
	}

	if len(paraSubmeter) == 0 {
		return false
	}

	go func(missoes []par) {
		var falhas []par
		for _, p := range missoes {
			time.Sleep(time.Duration(rand.Intn(50)) * time.Millisecond)

			sucesso := submeterDespacho(p.reqID, p.droneID)
			if !sucesso {
				falhas = append(falhas, p)
			}
		}

		// Reversão de estado em caso de falha na rede
		if len(falhas) > 0 {
			rwmu.Lock()
			for _, f := range falhas {
				if req, ok := mapaRequisicoes[f.reqID]; ok && req.Status == "reservado" {
					req.Status = "pendente"
				}
				if drone, ok := mapaDrones[f.droneID]; ok && drone.RequisicaoAtual == f.reqID {
					drone.Disponivel = true
					drone.RequisicaoAtual = ""
				}
				fmt.Printf("[FILA] ROLLBACK: Rede falhou. Req %s devolvida e Drone %s liberado!\n", f.reqID, f.droneID)
			}
			rwmu.Unlock()
		}
	}(paraSubmeter)

	return true
}
