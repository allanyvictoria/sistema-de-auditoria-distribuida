package main

import (
	"errors"
	"log"
	"sync"
)

// Ledger gerencia o saldo de créditos das empresas/navios de forma thread-safe
type Ledger struct {
	Creditos map[string]int
	mu       sync.RWMutex
}

// NovoLedger inicializa a estrutura.
// DICA: Para facilitar a apresentação no laboratório, já deixamos algumas
// empresas iniciadas com saldo para evitar ter que codificar uma rota de "recarga" agora.
func NovoLedger() *Ledger {
	return &Ledger{
		Creditos: map[string]int{
			"Navio_A": 100,
			"Navio_B": 50,
			"Navio_C": 10,
		},
	}
}

// VerificarCreditos checa se há saldo suficiente SEM debitar[cite: 145].
// ATENÇÃO: Esta função será chamada pela rede P2P (via CheckTx) apenas para
// perguntar: "Posso aprovar a entrada dessa missão no bloco?".
func (l *Ledger) VerificarCreditos(empresa string, valor int) bool {
	l.mu.RLock() // RLock permite múltiplas leituras simultâneas, otimizando performance
	defer l.mu.RUnlock()

	saldo, existe := l.Creditos[empresa]
	if !existe {
		return false // Empresa desconhecida, barra a transação
	}
	return saldo >= valor
}

// DebitarCreditos desconta o valor do saldo da empresa[cite: 145].
// REGRA DE OURO: Esta função NUNCA deve ser chamada diretamente pelo sensor.
// Ela só será acionada pelo DeliverTx (o FSM), APÓS o CometBFT confirmar
// que a rede inteira aprovou o bloco para evitar fraude de duplo gasto[cite: 146].
func (l *Ledger) DebitarCreditos(empresa string, valor int) error {
	l.mu.Lock() // Lock exclusivo para escrita
	defer l.mu.Unlock()

	saldo, existe := l.Creditos[empresa]
	if !existe {
		return errors.New("empresa não encontrada no ledger")
	}
	if saldo < valor {
		return errors.New("saldo insuficiente para realizar o débito")
	}

	l.Creditos[empresa] -= valor
	return nil
}

// CreditarCreditos adiciona fundos a uma empresa[cite: 145].
// Pode ser útil caso você queira criar uma funcionalidade de recarga no futuro.
func (l *Ledger) CreditarCreditos(empresa string, valor int) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.Creditos[empresa] += valor
}

func (l *Ledger) RegistrarEmpresa(empresa string, saldoInicial int) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if _, existe := l.Creditos[empresa]; !existe {
		l.Creditos[empresa] = saldoInicial
		log.Printf("[LEDGER] Nova empresa registrada: %s | Saldo: %d", empresa, saldoInicial)
	}
}
