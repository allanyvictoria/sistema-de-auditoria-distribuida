package main

import (
	"container/heap"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"time"

	abcitypes "github.com/cometbft/cometbft/abci/types"
)

type TipoBloco string

const (
	BlocoRegistro      TipoBloco = "REGISTRO"
	BlocoTransacao     TipoBloco = "TRANSACAO"
	BlocoTransferencia TipoBloco = "TRANSFERENCIA"
	BlocoLaudo         TipoBloco = "LAUDO"
	BlocoDespacho      TipoBloco = "DESPACHO"
	BlocoLiberacao     TipoBloco = "LIBERACAO"
)

type PacoteBase struct {
	Tipo TipoBloco       `json:"tipo"`
	Data json.RawMessage `json:"data"`
}

type PayloadRegistro struct {
	Empresa string `json:"empresa"`
}

type PayloadTransacao struct {
	Empresa      string `json:"empresa"`
	Valor        int    `json:"valor"`
	Criticidade  string `json:"criticidade"`
	Acao         string `json:"acao"`
	Timestamp    string `json:"timestamp"`
	ChavePublica string `json:"chave_publica"`
	Assinatura   string `json:"assinatura"`
}

type PayloadTransferencia struct {
	Origem       string `json:"origem"`
	Destino      string `json:"destino"`
	Valor        int    `json:"valor"`
	ChavePublica string `json:"chave_publica"`
	Assinatura   string `json:"assinatura"`
}

type PayloadLaudo struct {
	RequisicaoID string `json:"requisicao_id"`
	DroneID      string `json:"drone_id"`
	Log          string `json:"log"`
	Rota         string `json:"rota"`
	Timestamp    string `json:"timestamp"`
}

// PayloadDespacho anuncia, via consenso, que uma requisição pendente foi
// atribuída a um drone de um broker específico. Todas as réplicas aplicam
// essa marcação em FinalizeBlock, evitando despacho duplicado entre brokers.
type PayloadDespacho struct {
	RequisicaoID string `json:"requisicao_id"`
	DroneID      string `json:"drone_id"`
	BrokerID     string `json:"broker_id"`
}

// PayloadLiberacao anuncia, via consenso, que uma requisição "em atendimento"
// deve voltar a ser "pendente" — usado quando o drone responsável caiu, ou
// quando o broker responsável pelo despacho caiu e o drone reconectou em
// outro broker sem ter concluído a missão. Todas as réplicas aplicam essa
// marcação em FinalizeBlock, devolvendo a requisição para a fila.
type PayloadLiberacao struct {
	RequisicaoID string `json:"requisicao_id"`
	Motivo       string `json:"motivo"` // ex: "drone_caiu", "broker_caiu_drone_reconectou"
}

type DronesApp struct {
	abcitypes.BaseApplication
	ledger *Ledger
}

var _ abcitypes.Application = (*DronesApp)(nil)

func NovaDronesApp(l *Ledger) *DronesApp {
	return &DronesApp{ledger: l}
}

