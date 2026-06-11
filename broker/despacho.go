package main

import (
	"bufio"
	"container/heap"
	"fmt"
	"log"
	"net"
)

// despacharDrone tenta despachar as missões pendentes, primeiro localmente e
// depois para brokers remotos.
func despacharDrone() bool {
	algumDespachado := false

	for filaRequisicoes.Len() > 0 {
		req := filaRequisicoes[0] // obtem a requisição de maior prioridade (sem remover ainda)

		// Se a requisição já não for mais pendente, remove e continua
		if req.Status != "pendente" {
			heap.Pop(&filaRequisicoes)
			continue
		}

		droneEncontrado := false

		// Tenta despacho local primeiro
		for _, drone := range mapaDrones {
			if drone.Disponivel {
				req.Status = "em atendimento"
				req.DroneID = drone.ID
				drone.Disponivel = false
				drone.RequisicaoAtual = req.ID

				heap.Pop(&filaRequisicoes) // remove a requisição da fila após reservar o drone

				drone.Conn.Write([]byte(fmt.Sprintf("BROKER;%s;MISSAO;%s\n", brokerID, req.Tipo)))
				droneEncontrado = true
				algumDespachado = true
				break
			}
		}

		if droneEncontrado {
			continue
		}

		// Nenhum drone local — tenta brokers remotos
		reqRemovida := heap.Pop(&filaRequisicoes)
		reqID := req.ID
		reqTipo := req.Tipo

		rwmu.Unlock() // libera o mutex antes de tentar despachar para brokers remotos,
		// evitando bloqueios desnecessários durante conexões de rede
		remotoEncontrado := false

		// Tenta despachar para cada broker remoto até encontrar um disponível
		for _, broker := range brokers {
			conn, err := net.Dial("tcp", fmt.Sprintf("%s:1053", broker))
			if err != nil {
				continue
			}

			// Operação atômica: reserva e despacha em uma única mensagem para evitar
			// condições de corrida entre múltiplos brokers tentando despachar a mesma requisição
			conn.Write([]byte(fmt.Sprintf(
				"BROKER;%s;RESERVAR_E_DESPACHAR;%s/%s/%s\n",
				brokerID, reqID, reqTipo, brokerID,
			)))

			reader := bufio.NewReader(conn)
			linha, err := reader.ReadString('\n')
			conn.Close()

			if err != nil {
				log.Printf("[BROKER] Erro ao ler resposta do broker remoto %s: %v", broker, err)
				continue
			}

			mensagem, err := ParseMensagem(linha)
			if err != nil {
				log.Printf("[BROKER] Erro ao interpretar resposta do broker %s: %v | linha: %s", broker, err, linha)
				continue
			}

			if mensagem.Acao == "DESPACHO_OK" {
				remotoEncontrado = true
				algumDespachado = true
				break
			}
		}

		rwmu.Lock() // re-adquire o mutex

		if remotoEncontrado {
			req.Status = "em atendimento"
			continue
		} else {
			heap.Push(&filaRequisicoes, reqRemovida) // re-adiciona a requisição de volta à fila caso não tenha sido despachada para nenhum broker remoto
			break                                    // ninguém tem drone, para de tentar
		}
	}

	return algumDespachado
}
