package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
)

// O mapa de setores
var setores map[string]string

var tiposSensor = []string{
	"bloqueio_rota",
	"deriva",
	"congestionamento",
	"objeto_nao_identificado",
	"inspecao_visual",
	"risco_ambiental",
}

var criticidades = []string{
	"baixa",
	"media",
	"alta",
}

func escolher(titulo string, opcoes []string) string {
	for {
		fmt.Printf("\n=== %s ===\n", titulo)
		for i, op := range opcoes {
			fmt.Printf("  [%d] %s\n", i+1, op)
		}
		fmt.Print("Escolha: ")
		reader := bufio.NewReader(os.Stdin)
		linha, _ := reader.ReadString('\n')
		linha = strings.TrimSpace(linha)
		n, err := strconv.Atoi(linha)
		if err == nil && n >= 1 && n <= len(opcoes) {
			return opcoes[n-1]
		}
		fmt.Println("Opção inválida, tente novamente.")
	}
}

func escolherSetor() (string, string) {
	setoresOpts := []string{"Setor 1", "Setor 2", "Setor 3"}
	escolha := escolher("SETOR", setoresOpts)
	num := strings.Split(escolha, " ")[1]
	return num, setores[num]
}

func enviar(brokerAddr, tipoSensor, criticidade string) {
	conn, err := net.Dial("tcp", brokerAddr)
	if err != nil {
		fmt.Printf("[ERRO] Não foi possível conectar em %s: %v\n", brokerAddr, err)
		return
	}
	defer conn.Close()

	msg := fmt.Sprintf("SENSOR;sensor-manual-%s;%s;%s\n", strings.ReplaceAll(brokerAddr, ":", "-"), tipoSensor, criticidade)
	_, err = conn.Write([]byte(msg))
	if err != nil {
		fmt.Printf("[ERRO] Falha ao enviar: %v\n", err)
		return
	}
	fmt.Printf("\n✔ Enviado para %s → tipo: %s | criticidade: %s\n", brokerAddr, tipoSensor, criticidade)
}

func main() {
	// 1. Lê a variável de ambiente IP passada pelo Docker
	ipBase := os.Getenv("IP")
	if ipBase == "" {
		ipBase = "127.0.0.1" // Padrão caso você esqueça de passar o -e
	}

	// 2. Divide a string onde tiver vírgula
	ips := strings.Split(ipBase, ",")

	// 3. Monta os setores forçando todos para a porta 1053
	if len(ips) == 3 {
		setores = map[string]string{
			"1": fmt.Sprintf("%s:1053", strings.TrimSpace(ips[0])),
			"2": fmt.Sprintf("%s:1053", strings.TrimSpace(ips[1])),
			"3": fmt.Sprintf("%s:1053", strings.TrimSpace(ips[2])),
		}
	} else {
		setores = map[string]string{
			"1": fmt.Sprintf("%s:1053", strings.TrimSpace(ips[0])),
			"2": fmt.Sprintf("%s:1053", strings.TrimSpace(ips[0])),
			"3": fmt.Sprintf("%s:1053", strings.TrimSpace(ips[0])),
		}
	}

	fmt.Println("╔══════════════════════════════════════╗")
	fmt.Println("║     SENSOR MANUAL - MODO CLIENTE     ║")
	fmt.Println("╚══════════════════════════════════════╝")

	if len(ips) == 3 {
		fmt.Printf(" Modo Distribuído (3 IPs lidos da variável)\n")
	} else {
		fmt.Printf(" Modo Local (IP: %s)\n", ips[0])
	}

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Println("\n----------------------------------------")

		setorNum, brokerAddr := escolherSetor()
		fmt.Printf("→ Setor %s | Broker: %s\n", setorNum, brokerAddr)

		tipo := escolher("TIPO DE SENSOR", tiposSensor)
		crit := escolher("CRITICIDADE", criticidades)

		fmt.Printf("\nConfirmar envio?\n  Setor %s | %s | criticidade: %s\n  [s] Sim  [n] Cancelar\n> ", setorNum, tipo, crit)
		resp, _ := reader.ReadString('\n')
		resp = strings.TrimSpace(strings.ToLower(resp))

		if resp == "s" || resp == "sim" {
			enviar(brokerAddr, tipo, crit)
		} else {
			fmt.Println("Cancelado.")
		}

		fmt.Print("\nEnviar outro? [s/n]: ")
		cont, _ := reader.ReadString('\n')
		cont = strings.TrimSpace(strings.ToLower(cont))
		if cont != "s" && cont != "sim" {
			fmt.Println("Encerrando sensor manual.")
			break
		}
	}
}