func (app *DronesApp) CheckTx(ctx context.Context, req *abcitypes.RequestCheckTx) (*abcitypes.ResponseCheckTx, error) {
	var pacote PacoteBase
	if err := json.Unmarshal(req.Tx, &pacote); err != nil {
		return &abcitypes.ResponseCheckTx{Code: 1, Log: "JSON malformado"}, nil
	}

	// ---> PASSAGEM VIP PARA BLOCOS INTERNOS DO BROKER <---
	// Como estes blocos são gerados pelo próprio código do broker (que é confiável),
	// não precisamos validar assinatura de cliente aqui.
	if pacote.Tipo == BlocoDespacho || pacote.Tipo == BlocoLiberacao || pacote.Tipo == BlocoLaudo {
		return &abcitypes.ResponseCheckTx{Code: 0, Log: "Bloco interno aprovado"}, nil
	}

	switch pacote.Tipo {
	case BlocoRegistro:
		var tx PayloadRegistro
		json.Unmarshal(pacote.Data, &tx)

		// Trava de segurança: Se já existe, rejeita o bloco!
		if app.ledger.EmpresaExiste(tx.Empresa) {
			log.Printf("[ABCI - CheckTx] REJEITADO: Empresa %s já está registrada. Sem créditos extras!\n", tx.Empresa)
			return &abcitypes.ResponseCheckTx{Code: 4, Log: "Empresa já registrada"}, nil
		}
		log.Printf("[ABCI - CheckTx] Pedido de registro de %s válido. Indo para consenso.\n", tx.Empresa)
	case BlocoTransacao:
		var tx PayloadTransacao
		if err := json.Unmarshal(pacote.Data, &tx); err != nil {
			return &abcitypes.ResponseCheckTx{Code: 1, Log: "Erro no unmarshal"}, nil
		}

		pubKeyBytes, _ := hex.DecodeString(tx.ChavePublica)
		assinaturaBytes, _ := hex.DecodeString(tx.Assinatura)
		mensagemBruta := fmt.Sprintf("%s:%d:%s:%s", tx.Empresa, tx.Valor, tx.Acao, tx.Timestamp)

		if !ed25519.Verify(pubKeyBytes, []byte(mensagemBruta), assinaturaBytes) {
			log.Printf("[ABCI - CheckTx] FRAUDE DETECTADA: Assinatura inválida para %s.\n", tx.Empresa)
			return &abcitypes.ResponseCheckTx{Code: 3, Log: "Assinatura inválida"}, nil
		}

		if !app.ledger.VerificarCreditos(tx.Empresa, tx.Valor) {
			log.Printf("[ABCI - CheckTx] REJEITADO: Empresa %s sem créditos.\n", tx.Empresa)
			return &abcitypes.ResponseCheckTx{Code: 2, Log: "Saldo insuficiente"}, nil
		}
		log.Printf("[ABCI - CheckTx] APROVADO: Assinatura validada e saldo de %s verificado. Indo para consenso!\n", tx.Empresa)

	case BlocoTransferencia:
		var tx PayloadTransferencia
		if err := json.Unmarshal(pacote.Data, &tx); err != nil {
			return &abcitypes.ResponseCheckTx{Code: 1, Log: "Erro no unmarshal"}, nil
		}

		pubKeyBytes, _ := hex.DecodeString(tx.ChavePublica)
		assinaturaBytes, _ := hex.DecodeString(tx.Assinatura)
		mensagemBruta := fmt.Sprintf("%s:%s:%d", tx.Origem, tx.Destino, tx.Valor)

		if !ed25519.Verify(pubKeyBytes, []byte(mensagemBruta), assinaturaBytes) {
			log.Printf("[ABCI - CheckTx] FRAUDE DETECTADA: Transferência não autorizada por %s.\n", tx.Origem)
			return &abcitypes.ResponseCheckTx{Code: 3, Log: "Assinatura inválida"}, nil
		}

		if !app.ledger.VerificarCreditos(tx.Origem, tx.Valor) {
			return &abcitypes.ResponseCheckTx{Code: 2, Log: "Saldo insuficiente"}, nil
		}

	case BlocoDespacho:
		var tx PayloadDespacho
		if err := json.Unmarshal(pacote.Data, &tx); err != nil {
			return &abcitypes.ResponseCheckTx{Code: 1, Log: "Erro no unmarshal"}, nil
		}

		rwmu.Lock()
		req, existe := mapaRequisicoes[tx.RequisicaoID]
		jaAtendida := existe && req.Status != "pendente" && req.Status != "reservado"
		rwmu.Unlock()

		if !existe {
			return &abcitypes.ResponseCheckTx{Code: 5, Log: "Requisição desconhecida"}, nil
		}
		if jaAtendida {
			// Outro broker já reservou essa requisição — rejeita antes do consenso.
			return &abcitypes.ResponseCheckTx{Code: 6, Log: "Requisição já está em atendimento"}, nil
		}

	case BlocoLiberacao:
		var tx PayloadLiberacao
		if err := json.Unmarshal(pacote.Data, &tx); err != nil {
			return &abcitypes.ResponseCheckTx{Code: 1, Log: "Erro no unmarshal"}, nil
		}

		rwmu.Lock()
		req, existe := mapaRequisicoes[tx.RequisicaoID]
		rwmu.Unlock()

		if !existe {
			return &abcitypes.ResponseCheckTx{Code: 5, Log: "Requisição desconhecida"}, nil
		}
		if req.Status != "em atendimento" {
			// Já liberada (por outra réplica) ou já concluída — rejeita
			// antes do consenso para evitar reprocessamento duplicado.
			return &abcitypes.ResponseCheckTx{Code: 7, Log: "Requisição não está em atendimento"}, nil
		}
	}

	return &abcitypes.ResponseCheckTx{Code: 0}, nil
}

