package main

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
)

// iniciarAPI inicializa um servidor web para expor os dados do sistema.
func iniciarAPI(l *Ledger) {
	// Rota para exibir o saldo atual consolidado.
	http.HandleFunc("/saldos", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(l.SaldosAtuais())
	})

	// Rota para extrair o histórico completo de movimentos.
	http.HandleFunc("/extrato", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(l.HistoricoCompleto())
	})

	// Rota para forçar o recálculo dos saldos a partir do histórico.
	http.HandleFunc("/saldos/recalcular", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(l.RecalcularSaldos())
	})

	// Rota para listar o estado atual de todas as requisições.
	http.HandleFunc("/missoes", func(w http.ResponseWriter, r *http.Request) {
		rwmu.Lock()
		defer rwmu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mapaRequisicoes)
	})

	// Rota de auditoria realizando proxy direto para a API do CometBFT.
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

	go http.ListenAndServe(":8080", nil)
}
