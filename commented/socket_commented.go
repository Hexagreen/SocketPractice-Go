package temp

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

// Go 언어는 [var 변수명 자료형] 형식으로 변수를 다룸.
// :=		대입 생성 연산자. 변수 선언과 초기화를 동시에 수행
// defer 	함수 종료 직전에 실행할 함수를 등록. 오류로 인한 함수 종료 시에도 실행
// for		Go에는 while문이 없음. for문으로 while문을 대체
// nil		다른 언어에서 null 같은 것
// chan 	채널. 고루틴(비동기 스레드) 간 통신을 위한 데이터 전달 통로. <- 연산자로 데이터 송수신
// go 		고루틴 생성 연산자. 비동기 스레드 생성
// sync.WaitGroup	동기화 객체. 고루틴의 작업 완료를 대기하는 기능 제공
// \033[	터미널 제어용 ANSI Escape Code

// 서버 생성 함수
func createServer() (c net.Conn) {
	fmt.Println("일대일 양방향 통신용 서버를 구동합니다.")
	listener, err := net.Listen("tcp", ":65329") // net 패키지 Listen 함수로 포트 개방
	if err != nil {
		fmt.Println("서버 시작 실패")
		return nil
	}
	defer listener.Close()               // 함수 종료 시 포트 닫기
	connection, err := listener.Accept() // Accept 함수로 클라이언트 연결 대기
	if err != nil {
		fmt.Println("소켓 연결 실패")
		return nil
	}
	return connection
}

// 클라이언트 연결 함수
func getPrivateConnection(address string) (c net.Conn) {
	fmt.Print("\033[H\033[2J")
	var connection net.Conn
	var err error
	connection, err = net.Dial("tcp", address+":65329") // Dial 함수로 서버에 연결
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
			connection = createServer() // 상대방 서버가 닫혀있으면 서버 생성
		}
	}
	return connection
}

// 채팅 엔진
func chatEngine(c net.Conn) {
	fmt.Print("\033[H\033[2J")
	fmt.Println("   통신 연결 완료. !exit를 입력해 연결 해제. !file로 파일 전송")
	defer c.Close()            // 함수 종료 시 연결 닫기
	var ctrl sync.WaitGroup    // WaitGroup 생성. 송신, 수신 함수가 종료될 때까지 이 함수의 진행을 정지
	var sendBlocker bool       // 파일 수신 중인지 판별할 논리값
	keyIn := make(chan string) // 키 입력 채널 생성. 수신 함수가 송신 함수의 동작에서 입력된 값을 받을 수 있도록 함. malloc()과 유사
	ctrl.Add(1)                // WaitGroup에 작업 추가. 대기해야 하는 작업의 수를 설정하는 함수
	go send(c, &ctrl, &sendBlocker, keyIn)
	go recv(c, &sendBlocker, keyIn)

	ctrl.Wait() // WaitGroup에 추가된 작업이 모두 완료될 때까지 대기
	fmt.Println("엔터를 눌러 종료...")
}

// 송신 함수
func send(c net.Conn, ctrl *sync.WaitGroup, blocker *bool, keyChan chan<- string) {
	for {
		reader := bufio.NewReader(os.Stdin) // 키보드 입력을 읽는 Reader 생성
		input, _ := reader.ReadString('\n') // 키보드 입력을 한 줄 읽음
		input = strings.Trim(input, "\r\n") // 문자열 앞뒤의 공백, 개행문자 제거
		fmt.Print("\033[A\033[2K")
		if *blocker { // 파일 수신 중 모드일 때 채널로 입력을 전달. 파일 수신 중에는 텍스트 입력을 무시
			keyChan <- input
			continue
		}
		if input == "!exit" { // !exit 입력 시 연결 종료
			transmitText(c, "\000SENDEREXIT") // 상대방에게 연결 종료를 알리기 위해 특수 문자열 전송
			ctrl.Done()                       // WaitGroup에 작업 완료를 알림
			return
		}
		if input == "!file" { // !file 입력 시 파일 전송 모드로 전환
			transmitFile(c)
			continue
		}
		if input == "" { // 빈 문자열 입력 시 무시
			continue
		}
		transmitText(c, input)
	}
}

// 텍스트 송신 함수
func transmitText(c net.Conn, input string) {
	var dirc int
	_, err := c.Write([]byte(fmt.Sprintf("t%s\n", input))) // Conn 객체에 Write 함수로 메시지에 헤더를 붙여서 전송
	if err != nil {
		dirc = 2 // 에러 발생 시 dirc에 2 대입 : 빨간색으로 출력
	} else {
		dirc = 0 // 에러 없을 시 dirc에 0 대입
	}
	if strings.Contains(input, "\000") { // 특수 문자가 포함된 문자열이면 화면에 전송 기록을 출력하지 않음
		return
	}
	go printChat(input, dirc)
}

// 파일 전송 함수
func transmitFile(c net.Conn) {
	printChat("전송할 파일의 경로를 입력하세요... (파일 -> 우클릭 -> 경로로 복사(Ctrl+Shift+C) 후 붙여넣기)", 3) // dirc = 3 : 파란색으로 출력
	var src string
	fmt.Scanln(&src)
	src = strings.ReplaceAll(src, "\"", "") // 경로에 큰따옴표가 포함되어 있으면 제거
	file, fErr := os.ReadFile(src)          // 파일 읽기
	if _, isErr := fErr.(*os.PathError); isErr {
		printChat("파일을 찾을 수 없습니다.", 4) // 파일이 없을 시 에러 출력
		return
	}

	var dirc int
	_, err := c.Write([]byte(fmt.Sprintf("f%d|%s|%s", len(file), filepath.Ext(src), file))) // 파일 데이터에 타입, 길이, 확장자를 붙여서 전송
	if err != nil {
		dirc = 2
	} else {
		dirc = 0
	}
	go printChat("파일 "+src+" 전송함", dirc) // 파일 전송 완료 메시지 출력
}

