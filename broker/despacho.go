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

// submeterDespacho envia ao CometBFT um BlocoDespacho anunciando que
// `reqID` foi atribuída ao `droneID` deste broker. Roda em goroutine
// própria pois faz I/O de rede e NÃO deve ser chamada com rwmu travado.
// Retorna 'true' se o CometBFT aceitou, e 'false' se deu erro de conexão.
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

	// A rede Docker usa os nomes dos serviços (node0, node1, node2, node3)
	// Isso garante que o tráfego não passe pelo host do Windows
	cometURL := os.Getenv("COMET_URL")
	if cometURL == "" {
		fmt.Println("[BROKER] ERRO CRÍTICO: COMET_URL não configurada no ambiente!")
		return false
	}

	url := fmt.Sprintf("http://%s/broadcast_tx_commit?tx=%s", cometURL, txHex)

	resp, err := http.Get(url)
	if err != nil {
		// Não printa mais o erro feio do Go, apenas avisa que a rede tá caindo
		fmt.Printf("[BROKER] Atraso na rede CometBFT (%s). Aplicando Rollback...\n", cometURL)
		return false
	}
	defer resp.Body.Close()

	return true
}

// despacharDrone percorre a fila de requisições pendentes e, para cada uma
// que pode ser atendida por um drone LOCAL disponível, submete um
// BlocoDespacho ao consenso anunciando essa atribuição.
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

			// Reserva otimista local (trava o drone temporariamente)
			drone.Disponivel = false
			drone.RequisicaoAtual = req.ID
			req.Status = "reservado"

			paraSubmeter = append(paraSubmeter, par{reqID: req.ID, droneID: drone.ID})
			break // passa para a próxima requisição pendente da fila
		}
	}

	if len(paraSubmeter) == 0 {
		return false
	}

	// O SEGREDO ESTÁ AQUI: Iniciamos uma Goroutine!
	// O I/O de rede vai rodar em paralelo. Assim, a função despacharDrone()
	// retorna imediatamente, o FinalizeBlock termina, e o CometBFT é
	// descongelado a tempo de receber o HTTP que vamos mandar abaixo!
	go func(missoes []par) {
		var falhas []par
		for _, p := range missoes {
			// JITTER: Atraso aleatório para evitar colisão dos brokers
			time.Sleep(time.Duration(rand.Intn(50)) * time.Millisecond)

			sucesso := submeterDespacho(p.reqID, p.droneID)
			if !sucesso {
				falhas = append(falhas, p)
			}
		}

		// Se a rede caiu, pegamos o cadeado de volta em background e desfazemos a reserva
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
				fmt.Printf("[FILA] 🔄 ROLLBACK: Rede falhou. Req %s devolvida e Drone %s liberado!\n", f.reqID, f.droneID)
			}
			rwmu.Unlock()
		}
	}(paraSubmeter)

	return true
}
