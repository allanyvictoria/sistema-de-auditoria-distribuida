package main

import (
	"container/heap"
	"fmt"
	"time"
)

const (
	PrioridadeNormal = 1
	PrioridadeMedia  = 2
	PrioridadeAlta   = 3
)

// Estrutura para representar uma requisição
type Requisicao struct {
	ID           string    // id da requisição, gerada pelo sensor
	Tipo         string    // tipo de requisição (ex: "deriva", "congestionamento", etc)
	Criticidade  string    // criticidade da requisição (ex: "baixa", "media", "alta")
	Prioridade   int       // prioridade da requisição (ex: 1, 2, 3)
	Timestamp    time.Time // timestamp de criação da requisição
	Status       string    // status da requisição (ex: "pendente", "em_atendimento", "concluida")
	DroneID      string    // id do drone atribuído à requisição
	BrokerOrigem string    // id do broker de origem da requisição
}

type FilaRequisicoes []*Requisicao // tipo que implementa heap.Interface para ser usado como fila de prioridade

// ******************************************OS 5 Métodos da heap***********************************************
// Retorna o tamanho da fila
func (f FilaRequisicoes) Len() int {
	return len(f)
}

// qual tem maior prioridade
func (f FilaRequisicoes) Less(i, j int) bool {
	if f[i].Prioridade > f[j].Prioridade {
		return true
	} else if f[i].Prioridade == f[j].Prioridade {
		return f[i].Timestamp.Before(f[j].Timestamp) // se tiver mesma prioridade, quem chegou primeiro tem preferência
	}
	return false
}

// troca dois elementos
func (f FilaRequisicoes) Swap(i, j int) {
	f[i], f[j] = f[j], f[i]
}

// adiciona elemento
func (f *FilaRequisicoes) Push(x any) {
	requisicao := x.(*Requisicao)
	*f = append(*f, requisicao)

}

// remove e retorna o de maior prioridade
func (f *FilaRequisicoes) Pop() any {
	n := len(*f)
	item := (*f)[n-1]
	*f = (*f)[:n-1]

	return item
}

// função para aumentar a prioridade de uma requisição (aging)
func aging(r *Requisicao) {
	switch r.Prioridade {
	case 1:
		r.Prioridade = 2
	case 2:
		r.Prioridade = 3
	}
}

// função para verificar periodicamente as requisições na fila e aplicar aging
func iniciarAging(fila *FilaRequisicoes) {
	for {
		time.Sleep(10 * time.Second) // verifica a cada 10 segundos

		rwmu.Lock()

		houveAlteracao := false

		// Percorre a fila e aplica aging nas requisições pendentes que estão há mais de 30 segundos sem atendimento
		for _, requisicao := range *fila {
			if requisicao.Status == "pendente" {
				if time.Since(requisicao.Timestamp) > 30*time.Second {
					if requisicao.Prioridade < PrioridadeAlta {
						aging(requisicao)
						houveAlteracao = true
					}
				}
			}
		}

		// Reorganiza a heap APENAS se alguma prioridade realmente subiu, poupando processamento
		if houveAlteracao {
			heap.Init(fila)
		}

		// Se sobrou alguém pendente na fila, tenta despachar de novo
		if fila.Len() > 0 {

			droneEnviado := despacharDrone()
			if houveAlteracao {
				if !droneEnviado {
					// Caiu aqui porque tentou despachar e não tinha drone
					fmt.Printf("[BROKER] Verificação concluída. %d requisições pendentes. Nenhum drone disponível.\n", fila.Len())
				} else {
					// Opcional: Um log de sucesso caso o drone tenha ido
					fmt.Printf("[BROKER] Sucesso! Drone despachado. %d requisições restantes na fila.\n", fila.Len()-1)
				} // isso funciona como um "heartbeat" de despacho.
			}
		}

		rwmu.Unlock()

	}

}
