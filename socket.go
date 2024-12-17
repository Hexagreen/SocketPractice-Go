package main

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"strings"
	"sync"
	"time"
)

func createServer() (c net.Conn) {
	fmt.Println("일대일 양방향 통신용 서버를 구동합니다.")
	// 포트 대기
	listener, err := net.Listen("tcp", ":65329")
	if err != nil {
		fmt.Println("서버 시작 실패")
		return
	}
	// 연결 수락
	connection, err := listener.Accept()
	if err != nil {
		fmt.Println("소켓 연결 실패")

	}
	return connection
}

func getPrivateConnection(address string) (c net.Conn) {
	fmt.Print("\033[H\033[2J")
	var connection net.Conn
	var err error
	// 서버에 연결
	connection, err = net.Dial("tcp", address+":65329")
	if err != nil {
		if _, ok := err.(*net.DNSError); ok {
			fmt.Println("DNS 조회에 실패했습니다. 주소를 다시 확인하세요.")
			return nil
		} else if _, ok := err.(*net.AddrError); ok {
			fmt.Println("잘못된 주소 형식입니다. 다시 확인하세요.")
			return nil
		} else {
			fmt.Println("상대방이 수신할 수 있는 상태가 아닙니다.")
			fmt.Println("서버를 개설하고 상대방의 응답을 대기합니다.")
			connection = createServer()
		}
	}
	return connection
}

func chatEngine(c net.Conn) {
	fmt.Print("\033[H\033[2J")
	fmt.Println("   통신 연결 완료. !exit를 입력해 연결 해제")
	var ctrl sync.WaitGroup
	ctrl.Add(1)
	go sendMsg(c, &ctrl)
	go recvMsg(c)

	ctrl.Wait()
	fmt.Println("엔터를 눌러 종료...")
	c.Close()
}

func sendMsg(c net.Conn, ctrl *sync.WaitGroup) {
	for {
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.Trim(input, "\r\n")
		fmt.Print("\033[A\033[2K")
		if input == "!exit" {
			transmit(c, "\000SENDEREXIT")
			ctrl.Done()
			return
		}
		transmit(c, input)
	}
}

func transmit(c net.Conn, input string) {
	var dirc int
	_, err := c.Write([]byte(input + "\n"))
	if err != nil {
		dirc = 2
	} else {
		dirc = 0
	}
	if strings.Contains(input, "\000") {
		return
	}
	go printChat(input, dirc)
}

func recvMsg(c net.Conn) {
	for {
		scanner := bufio.NewScanner(c)
		if scanner.Scan() {
			message := scanner.Text()
			if message == "\000SENDEREXIT" {
				fmt.Println("상대방이 연결을 끊었습니다. !exit를 입력하여 연결 해제")
				return
			}
			go printChat(message, 1)
		}
	}
}

func printChat(message string, direction int) {
	var formatted string
	if direction == 0 {
		formatted += "<< "
	} else if direction == 1 {
		formatted += ">> "
	} else {
		formatted += "\033[31mX< "
	}
	formatted += "["
	formatted += time.Now().Format("15:04:05")
	formatted += "] "
	formatted += message
	fmt.Println(formatted + "\033[0m")
}

func main() {
	fmt.Print("\033[H\033[2J")
	var address string
	fmt.Println("상대방과 연결하려면 [IP주소] 혹은 [도메인] 을 입력")
	fmt.Print("포트를 개방하여 수신을 대기하려면 [s] 를 입력하세요: ")
	fmt.Scanln(&address)

	if address == "s" {
		fmt.Print("\033[H\033[2J")
		c := createServer()
		chatEngine(c)
	} else {
		c := getPrivateConnection(address)
		chatEngine(c)
	}

	var input string
	fmt.Scanln(&input)
}
