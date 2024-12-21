package main

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strconv"
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
		return nil
	}
	defer listener.Close()
	// 연결 수락
	connection, err := listener.Accept()
	if err != nil {
		fmt.Println("소켓 연결 실패")
		return nil
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
		if _, isErr := err.(*net.DNSError); isErr {
			fmt.Println("DNS 조회에 실패했습니다. 주소를 다시 확인하세요.")
			return nil
		} else if _, isErr := err.(*net.AddrError); isErr {
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
	fmt.Println("   통신 연결 완료. !exit를 입력해 연결 해제. !file로 파일 전송")
	defer c.Close()
	var ctrl sync.WaitGroup
	var sendBlocker bool
	keyIn := make(chan string)
	ctrl.Add(1)
	go send(c, &ctrl, &sendBlocker, keyIn)
	go recv(c, &sendBlocker, keyIn)

	ctrl.Wait()
	fmt.Println("엔터를 눌러 종료...")
}

func send(c net.Conn, ctrl *sync.WaitGroup, blocker *bool, keyChan chan<- string) {
	for {
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.Trim(input, "\r\n")
		fmt.Print("\033[A\033[2K")
		if *blocker {
			keyChan <- input
			continue
		}
		if input == "!exit" {
			transmitText(c, "\000SENDEREXIT")
			ctrl.Done()
			return
		}
		if input == "!file" {
			transmitFile(c)
			continue
		}
		if input == "" {
			continue
		}
		transmitText(c, input)
	}
}

func transmitText(c net.Conn, input string) {
	var dirc int
	_, err := c.Write([]byte(fmt.Sprintf("t%s\n", input)))
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

func transmitFile(c net.Conn) {
	printChat("전송할 파일의 경로를 입력하세요... (파일 -> 우클릭 -> 경로로 복사(Ctrl+Shift+C) 후 붙여넣기)", 3)
	var src string
	fmt.Scanln(&src)
	src = strings.ReplaceAll(src, "\"", "")
	file, fErr := os.ReadFile(src)
	if _, isErr := fErr.(*os.PathError); isErr {
		printChat("파일을 찾을 수 없습니다.", 4)
		return
	}

	var dirc int
	_, err := c.Write([]byte(fmt.Sprintf("f%d|%s|%s", len(file), filepath.Ext(src), file)))
	if err != nil {
		dirc = 2
	} else {
		dirc = 0
	}
	go printChat("파일 "+src+" 전송함", dirc)
}

func recv(c net.Conn, sendBlock *bool, keyChan chan string) {
	for {
		reader := bufio.NewReader(c)
		typecode, _ := reader.ReadByte()
		if typecode == 't' {
			message, _ := reader.ReadString('\n')
			if message == "\000SENDEREXIT\n" {
				fmt.Println("   상대방이 연결을 끊었습니다. !exit를 입력하여 연결 해제")
				return
			}
			go printChat(strings.Trim(message, "\n"), 1)
		}
		if typecode == 'f' {
			printChat("파일 수신 중...", 1)
			fileLen, _ := reader.ReadString('|')
			fileLen = strings.Trim(fileLen, "|")
			fileLenInt, _ := strconv.Atoi(fileLen)
			fileExt, _ := reader.ReadString('|')
			fileExt = strings.Trim(fileExt, "|")
			data := make([]byte, fileLenInt)
			io.ReadFull(reader, data)
			go recvFile(data, fileExt, sendBlock, keyChan)
			continue
		}
	}
}

func recvFile(readData []byte, fileExt string, sendBlock *bool, keyChan <-chan string) {
	*sendBlock = true
	defer func() {
		*sendBlock = false
	}()
	printChat("파일을 수신했습니다. 저장하려면 y 를 입력하세요.", 3)
	accept := <-keyChan
	if accept != "y" {
		printChat("파일 수신을 거부했습니다.", 4)
		return
	} else {
		printChat("파일을 저장할 경로를 입력하세요.", 3)
		dst := <-keyChan
		dst = strings.ReplaceAll(dst, "\"", "")
		dst, _ = strings.CutSuffix(dst, fileExt)
		dst += fileExt
		file, fErr := os.Create(dst)
		writer := bufio.NewWriter(file)
		writer.Write(readData)
		writer.Flush()
		if fErr != nil {
			printChat("파일 저장 중 문제가 발생했습니다.", 4)
			return
		}
		defer file.Close()
		printChat("파일 저장 완료. "+dst, 3)
	}
}

func printChat(message string, direction int) {
	var formatted string
	if direction == 0 {
		formatted += "<< "
	} else if direction == 1 {
		formatted += ">> "
	} else if direction == 2 {
		formatted += "\033[31mX< "
	} else if direction == 3 {
		formatted += "\033[34m** "
	} else if direction == 4 {
		formatted += "\033[31m** "
	}
	formatted += "["
	formatted += time.Now().Format("15:04:05")
	formatted += "] "
	formatted += message
	fmt.Println(formatted + "\033[0m")
}

func body() int {
	fmt.Print("\033[H\033[2J")
	var address string
	fmt.Println("상대방과 연결하려면 [IP주소] 혹은 [도메인] 을 입력")
	fmt.Print("포트를 개방하여 수신을 대기하려면 [s] 를 입력하세요: ")
	fmt.Scanln(&address)

	var c net.Conn
	if address == "s" {
		fmt.Print("\033[H\033[2J")
		c = createServer()
	} else {
		c = getPrivateConnection(address)
	}
	if c == nil {
		var input string
		fmt.Scanln(&input)
		return -1
	}
	chatEngine(c)

	var input string
	fmt.Scanln(&input)
	return 0
}

func main() {
	for {
		exitcode := body()
		if exitcode == 0 {
			return
		}
	}
}