// 수신 함수
func recv(c net.Conn, sendBlock *bool, keyChan chan string) {
	for {
		reader := bufio.NewReader(c)     // Conn 에서 Reader 생성
		typecode, _ := reader.ReadByte() // Reader에서 바이트 하나 읽음 = 헤더 첫 글자 확인
		if typecode == 't' {             // 헤더가 t일 때 텍스트 수신
			message, _ := reader.ReadString('\n')
			if message == "\000SENDEREXIT\n" { // 특수 문자열 수신 시 연결 종료
				fmt.Println("   상대방이 연결을 끊었습니다. !exit를 입력하여 연결 해제")
				return
			}
			go printChat(strings.Trim(message, "\n"), 1)
		}
		if typecode == 'f' { // 헤더가 f일 때 파일 수신
			printChat("파일 수신 중...", 1)
			fileLen, _ := reader.ReadString('|') // 파일 길이 읽기기
			fileLen = strings.Trim(fileLen, "|")
			fileLenInt, _ := strconv.Atoi(fileLen) // 문자열을 정수로 변환
			fileExt, _ := reader.ReadString('|')   // 확장자 읽기
			fileExt = strings.Trim(fileExt, "|")
			data := make([]byte, fileLenInt) // 파일 길이만큼의 메모리 확보
			io.ReadFull(reader, data)        // Reader에서 파일 데이터 읽기
			go recvFile(data, fileExt, sendBlock, keyChan)
			continue
		}
	}
}

// 파일 수신 함수
func recvFile(readData []byte, fileExt string, sendBlock *bool, keyChan <-chan string) {
	*sendBlock = true // 파일 수신 중 send의 동작을 제한
	defer func() {    // 함수 종료 시 sendBlock 해제
		*sendBlock = false
	}()
	printChat("파일을 수신했습니다. 저장하려면 y 를 입력하세요.", 3)
	accept := <-keyChan // 채널로 입력을 받음. 채널을 통해 값이 들어올 때까지 함수는 정지함
	if accept != "y" {  // y가 아니면 파일 저장 거부
		printChat("파일 수신을 거부했습니다.", 4)
		return
	} else {
		printChat("파일을 저장할 경로를 입력하세요.", 3)
		dst := <-keyChan                         // 채널로 입력을 받음. 채널을 통해 값이 들어올 때까지 함수는 정지함
		dst = strings.ReplaceAll(dst, "\"", "")  // 큰따옴표 제거
		dst, _ = strings.CutSuffix(dst, fileExt) // 경로로 지정된 파일의 이름에서 확장자 제거
		dst += fileExt                           // 수신 받은 데이터에서 추출한 확장자를 붙임
		file, fErr := os.Create(dst)             // 파일 생성
		writer := bufio.NewWriter(file)          // 파일에 쓰기 위한 Writer 생성
		writer.Write(readData)                   // 파일에 데이터 쓰기
		writer.Flush()                           // 버퍼 비우기 = 파일에 기록 완료
		if fErr != nil {
			printChat("파일 저장 중 문제가 발생했습니다.", 4)
			return
		}
		defer file.Close() // 함수 종료 시 파일 닫기
		printChat("파일 저장 완료. "+dst, 3)
	}
}

// 채팅 출력 함수. ANSI Escape Code를 사용하여 색상 변경
func printChat(message string, direction int) {
	var formatted string
	if direction == 0 {
		formatted += "<< "
	} else if direction == 1 {
		formatted += ">> "
	} else if direction == 2 {
		formatted += "\033[31mX< " // 빨간색. 송신 실패 화살표
	} else if direction == 3 {
		formatted += "\033[34m** " // 파란색. 시스템 메시지
	} else if direction == 4 {
		formatted += "\033[31m** " // 빨간색. 에러 메시지
	}
	formatted += "["
	formatted += time.Now().Format("15:04:05") // 타임스탬프 붙이기
	formatted += "] "
	formatted += message
	fmt.Println(formatted + "\033[0m") // 화면에 채팅 출력 후 색상 초기화
}

// 주 함수
func body() int {
	fmt.Print("\033[H\033[2J")
	var address string
	fmt.Println("상대방과 연결하려면 [IP주소] 혹은 [도메인] 을 입력")
	fmt.Print("포트를 개방하여 수신을 대기하려면 [s] 를 입력하세요: ")
	fmt.Scanln(&address)

	var c net.Conn
	if address == "s" { // s 입력 시 서버 생성
		fmt.Print("\033[H\033[2J")
		c = createServer()
	} else { // s 이외의 입력 시 클라이언트 연결
		c = getPrivateConnection(address)
	}
	if c == nil { // 연결 실패 시 함수 종료. -1 반환
		var input string
		fmt.Scanln(&input)
		return -1
	}
	chatEngine(c) // 문제 없으면 채팅 엔진 실행

	var input string
	fmt.Scanln(&input)
	return 0 // 정상 종료 시 0 반환
}

// 메인 함수. 프로그램 실행 시 실행되는 함수
func main() {
	for { // 무한 루프를 이용하여 정상 종료 외에는 프로그램을 다시 실행하도록 함
		exitcode := body()
		if exitcode == 0 {
			return
		}
	}
}
