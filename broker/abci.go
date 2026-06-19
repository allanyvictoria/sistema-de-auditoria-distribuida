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

// Dicionários globais de chaves públicas (Protegidos pelo ecossistema do Broker)
var mapaChavesPublicas = make(map[string]string) // Chaves das Empresas/Navios
var mapaChavesDrones = make(map[string]string)   // Chaves dos Drones Homologados

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
	Empresa      string `json:"empresa"`
	ChavePublica string `json:"chave_publica"`
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
	ChavePublica string `json:"chave_publica"`
	Assinatura   string `json:"assinatura"`
}

type PayloadDespacho struct {
	RequisicaoID string `json:"requisicao_id"`
	DroneID      string `json:"drone_id"`
	BrokerID     string `json:"broker_id"`
}

type PayloadLiberacao struct {
	RequisicaoID string `json:"requisicao_id"`
	Motivo       string `json:"motivo"`
}

type DronesApp struct {
	abcitypes.BaseApplication
	ledger *Ledger
}

var _ abcitypes.Application = (*DronesApp)(nil)

func NovaDronesApp(l *Ledger) *DronesApp {
	return &DronesApp{ledger: l}
}

// CheckTx é a função de validação pré-consenso. Ela é chamada para cada transação recebida, antes de ser incluída em um bloco.
func (app *DronesApp) CheckTx(ctx context.Context, req *abcitypes.RequestCheckTx) (*abcitypes.ResponseCheckTx, error) {
	var pacote PacoteBase
	if err := json.Unmarshal(req.Tx, &pacote); err != nil {
		return &abcitypes.ResponseCheckTx{Code: 1, Log: "JSON malformado"}, nil
	}

	// Para os blocos de DESPACHO e LIBERAÇÃO, a validação é mais leve, pois eles só podem ser originados internamente
	// pelo Broker, e não por transações externas.
	if pacote.Tipo == BlocoDespacho || pacote.Tipo == BlocoLiberacao {
		return &abcitypes.ResponseCheckTx{Code: 0, Log: "Bloco interno aprovado"}, nil
	}

	// Para os blocos de REGISTRO, TRANSACAO, TRANSFERENCIA e LAUDO, a validação é mais rigorosa
	switch pacote.Tipo {
	case BlocoRegistro:
		var tx PayloadRegistro
		json.Unmarshal(pacote.Data, &tx)

		if app.ledger.EmpresaExiste(tx.Empresa) {
			log.Printf("[ABCI - CheckTx] REJEITADO: Empresa %s já está registrada.\n", tx.Empresa)
			return &abcitypes.ResponseCheckTx{Code: 4, Log: "Empresa já registrada"}, nil
		}
		log.Printf("[ABCI - CheckTx] Pedido de registro de %s válido. Indo para consenso.\n", tx.Empresa)

	case BlocoTransacao:
		var tx PayloadTransacao
		if err := json.Unmarshal(pacote.Data, &tx); err != nil {
			return &abcitypes.ResponseCheckTx{Code: 1, Log: "Erro no unmarshal"}, nil
		}

		rwmu.RLock()
		chaveReal := mapaChavesPublicas[tx.Empresa]
		rwmu.RUnlock()

		if chaveReal == "" {
			log.Printf("[ABCI - CheckTx]   BLOQUEADO: Empresa %s não enviou chave no registro!\n", tx.Empresa)
			return &abcitypes.ResponseCheckTx{Code: 3, Log: "Empresa nao registrada na memoria"}, nil
		}

		if chaveReal != tx.ChavePublica {
			log.Printf("[ABCI - CheckTx]   HACKER DETECTADO: Chave pública falsa para %s!\n", tx.Empresa)
			return &abcitypes.ResponseCheckTx{Code: 3, Log: "Chave publica não pertence à empresa"}, nil
		}

		pubKeyBytes, _ := hex.DecodeString(tx.ChavePublica)
		assinaturaBytes, _ := hex.DecodeString(tx.Assinatura) //
		mensagemBruta := fmt.Sprintf("%s:%d:%s:%s", tx.Empresa, tx.Valor, tx.Acao, tx.Timestamp)

		// Verifica a assinatura digital da transação para garantir que ela foi realmente autorizada pela empresa.
		if !ed25519.Verify(pubKeyBytes, []byte(mensagemBruta), assinaturaBytes) {
			log.Printf("[ABCI - CheckTx] FRAUDE DETECTADA: Assinatura inválida para %s.\n", tx.Empresa)
			return &abcitypes.ResponseCheckTx{Code: 3, Log: "Assinatura inválida"}, nil
		}

		// Verifica se a empresa tem saldo suficiente para a transação. Se não tiver, rejeita a transação.
		if !app.ledger.VerificarCreditos(tx.Empresa, tx.Valor) {
			log.Printf("[ABCI - CheckTx] REJEITADO: Empresa %s sem créditos.\n", tx.Empresa)
			return &abcitypes.ResponseCheckTx{Code: 2, Log: "Saldo insuficiente"}, nil
		}
		log.Printf("[ABCI - CheckTx] APROVADO: Assinatura validada e saldo de %s verificado.\n", tx.Empresa)

	case BlocoTransferencia:
		var tx PayloadTransferencia
		if err := json.Unmarshal(pacote.Data, &tx); err != nil {
			return &abcitypes.ResponseCheckTx{Code: 1, Log: "Erro no unmarshal"}, nil
		}

		rwmu.RLock()
		chaveReal := mapaChavesPublicas[tx.Origem]
		rwmu.RUnlock()

		if chaveReal == "" {
			log.Printf("[ABCI - CheckTx]   BLOQUEADO: Empresa %s não enviou chave no registro!\n", tx.Origem)
			return &abcitypes.ResponseCheckTx{Code: 3, Log: "Empresa nao registrada na memoria"}, nil
		}

		if chaveReal != tx.ChavePublica {
			log.Printf("[ABCI - CheckTx]   TENTATIVA DE ROUBO! Chave falsa detectada para %s!\n", tx.Origem)
			return &abcitypes.ResponseCheckTx{Code: 3, Log: "Chave publica não pertence à empresa"}, nil
		}

		pubKeyBytes, _ := hex.DecodeString(tx.ChavePublica)
		assinaturaBytes, _ := hex.DecodeString(tx.Assinatura)
		mensagemBruta := fmt.Sprintf("%s:%s:%d", tx.Origem, tx.Destino, tx.Valor)

		if !ed25519.Verify(pubKeyBytes, []byte(mensagemBruta), assinaturaBytes) {
			log.Printf("[ABCI - CheckTx] FRAUDE DETECTADA: Transferência não autorizada por %s.\n", tx.Origem)
			return &abcitypes.ResponseCheckTx{Code: 3, Log: "Assinatura inválida"}, nil
		}

		if !app.ledger.VerificarCreditos(tx.Origem, tx.Valor) {
			log.Printf("[ABCI - CheckTx] REJEITADO: %s está sem saldo para a transferência.\n", tx.Origem)
			return &abcitypes.ResponseCheckTx{Code: 2, Log: "Saldo insuficiente"}, nil
		}

	case BlocoLaudo:
		var tx PayloadLaudo
		if err := json.Unmarshal(pacote.Data, &tx); err != nil {
			return &abcitypes.ResponseCheckTx{Code: 1, Log: "Erro no unmarshal"}, nil
		}

		// TRAVA CONTRA HACKER: O Drone existe no sistema e a chave bate?
		rwmu.RLock()
		chaveRealDrone := mapaChavesDrones[tx.DroneID]
		rwmu.RUnlock()

		if chaveRealDrone == "" {
			log.Printf("[ABCI - CheckTx]   BLOQUEADO: Drone %s não está homologado nesta sessão!\n", tx.DroneID)
			return &abcitypes.ResponseCheckTx{Code: 3, Log: "Drone nao homologado na rede"}, nil
		}

		if chaveRealDrone != tx.ChavePublica {
			log.Printf("[ABCI - CheckTx]   FRAUDE: Chave pública falsa para o Drone %s!\n", tx.DroneID)
			return &abcitypes.ResponseCheckTx{Code: 3, Log: "Chave publica nao pertence ao drone"}, nil
		}

		// Verifica a assinatura matemática do Laudo (RequisicaoID:DroneID:Timestamp)
		pubKeyBytes, _ := hex.DecodeString(tx.ChavePublica)
		assinaturaBytes, _ := hex.DecodeString(tx.Assinatura)
		mensagemBruta := fmt.Sprintf("%s:%s:%s", tx.RequisicaoID, tx.DroneID, tx.Timestamp)

		if !ed25519.Verify(pubKeyBytes, []byte(mensagemBruta), assinaturaBytes) {
			log.Printf("[ABCI - CheckTx]   FRAUDE: Assinatura do laudo corrompida para o Drone %s.\n", tx.DroneID)
			return &abcitypes.ResponseCheckTx{Code: 3, Log: "Assinatura do laudo invalida"}, nil
		}

		log.Printf("[ABCI - CheckTx] Laudo do Drone %s verificado com sucesso. Indo para consenso.\n", tx.DroneID)

	case BlocoDespacho:
		var tx PayloadDespacho
		if err := json.Unmarshal(pacote.Data, &tx); err != nil {
			return &abcitypes.ResponseCheckTx{Code: 1, Log: "Erro no unmarshal"}, nil
		}

		rwmu.RLock()
		req, existe := mapaRequisicoes[tx.RequisicaoID]
		jaAtendida := existe && req.Status != "pendente" && req.Status != "reservado"
		rwmu.RUnlock()

		if !existe {
			return &abcitypes.ResponseCheckTx{Code: 5, Log: "Requisição desconhecida"}, nil
		}
		if jaAtendida {
			return &abcitypes.ResponseCheckTx{Code: 6, Log: "Requisição já está em atendimento"}, nil
		}

	case BlocoLiberacao:
		var tx PayloadLiberacao
		if err := json.Unmarshal(pacote.Data, &tx); err != nil {
			return &abcitypes.ResponseCheckTx{Code: 1, Log: "Erro no unmarshal"}, nil
		}

		rwmu.RLock()
		req, existe := mapaRequisicoes[tx.RequisicaoID]
		rwmu.RUnlock()

		if !existe {
			return &abcitypes.ResponseCheckTx{Code: 5, Log: "Requisição desconhecida"}, nil
		}
		if req.Status != "em atendimento" {
			return &abcitypes.ResponseCheckTx{Code: 7, Log: "Requisição não está em atendimento"}, nil
		}
	}

	return &abcitypes.ResponseCheckTx{Code: 0}, nil
}

