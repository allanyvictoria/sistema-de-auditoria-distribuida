package main

import (
	"fmt"
	"strings"
)

// Protocolo de comunicação
type Mensagem struct {
	Tipo    string // Quem está enviando: "SENSOR", "DRONE", "BROKER"
	ID      string // identificador de quem está enviando
	Acao    string // ação ou evento a ser realizado
	Payload string // dados extras da mensagem (ex: criticidade, id da requisição, etc)
}

// função para parsear a mensagem recebida e retornar uma struct Mensagem
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

// função para converter uma struct Mensagem em bytes para enviar pela rede
func ToBytes(m Mensagem) []byte {
	return []byte(fmt.Sprintf("%s;%s;%s;%s", m.Tipo, m.ID, m.Acao, m.Payload))
}
