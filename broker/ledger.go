package main

import (
	"errors"
	"log"
	"sync"
	"time"
)

// Movimento representa uma entrada imutável no histórico de créditos.
// É a unidade básica do "extrato" de cada empresa — o saldo é sempre
// a soma de todos os Movimentos daquela empresa.
type Movimento struct {
	Empresa   string    `json:"empresa"`
	Delta     int       `json:"delta"`   // positivo = crédito, negativo = débito
	Tipo      string    `json:"tipo"`    // "REGISTRO", "DEBITO", "TRANSFERENCIA_ENVIO", "TRANSFERENCIA_RECEBE"
	Detalhe   string    `json:"detalhe"` // ex: ID da requisição, ou empresa de origem/destino
	Timestamp time.Time `json:"timestamp"`
}

// Ledger gerencia o histórico de créditos das empresas/navios de forma thread-safe.
// O saldo de cada empresa NÃO é uma variável independente: ele é sempre derivado
// da soma dos Movimentos registrados em Historico. O mapa `saldoCache` existe
// apenas como otimização (evita somar o histórico inteiro a cada CheckTx), mas
// é recalculável a qualquer momento a partir de Historico via RecalcularSaldos().
type Ledger struct {
	Historico  []Movimento
	saldoCache map[string]int
	mu         sync.RWMutex
}

// NovoLedger inicializa a estrutura vazia.
// O saldo é estritamente derivado do histórico de Movimentos (Blocos de
// Registro, Transação e Transferência aplicados via FinalizeBlock).
func NovoLedger() *Ledger {
	return &Ledger{
		Historico:  make([]Movimento, 0),
		saldoCache: make(map[string]int),
	}
}

// registrarMovimento adiciona uma entrada ao histórico e atualiza o cache.
// Chamado apenas internamente, sempre dentro de uma seção protegida por l.mu.
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

// VerificarCreditos checa se há saldo suficiente SEM debitar.
func (l *Ledger) VerificarCreditos(empresa string, valor int) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()

	saldo, existe := l.saldoCache[empresa]
	if !existe {
		return false
	}
	return saldo >= valor
}

// DebitarCreditos desconta o valor do saldo da empresa e registra o
// movimento no histórico (origem da derivação do saldo).
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

// CreditarCreditos adiciona fundos a uma empresa e registra o movimento
// no histórico (usado em REGISTRO e TRANSFERENCIA).
func (l *Ledger) CreditarCreditos(empresa string, valor int, tipo, detalhe string) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.registrarMovimento(empresa, valor, tipo, detalhe)
}

// RegistrarEmpresa cria a empresa no ledger com um saldo inicial (gênese),
// registrando esse crédito inicial como o primeiro Movimento do histórico dela.
func (l *Ledger) RegistrarEmpresa(empresa string, saldoInicial int) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if _, existe := l.saldoCache[empresa]; existe {
		return
	}

	l.registrarMovimento(empresa, saldoInicial, "REGISTRO", "Gênese / créditos iniciais")
	log.Printf("[LEDGER] Nova empresa registrada: %s | Saldo inicial: %d", empresa, saldoInicial)
}

// EmpresaExiste checa se o navio já pegou os créditos iniciais dele.
func (l *Ledger) EmpresaExiste(empresa string) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	_, existe := l.saldoCache[empresa]
	return existe
}

// ConsultarSaldo retorna o saldo atual de uma empresa.
func (l *Ledger) ConsultarSaldo(empresa string) (int, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	saldo, existe := l.saldoCache[empresa]
	return saldo, existe
}

// SaldosAtuais retorna uma cópia do mapa de saldos (para a API /saldos).
func (l *Ledger) SaldosAtuais() map[string]int {
	l.mu.RLock()
	defer l.mu.RUnlock()

	copia := make(map[string]int, len(l.saldoCache))
	for k, v := range l.saldoCache {
		copia[k] = v
	}
	return copia
}

// HistoricoCompleto retorna uma cópia do histórico de movimentos (para /extrato).
func (l *Ledger) HistoricoCompleto() []Movimento {
	l.mu.RLock()
	defer l.mu.RUnlock()

	copia := make([]Movimento, len(l.Historico))
	copy(copia, l.Historico)
	return copia
}

// RecalcularSaldos prova que o saldo é derivado do histórico: soma todos os
// Movimentos do zero e retorna o resultado. Usado para auditoria — se o
// resultado bater com saldoCache, o estado é consistente com o histórico.
func (l *Ledger) RecalcularSaldos() map[string]int {
	l.mu.RLock()
	defer l.mu.RUnlock()

	recalculado := make(map[string]int)
	for _, mov := range l.Historico {
		recalculado[mov.Empresa] += mov.Delta
	}
	return recalculado
}
