package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	abcitypes "github.com/cometbft/cometbft/abci/types"
)

// Estruturas genéricas para os payloads que vão trafegar na rede
type TipoBloco string

const (
	BlocoTransacao TipoBloco = "TRANSACAO"
	BlocoLaudo     TipoBloco = "LAUDO"
)

// PacoteBase é o envelope que o CometBFT vai enviar pra gente
type PacoteBase struct {
	Tipo TipoBloco       `json:"tipo"`
	Data json.RawMessage `json:"data"`
}

type PayloadTransacao struct {
	Empresa     string `json:"empresa"`
	Valor       int    `json:"valor"`
	Criticidade string `json:"criticidade"`
	Acao        string `json:"acao"`
}

type PayloadLaudo struct {
	DroneID string `json:"drone_id"`
	Log     string `json:"log"`
}

// =====================================================================
// A MÁGICA ACONTECE AQUI: Incorporar abcitypes.BaseApplication
// Isso resolve TODOS os erros de "missing method" e "undefined"!
// =====================================================================
type DronesApp struct {
	abcitypes.BaseApplication
	ledger *Ledger
}

var _ abcitypes.Application = (*DronesApp)(nil)

func NovaDronesApp(l *Ledger) *DronesApp {
	return &DronesApp{ledger: l}
}

// =====================================================================
// 1. A PORTA DE ENTRADA (Validação Prévia)
// =====================================================================
func (app *DronesApp) CheckTx(ctx context.Context, req *abcitypes.RequestCheckTx) (*abcitypes.ResponseCheckTx, error) {
	var pacote PacoteBase
	if err := json.Unmarshal(req.Tx, &pacote); err != nil {
		return &abcitypes.ResponseCheckTx{Code: 1, Log: "JSON malformado"}, nil
	}

	if pacote.Tipo == BlocoTransacao {
		var tx PayloadTransacao
		json.Unmarshal(pacote.Data, &tx)

		temSaldo := app.ledger.VerificarCreditos(tx.Empresa, tx.Valor)
		if !temSaldo {
			log.Printf("[ABCI - CheckTx] ❌ REJEITADO: Empresa %s sem créditos.\n", tx.Empresa)
			return &abcitypes.ResponseCheckTx{Code: 2, Log: "Saldo insuficiente"}, nil
		}
		log.Printf("[ABCI - CheckTx] ✅ APROVADO: Empresa %s tem saldo. Seguindo para consenso.\n", tx.Empresa)
	}

	return &abcitypes.ResponseCheckTx{Code: 0}, nil
}

// =====================================================================
// 2. A EXECUÇÃO DEFINITIVA (O Consenso foi Atingido!)
// =====================================================================
func (app *DronesApp) FinalizeBlock(ctx context.Context, req *abcitypes.RequestFinalizeBlock) (*abcitypes.ResponseFinalizeBlock, error) {

	txResults := make([]*abcitypes.ExecTxResult, len(req.Txs))

	for i, txBytes := range req.Txs {
		var pacote PacoteBase
		json.Unmarshal(txBytes, &pacote)

		switch pacote.Tipo {

		case BlocoTransacao:
			var tx PayloadTransacao
			json.Unmarshal(pacote.Data, &tx)

			// CORREÇÃO DO ERRO "cannot slice": Código puro, sem colchetes!
			err := app.ledger.DebitarCreditos(tx.Empresa, tx.Valor)

			if err == nil {
				fmt.Printf("[BLOCKCHAIN] 💰 Débito confirmado! Empresa: %s | Valor: %d\n", tx.Empresa, tx.Valor)

				// Integração com o seu Problema 2: Coloca na Heap e despacha
				rwmu.Lock()
				novaReq := &Requisicao{
					ID:          fmt.Sprintf("%s-%d", tx.Empresa, time.Now().UnixNano()),
					Tipo:        tx.Acao,
					Criticidade: tx.Criticidade,
					Prioridade:  definirPrioridade(tx.Criticidade),
					Timestamp:   time.Now(),
					Status:      "pendente",
				}
				mapaRequisicoes[novaReq.ID] = novaReq
				filaRequisicoes.Push(novaReq)
				fmt.Printf("[FILA] ENTROU VIA CONSENSO: Req %s | Tamanho: %d\n", novaReq.ID, filaRequisicoes.Len())

				despacharDrone()
				rwmu.Unlock()
			} else {
				fmt.Printf("[BLOCKCHAIN] ❌ Erro ao debitar: %v\n", err)
			}

		case BlocoLaudo:
			var laudo PayloadLaudo
			json.Unmarshal(pacote.Data, &laudo)
			fmt.Printf("[BLOCKCHAIN] 📜 Laudo Imutável Registrado! Drone %s: %s\n", laudo.DroneID, laudo.Log)
		}

		txResults[i] = &abcitypes.ExecTxResult{Code: 0}
	}

	return &abcitypes.ResponseFinalizeBlock{
		TxResults: txResults,
	}, nil
}
