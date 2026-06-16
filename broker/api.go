package main

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
)

// iniciarAPI sobe um servidor web leve para demonstrar transparência (exigência do barema)
func iniciarAPI(l *Ledger) {
	// Rota 1: Exibe o saldo atual consolidado das companhias (derivado do histórico)
	http.HandleFunc("/saldos", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(l.SaldosAtuais())
	})

	// Rota 4: Extrato completo (histórico de movimentos) — prova que o saldo
	// é derivado do histórico, e não de uma variável local isolada.
	http.HandleFunc("/extrato", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(l.HistoricoCompleto())
	})

	// Rota 5: Recalcula os saldos do zero a partir do histórico, para auditoria.
	http.HandleFunc("/saldos/recalcular", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(l.RecalcularSaldos())
	})

	// Rota 3: O "Raio-X" de todas as missões e seus status
	http.HandleFunc("/missoes", func(w http.ResponseWriter, r *http.Request) {
		rwmu.Lock()
		defer rwmu.Unlock()

		w.Header().Set("Content-Type", "application/json")

		// Retorna o mapa completo de requisições para vermos o que está pendente,
		// em atendimento ou concluído.
		json.NewEncoder(w).Encode(mapaRequisicoes)
	})

	// Rota 2: Faz um proxy direto para a API do CometBFT provando a imutabilidade dos blocos
	http.HandleFunc("/auditoria", func(w http.ResponseWriter, r *http.Request) {
		cometURL := os.Getenv("COMET_URL")
		if cometURL == "" {
			cometURL = "localhost:26657"
		}

		resp, err := http.Get("http://" + cometURL + "/block")
		if err != nil {
			http.Error(w, "Erro ao acessar o nó da blockchain", http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		w.Header().Set("Content-Type", "application/json")
		io.Copy(w, resp.Body)
	})

	// Roda o servidor na porta 8080 (Lembre-se de expor essa porta no docker-compose.yml)
	go http.ListenAndServe(":8080", nil)
}