// FinalizeBlock é a função de execução pós-consenso. Ela é chamada para cada bloco que foi
// acordado pelo consenso, e é onde as transações são realmente aplicadas ao estado do aplicativo.
func (app *DronesApp) FinalizeBlock(ctx context.Context, req *abcitypes.RequestFinalizeBlock) (*abcitypes.ResponseFinalizeBlock, error) {
	txResults := make([]*abcitypes.ExecTxResult, len(req.Txs))

	for i, txBytes := range req.Txs {
		var pacote PacoteBase
		json.Unmarshal(txBytes, &pacote)

		// O processamento de cada tipo de bloco é feito de acordo com a sua natureza e regras de negócio.
		switch pacote.Tipo {
		case BlocoRegistro:
			var tx PayloadRegistro
			json.Unmarshal(pacote.Data, &tx)

			rwmu.Lock()
			mapaChavesPublicas[tx.Empresa] = tx.ChavePublica
			rwmu.Unlock()

			app.ledger.RegistrarEmpresa(tx.Empresa, 100)
			fmt.Printf("[BLOCKCHAIN] Registro: %s entrou. Chave real gravada: [%s]\n", tx.Empresa, tx.ChavePublica)

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
			// Atualiza o estado da missão na blockchain para concluída
			if req, existe := mapaRequisicoes[laudo.RequisicaoID]; existe {
				req.Status = "concluida"
				fmt.Printf("[BLOCKCHAIN]   REQUISIÇÃO %s ATUALIZADA PARA 'concluida'.\n", laudo.RequisicaoID)
			}

			// Protege contra ponteiros nulos caso o mapa de conexões TCP não esteja acessível aqui
			if mapaDrones != nil {
				if drone, ok := mapaDrones[laudo.DroneID]; ok {
					drone.Disponivel = true
					drone.RequisicaoAtual = ""
				}
			}

			fmt.Printf("[BLOCKCHAIN] LAUDO CONFIRMADO E GRAVADO EM BLOCO | Drone: %s\n", laudo.DroneID)
			despacharDrone()
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

				for idx, r := range filaRequisicoes {
					if r.ID == tx.RequisicaoID {
						heap.Remove(&filaRequisicoes, idx)
						break
					}
				}

				if tx.BrokerID == brokerID {
					if drone, ok := mapaDrones[tx.DroneID]; ok {
						drone.Conn.Write([]byte(fmt.Sprintf("BROKER;%s;MISSAO;%s\n", brokerID, req.ID)))
						fmt.Printf("[FILA] Despacho confirmado: Req %s -> Drone %s\n", req.ID, tx.DroneID)
					}
				}
			} else if existe && tx.BrokerID == brokerID {
				if drone, ok := mapaDrones[tx.DroneID]; ok && drone.RequisicaoAtual == tx.RequisicaoID {
					drone.Disponivel = true
					drone.RequisicaoAtual = ""
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
				heap.Push(&filaRequisicoes, req)
				fmt.Printf("[FILA] Req %s devolvida à fila.\n", req.ID)
				despacharDrone()
			}
			rwmu.Unlock()
		}
		txResults[i] = &abcitypes.ExecTxResult{Code: 0}
	}

	return &abcitypes.ResponseFinalizeBlock{TxResults: txResults}, nil
}

// Função para responder a consultas de leitura (apenas para depuração, não usada na lógica principal)
func (app *DronesApp) Query(ctx context.Context, req *abcitypes.RequestQuery) (*abcitypes.ResponseQuery, error) {
	if req.Path == "estado_memoria" {
		rwmu.RLock()
		defer rwmu.RUnlock()

		resumoReqs := make(map[string]string)
		for id, r := range mapaRequisicoes {
			resumoReqs[id] = fmt.Sprintf("Status: %s | Drone: %s", r.Status, r.DroneID)
		}

		estadoAtual := map[string]interface{}{
			"1_empresas_homologadas": mapaChavesPublicas,
			"2_drones_homologados":   mapaChavesDrones,
			"3_missoes_ativas":       resumoReqs,
		}

		estadoBytes, _ := json.Marshal(estadoAtual)
		return &abcitypes.ResponseQuery{Code: 0, Value: estadoBytes}, nil
	}

	return &abcitypes.ResponseQuery{Code: 1, Log: "Caminho de consulta não encontrado"}, nil
}
