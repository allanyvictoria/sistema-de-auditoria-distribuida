package main

/*
import (
	"fmt"
	"net"
	"time"
)

func main() {
	fmt.Println("======================================================")
	fmt.Println("INICIANDO SIMULAÇÃO DE ECONOMIA E AUDITORIA DE GUERRA")
	fmt.Println("======================================================")

	// 1. SIMULANDO O SENSOR DO NAVIO
	fmt.Println("\n[SENSOR] Solicitando drone para o Navio_A (Custo: 1 Crédito)...")
	connSensor, err := net.Dial("tcp", "localhost:1053")
	if err != nil {
		fmt.Println("Erro ao conectar no Broker:", err)
		return
	}
	// Envia a mensagem no protocolo TCP que o seu Broker entende
	connSensor.Write([]byte("SENSOR; Navio_A; escolta; alta\n"))
	connSensor.Close()

	// Dá um tempo para o CometBFT rodar o consenso, fechar o bloco e debitar o saldo
	fmt.Println("Aguardando consenso da rede Blockchain (Aprovação do Bloco)...")
	time.Sleep(3 * time.Second)

	// 2. SIMULANDO O DRONE
	fmt.Println("\n[DRONE] Drone-Alpha conectando à rede do Broker...")
	connDrone, err := net.Dial("tcp", "localhost:1053")
	if err != nil {
		fmt.Println("Erro ao conectar Drone:", err)
		return
	}

	// Drone manda o heartbeat para entrar na lista de disponíveis
	connDrone.Write([]byte("DRONE; Drone-Alpha; HEARTBEAT;\n"))
	time.Sleep(2 * time.Second) // Tempo para o broker despachar a missão pendente

	fmt.Println("[DRONE] Missão recebida! Aceitando e iniciando voo no Estreito de Ormuz...")
	connDrone.Write([]byte("DRONE; Drone-Alpha; ACEITE;\n"))

	// Simula o tempo de voo do drone
	time.Sleep(4 * time.Second)

	fmt.Println("[DRONE] Missão concluída com sucesso! Emitindo o Laudo para a Blockchain...")
	// Ao mandar a CONCLUSÃO, o seu Broker vai gerar o PacoteBase do tipo LAUDO
	connDrone.Write([]byte("DRONE; Drone-Alpha; CONCLUSAO;\n"))

	time.Sleep(1 * time.Second)
	connDrone.Close()

	fmt.Println("\nSimulação finalizada! Olhe os logs do Docker para ver a gravação imutável.")
}
*/
