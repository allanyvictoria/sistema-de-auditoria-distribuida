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

// Requisicao contém os dados pertinentes à necessidade solicitada por um sensor.
type Requisicao struct {
	ID           string
	Tipo         string
	Criticidade  string
	Prioridade   int
	Timestamp    time.Time
	Status       string
	DroneID      string
	BrokerOrigem string
}

// FilaRequisicoes é um array manipulado como fila de prioridade implementando heap.Interface.
type FilaRequisicoes []*Requisicao

// Len retorna a quantidade de itens na fila.
func (f FilaRequisicoes) Len() int {
	return len(f)
}

// Less determina qual requisição possui maior prioridade na fila.
func (f FilaRequisicoes) Less(i, j int) bool {
	if f[i].Prioridade > f[j].Prioridade {
		return true
	} else if f[i].Prioridade == f[j].Prioridade {
		return f[i].Timestamp.Before(f[j].Timestamp)
	}
	return false
}

// Swap altera as posições de dois itens da fila.
func (f FilaRequisicoes) Swap(i, j int) {
	f[i], f[j] = f[j], f[i]
}

// Push insere um novo item na fila.
func (f *FilaRequisicoes) Push(x any) {
	requisicao := x.(*Requisicao)
	*f = append(*f, requisicao)

}

// Pop retira e retorna o elemento de maior prioridade atual da fila.
func (f *FilaRequisicoes) Pop() any {
	n := len(*f)
	item := (*f)[n-1]
	*f = (*f)[:n-1]

	return item
}

// aging incrementa a prioridade da requisição se ainda não atingiu o topo.
func aging(r *Requisicao) {
	switch r.Prioridade {
	case 1:
		r.Prioridade = 2
	case 2:
		r.Prioridade = 3
	}
}

// iniciarAging monitora ativamente as requisições em fila para prevenir preempção excessiva.
func iniciarAging(fila *FilaRequisicoes) {
	for {
		time.Sleep(10 * time.Second)

		rwmu.Lock()

		houveAlteracao := false

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

		if houveAlteracao {
			heap.Init(fila)
		}

		if fila.Len() > 0 {

			droneEnviado := despacharDrone()
			if houveAlteracao {
				if !droneEnviado {
					fmt.Printf("[BROKER] Verificação concluída. %d requisições pendentes. Nenhum drone disponível.\n", fila.Len())
				} else {
					fmt.Printf("[BROKER] Sucesso! Drone despachado. %d requisições restantes na fila.\n", fila.Len()-1)
				}
			}
		}

		rwmu.Unlock()

	}

}
