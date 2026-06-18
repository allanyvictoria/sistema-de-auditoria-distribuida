package main

import (
	"bufio"
	"fmt"
	"log"
	"math/rand/v2"
	"net"
	"os"
	"strings"
	"time"
)

// obterBrokerAddr recupera o endereço do broker da variável de ambiente ou aplica o valor padrão.
func obterBrokerAddr() string {
	addr := os.Getenv("BROKER_ADDR")
	if addr == "" {
		addr = "broker:1883"
	}
	fmt.Println("Endereço do broker:", addr)
	return addr
}

// conectarBroker tenta estabelecer uma conexão TCP com o broker principal ou seus alternativos listados.
func conectarBroker() (net.Conn, string, error) {
	addrPrincipal := obterBrokerAddr()

	// Tentativa de conexão com o broker principal.
	conn, err := net.DialTimeout("tcp", addrPrincipal, 5*time.Second)
	if err == nil {
		return conn, addrPrincipal, nil
	}

	// Iteração e tentativa de conexão com os brokers alternativos.
	brokersStr := os.Getenv("BROKERS_ADDR")
	if brokersStr != "" {
		for _, broker := range strings.Split(brokersStr, ",") {
			broker = strings.TrimSpace(broker)
			if broker == "" {
				continue
			}

			addrAlternativo := broker
			if !strings.Contains(addrAlternativo, ":") {
				addrAlternativo = fmt.Sprintf("%s:1053", broker)
			}

			if addrAlternativo == addrPrincipal {
				continue
			}
			conn, err := net.DialTimeout("tcp", addrAlternativo, 5*time.Second)
			if err == nil {
				return conn, addrAlternativo, nil
			}
		}
	}
	return nil, "", fmt.Errorf("nenhum broker disponível")
}

// heartbeat transmite pacotes periódicos para confirmar a disponibilidade do cliente na rede.
func heartbeat(conn net.Conn, id string) {
	for {
		mensagem := fmt.Sprintf("DRONE;%s;HEARTBEAT;%s\n", id, "")
		_, err := conn.Write([]byte(mensagem))
		if err != nil {
			// Interrupção silenciosa da rotina caso a conexão de rede já tenha sido encerrada.
			if strings.Contains(err.Error(), "use of closed network connection") {
				return
			}
			log.Printf("Erro ao enviar heartbeat: %v", err)
			return
		}
		time.Sleep(10 * time.Second)
	}
}

// receberMissao processa as mensagens vindas do broker e gerencia o fluxo de execução das missões.
func receberMissao(conn net.Conn, id string) {
	emMissao := false
	reader := bufio.NewReader(conn)
	
	for {
		mensagem, err := reader.ReadString('\n')
		if err != nil {
			log.Printf("[DRONE %s] Erro de leitura. Conexão perdida com o servidor.", id)
			return
		}
		mensagem = strings.TrimSpace(mensagem)
		log.Printf("[DRONE %s] Mensagem recebida: %s", id, mensagem)

		if strings.Contains(mensagem, "MISSAO") && !emMissao {
			emMissao = true
			log.Printf("[DRONE %s] Iniciando missão.", id)

			// Envio da confirmação de aceite.
			conn.Write([]byte(fmt.Sprintf("DRONE;%s;ACEITE;\n", id)))

			// Simulação do tempo de execução do trajeto.
			time.Sleep(10 * time.Second)

			// Geração autônoma de coordenadas para a rota percorrida.
			latOrigem := -10.0 + (rand.Float64() * 20.0)
			lonOrigem := -40.0 + (rand.Float64() * 20.0)
			latDestino := latOrigem + (rand.Float64() * 2.0)
			lonDestino := lonOrigem + (rand.Float64() * 2.0)
			rotaDinamica := fmt.Sprintf("Lat: %.4f, Lon: %.4f -> Lat: %.4f, Lon: %.4f", latOrigem, lonOrigem, latDestino, lonDestino)

			// Transmissão do status de conclusão juntamente com os dados da rota.
			conn.Write([]byte(fmt.Sprintf("DRONE;%s;CONCLUSAO;%s\n", id, rotaDinamica)))

			emMissao = false
			log.Printf("[DRONE %s] Missão concluída. Rota enviada: %s", id, rotaDinamica)
		}
	}

}

func main() {
	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalf("Erro ao obter hostname: %v", err)
	}

	for {
		conn, addrConectado, err := conectarBroker()
		if err != nil {
			log.Printf("Falha na conexão com brokers. Nova tentativa em 5 segundos.")
			time.Sleep(5 * time.Second)
			continue
		} else {
			log.Printf("[DRONE %s] Conectado ao broker (%s) com sucesso.", hostname, addrConectado)
		}

		tipo := "DRONE"
		id := hostname
		dado := "REGISTRO"
		mensagem := fmt.Sprintf("%s;%s;%s;%s\n", tipo, id, dado, "")

		_, err = conn.Write([]byte(mensagem))
		if err != nil {
			log.Printf("Erro ao enviar registro: %v", err)
		}

		go heartbeat(conn, id)

		receberMissao(conn, id)

		conn.Close()
		log.Printf("Conexão perdida. Iniciando rotina de reconexão.")
	}

}
