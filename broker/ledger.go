package main

import (
	"errors"
	"log"
	"sync"
	"time"
)

// Movimento representa um registro no histórico de transações.
type Movimento struct {
	Empresa   string    `json:"empresa"`
	Delta     int       `json:"delta"`
	Tipo      string    `json:"tipo"`
	Detalhe   string    `json:"detalhe"`
	Timestamp time.Time `json:"timestamp"`
}

// Ledger gerencia o histórico de transações e saldos.
type Ledger struct {
	Historico  []Movimento
	saldoCache map[string]int
	mu         sync.RWMutex
}

// NovoLedger inicializa uma nova instância de Ledger.
func NovoLedger() *Ledger {
	return &Ledger{
		Historico:  make([]Movimento, 0),
		saldoCache: make(map[string]int),
	}
}

// registrarMovimento registra uma transação no histórico e atualiza o saldo da empresa.
func (l *Ledger) registrarMovimento(empresa string, delta int, tipo, detalhe string) {
	l.Historico = append(l.Historico, Movimento{
		Empresa:   empresa,
		Delta:     delta,
		Tipo:      tipo,
		Detalhe:   detalhe,
		Timestamp: time.Now(),
	})
	l.saldoCache[empresa] += delta
}

// VerificarCreditos retorna verdadeiro caso o saldo seja suficiente para a operação.
func (l *Ledger) VerificarCreditos(empresa string, valor int) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()

	saldo, existe := l.saldoCache[empresa]
	if !existe {
		return false
	}
	return saldo >= valor
}

// DebitarCreditos desconta o valor especificado da conta da empresa.
func (l *Ledger) DebitarCreditos(empresa string, valor int, detalhe string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	saldo, existe := l.saldoCache[empresa]
	if !existe {
		return errors.New("empresa não encontrada no ledger")
	}
	if saldo < valor {
		return errors.New("saldo insuficiente para realizar o débito")
	}

	l.registrarMovimento(empresa, -valor, "DEBITO", detalhe)
	return nil
}

// CreditarCreditos credita o valor na conta da empresa e registra a movimentação.
func (l *Ledger) CreditarCreditos(empresa string, valor int, tipo, detalhe string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.registrarMovimento(empresa, valor, tipo, detalhe)
}

// RegistrarEmpresa inclui uma nova empresa no ledger com saldo inicial.
func (l *Ledger) RegistrarEmpresa(empresa string, saldoInicial int) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if _, existe := l.saldoCache[empresa]; existe {
		return
	}

	l.registrarMovimento(empresa, saldoInicial, "REGISTRO", "Gênese / créditos iniciais")
	log.Printf("[LEDGER] Nova empresa registrada: %s | Saldo inicial: %d", empresa, saldoInicial)
}

// EmpresaExiste verifica a presença da empresa no sistema.
func (l *Ledger) EmpresaExiste(empresa string) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	_, existe := l.saldoCache[empresa]
	return existe
}

// ConsultarSaldo recupera o saldo da empresa informada.
func (l *Ledger) ConsultarSaldo(empresa string) (int, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	saldo, existe := l.saldoCache[empresa]
	return saldo, existe
}

// SaldosAtuais retorna um mapa consolidado de saldos.
func (l *Ledger) SaldosAtuais() map[string]int {
	l.mu.RLock()
	defer l.mu.RUnlock()

	copia := make(map[string]int, len(l.saldoCache))
	for k, v := range l.saldoCache {
		copia[k] = v
	}
	return copia
}

// HistoricoCompleto retorna os dados inteiros do histórico do ledger.
func (l *Ledger) HistoricoCompleto() []Movimento {
	l.mu.RLock()
	defer l.mu.RUnlock()

	copia := make([]Movimento, len(l.Historico))
	copy(copia, l.Historico)
	return copia
}

// RecalcularSaldos processa e valida todos os registros do histórico.
func (l *Ledger) RecalcularSaldos() map[string]int {
	l.mu.RLock()
	defer l.mu.RUnlock()

	recalculado := make(map[string]int)
	for _, mov := range l.Historico {
		recalculado[mov.Empresa] += mov.Delta
	}
	return recalculado
}