func (app *DronesApp) FinalizeBlock(ctx context.Context, req *abcitypes.RequestFinalizeBlock) (*abcitypes.ResponseFinalizeBlock, error) {
	txResults := make([]*abcitypes.ExecTxResult, len(req.Txs))

	for i, txBytes := range req.Txs {
		var pacote PacoteBase
		json.Unmarshal(txBytes, &pacote)

		switch pacote.Tipo {
		case BlocoRegistro:
			var tx PayloadRegistro
			json.Unmarshal(pacote.Data, &tx)

			// Grava a empresa com exatos 100 créditos no ledger
			app.ledger.RegistrarEmpresa(tx.Empresa, 100)
			fmt.Printf("[BLOCKCHAIN] Registro Confirmado: %s entrou no sistema com 100 créditos.\n", tx.Empresa)
		case BlocoTransacao:
			var tx PayloadTransacao
			json.Unmarshal(pacote.Data, &tx)

			err := app.ledger.DebitarCreditos(tx.Empresa, tx.Valor, fmt.Sprintf("Missão %s (%s)", tx.Acao, tx.Criticidade))
			if err == nil {
				fmt.Printf("[BLOCKCHAIN] Débito confirmado! Empresa: %s | Valor: %d\n", tx.Empresa, tx.Valor)
				novaReq := &Requisicao{
					ID:          fmt.Sprintf("%s-%s", tx.Empresa, tx.Timestamp),
					Tipo:        tx.Acao,
					Criticidade: tx.Criticidade,
					Prioridade:  definirPrioridade(tx.Criticidade),
					Timestamp:   time.Now(),
					Status:      "pendente",
				}

				rwmu.Lock()

				mapaRequisicoes[novaReq.ID] = novaReq
				heap.Push(&filaRequisicoes, novaReq)
				fmt.Printf("[FILA] Inserida pós-consenso: Req %s | Tamanho atual da Fila: %d\n", novaReq.ID, filaRequisicoes.Len())

				despacharDrone()
				rwmu.Unlock()
			}

		case BlocoTransferencia:
			var tx PayloadTransferencia
			json.Unmarshal(pacote.Data, &tx)

			rwmu.Lock()
			if err := app.ledger.DebitarCreditos(tx.Origem, tx.Valor, fmt.Sprintf("Transferência para %s", tx.Destino)); err == nil {
				app.ledger.CreditarCreditos(tx.Destino, tx.Valor, "TRANSFERENCIA_RECEBE", fmt.Sprintf("Recebido de %s", tx.Origem))
				fmt.Printf("[BLOCKCHAIN] Transferência: %s enviou %d para %s\n", tx.Origem, tx.Valor, tx.Destino)
			}
			rwmu.Unlock()

		case BlocoLaudo:
			var laudo PayloadLaudo
			json.Unmarshal(pacote.Data, &laudo)

			rwmu.Lock()
			// 1. Marca a requisição como concluída
			if req, existe := mapaRequisicoes[laudo.RequisicaoID]; existe {
				req.Status = "concluida"
			}

			// 2. Libera o drone para ele poder pegar a próxima missão!
			if drone, ok := mapaDrones[laudo.DroneID]; ok {
				drone.Disponivel = true
				drone.RequisicaoAtual = ""
			}

			fmt.Printf("[BLOCKCHAIN] LAUDO REGISTRADO | Drone: %s | Rota: %s | Time: %s\n", laudo.DroneID, laudo.Rota, laudo.Timestamp)

			// 3. O despacharDrone entra AQUI, antes de soltar o cadeado!
			despacharDrone()

			// 4. Agora sim, missão finalizada, liberamos a memória
			rwmu.Unlock()

		case BlocoDespacho:
			var tx PayloadDespacho
			json.Unmarshal(pacote.Data, &tx)

			rwmu.Lock()
			req, existe := mapaRequisicoes[tx.RequisicaoID]
			if existe && (req.Status == "pendente" || req.Status == "reservado") {
				req.Status = "em atendimento"
				req.DroneID = tx.DroneID
				req.BrokerOrigem = tx.BrokerID

				// Remove a requisição da heap local (em todas as réplicas,
				// pois cada uma tem a mesma requisição na própria fila).
				for idx, r := range filaRequisicoes {
					if r.ID == tx.RequisicaoID {
						heap.Remove(&filaRequisicoes, idx)
						break
					}
				}

				if tx.BrokerID == brokerID {
					// Esta réplica é a que tem o drone: confirma o despacho na conexão real.
					if drone, ok := mapaDrones[tx.DroneID]; ok {
						drone.Conn.Write([]byte(fmt.Sprintf("BROKER;%s;MISSAO;%s\n", brokerID, req.ID)))
						fmt.Printf("[FILA] Despacho confirmado via consenso: Req %s -> Drone %s\n", req.ID, tx.DroneID)
					}
				} else {
					fmt.Printf("[FILA] Req %s atribuída ao broker %s (drone %s)\n", req.ID, tx.BrokerID, tx.DroneID)
				}
			} else if existe && tx.BrokerID == brokerID {
				// Esta réplica reservou o drone para essa requisição, mas
				// outro broker venceu a corrida pelo consenso. Libera o drone.
				if drone, ok := mapaDrones[tx.DroneID]; ok && drone.RequisicaoAtual == tx.RequisicaoID {
					drone.Disponivel = true
					drone.RequisicaoAtual = ""
					fmt.Printf("[FILA] Despacho de Req %s perdeu a corrida de consenso — Drone %s liberado novamente.\n", tx.RequisicaoID, tx.DroneID)
				}
			}
			rwmu.Unlock()

		case BlocoLiberacao:
			var tx PayloadLiberacao
			json.Unmarshal(pacote.Data, &tx)

			rwmu.Lock()
			req, existe := mapaRequisicoes[tx.RequisicaoID]
			if existe && req.Status == "em atendimento" {
				req.Status = "pendente"
				req.DroneID = ""
				req.BrokerOrigem = ""

				// Re-adiciona na heap em todas as réplicas
				heap.Push(&filaRequisicoes, req)

				fmt.Printf("[FILA] Req %s liberada (%s) e devolvida à fila | Tamanho atual: %d\n", req.ID, tx.Motivo, filaRequisicoes.Len())

				despacharDrone()
			}
			rwmu.Unlock()
		}
		txResults[i] = &abcitypes.ExecTxResult{Code: 0}
	}

	return &abcitypes.ResponseFinalizeBlock{TxResults: txResults}, nil
}
