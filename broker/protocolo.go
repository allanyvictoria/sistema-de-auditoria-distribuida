package main

import (
	"fmt"
	"strings"
)

// Mensagem define a estrutura do protocolo de comunicação.
type Mensagem struct {
	Tipo    string
	ID      string
	Acao    string
	Payload string
}

// ParseMensagem converte uma string formatada em uma estrutura Mensagem.
func ParseMensagem(linha string) (Mensagem, error) {
	mensagem := strings.TrimSpace(linha)
	parts := strings.Split(mensagem, ";")

	if len(parts) < 4 {
		return Mensagem{}, fmt.Errorf("mensagem malformada: '%s'", mensagem)
	}

	return Mensagem{
		Tipo:    strings.TrimSpace(parts[0]),
		ID:      strings.TrimSpace(parts[1]),
		Acao:    strings.TrimSpace(parts[2]),
		Payload: strings.TrimSpace(parts[3]),
	}, nil
}

// ToBytes processa uma estrutura Mensagem em seu equivalente em slice de bytes.
func ToBytes(m Mensagem) []byte {
	return []byte(fmt.Sprintf("%s;%s;%s;%s", m.Tipo, m.ID, m.Acao, m.Payload))
}
