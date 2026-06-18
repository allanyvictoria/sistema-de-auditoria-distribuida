package main

import (
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"strconv"
	"time"
)

// obterIntervalo recupera o tempo de espera entre envios via variável de ambiente ou aplica o valor padrão.
func obterIntervalo() int {
	intervaloStr := os.Getenv("INTERVALO")
	if intervaloStr == "" {
		return 5
	}

	// Conversão do valor recuperado para tipo numérico inteiro.
	intervalo, err := strconv.Atoi(intervaloStr)
	if err != nil {
		// Retorno do valor numérico padrão em caso de falha na conversão.
		log.Printf("Erro na conversão do intervalo: %v", err)
		return 5
	}

	return intervalo
}

// obterBrokerAddr recupera o endereço do broker da variável de ambiente ou aplica o valor padrão.
func obterBrokerAddr() string {
	addr := os.Getenv("BROKER_ADDR")
	if addr == "" {
		addr = "broker:1053"
	}
	fmt.Println("Endereço do broker:", addr)
	return addr
}

// conectarBroker estabelece a conexão TCP com o endereço do broker configurado.
func conectarBroker() (net.Conn, error) {
	conn, err := net.Dial("tcp", obterBrokerAddr())
	if err != nil {
		return nil, err
	}
	return conn, nil
}

func main() {
	hostname, err := os.Hostname()
	if err != nil {
		log.Fatalf("Erro ao obter o hostname: %v", err)
	}

	// Inicialização da conexão primária com o broker.
	conn, err := conectarBroker()
	if err != nil {
		log.Fatalf("Erro inicial ao conectar ao servidor: %v", err)
	}
	defer conn.Close()

	for {
		// Definição dos domínios de tipos de evento e níveis de criticidade.
		tipos := []string{
			"deriva",
			"bloqueio_rota",
			"objeto_nao_identificado",
			"congestionamento",
			"inspecao_visual",
			"risco_ambiental",
		}
		criticidades := []string{"baixa", "media", "alta"}
		tipoSensor := tipos[rand.Intn(len(tipos))]
		nivelcriticidade := criticidades[rand.Intn(len(criticidades))]

		tipo := "SENSOR"
		id := hostname
		dado := tipoSensor
		criticidade := nivelcriticidade

		mensagem := fmt.Sprintf("%s;%s;%s;%s\n", tipo, id, dado, criticidade)

		_, err := conn.Write([]byte(mensagem))
		if err != nil {
			log.Printf("Erro de transmissão: %v. Iniciando rotina de reconexão.", err)
			
			// Encerramento da conexão defeituosa para liberação de recursos do sistema.
			if conn != nil {
				conn.Close()
			}

			for {
				conn, err = conectarBroker()
				if err == nil {
					log.Println("Reconexão estabelecida com sucesso.")
					break 
				}

				log.Printf("Falha na reconexão: %v. Nova tentativa em breve.", err)
				time.Sleep(5 * time.Second) 
			}
		}

		// Atualização da interface de terminal e registro em log da operação atual.
		fmt.Printf("\r\033[2k\r[SENSOR %s] Criticidade: %s | Horário: %s", dado, criticidade, time.Now().UTC().Format("2006-01-02 15:04:05"))
		log.Printf("[SENSOR %s] Enviando dado de criticidade: %s", dado, criticidade)
		time.Sleep(time.Duration(obterIntervalo()) * time.Second)
	}

}
