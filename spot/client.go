package spot

import (
	"bufio"
	"bytes"
	"context"
	"crypto"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

type BinanceFixClient struct {
	apiKey     string
	secretKey  string
	privateKey crypto.PrivateKey // 经过解析后的私钥
	//subAccount           string
	ignoreUnknownMessage bool
	//cancelOrdersOnDisconnect bool

	tlsConn   *tls.Conn
	writeChan chan []byte
	readChan  chan string
	failConn  chan error

	closed        bool
	connecting    bool
	authenticated bool

	//tagMap             map[string]string
	flipTagMap map[string]string
	//messageTypeMap     map[string]string
	flipMessageTypeMap map[string]string
	messageSeqNumber   int64
	//noneBodyProp       []string

	localIP       string
	NetResolver   *net.Resolver // 用于指定服务端IP地址
	UseTestServer bool

	senderCompID string // Must obey regex: ^[a-zA-Z0-9-_]{1,8}$
	orderChan    *chan ExecutionReportMessage
	errChan      *chan error
	unknownChan  *chan string
	rejectChan   *chan RejectMessage
	logonChan    *chan string
	logoutChan   *chan bool
}

func NewBinanceFixClientWithLocalIP(apiKey, secretKey, localIP string) (*BinanceFixClient, error) {

	client := BinanceFixClient{
		apiKey:           apiKey,
		secretKey:        secretKey,
		localIP:          localIP,
		messageSeqNumber: 0,
		closed:           true,
	}

	client.writeChan = make(chan []byte)
	client.readChan = make(chan string)
	client.failConn = make(chan error)

	//client.tagMap = makeTagKeyValue()
	client.flipTagMap = flipTagKeyValue()
	//client.messageTypeMap = makeMessageKeyValue()
	client.flipMessageTypeMap = flipMessageKeyValue()
	//client.noneBodyProp = makeNoneBodyProp()

	privateKey, err := parsePrivateKey(secretKey)
	if err != nil {
		return nil, err
	}
	client.privateKey = privateKey

	return &client, nil
}

func NewBinanceFixClient(apiKey, secretKey string) (*BinanceFixClient, error) {
	return NewBinanceFixClientWithLocalIP(apiKey, secretKey, "")
}

func (c *BinanceFixClient) SetChannels(orderChan *chan ExecutionReportMessage, errChan *chan error,
	unknownChan *chan string, rejectChan *chan RejectMessage) {
	c.errChan = errChan
	c.orderChan = orderChan
	c.unknownChan = unknownChan
	c.rejectChan = rejectChan
}

func (c *BinanceFixClient) SetLoginChannels(logonChan *chan string, logoutChan *chan bool) {
	c.logonChan = logonChan
	c.logoutChan = logoutChan
}

func (c *BinanceFixClient) Start(senderCompID string) error {

	// 创建链接
	err, _ := c.dial()
	if err != nil {
		return err
	}

	// 监听错误
	go func() {
		for {
			fail := <-c.failConn

			if !c.closed {
				c.closed = true
				_ = c.tlsConn.Close()
			}
			c.connecting = false
			c.authenticated = false
			c.messageSeqNumber = 0

			if c.errChan != nil {
				*c.errChan <- fail
			} else {
				fmt.Errorf("read/write error: %+v", fail)
			}
		}
	}()

	// TODO test reconnect after error
	//go func() {
	//	time.Sleep(time.Second * 10)
	//	fmt.Printf(">>>>> Before Send Fail For Reconnet\n")
	//	c.failConn <- errors.New("test reconnect")
	//	fmt.Printf(">>>>> After Sent Fail For Reconnet\n")
	//}()

	c.startReadLoop()
	c.startWriteLoop()
	c.startResponseParsing()
	c.startHeartbeat()

	c.senderCompID = senderCompID
	// 发起登录
	err = c.logon()
	return err
}

func (c *BinanceFixClient) PlaceOrder(order NewOrder) error {
	if order.OrderQty == nil && order.CashOrderQty == nil {
		return errors.New("require order-qty or cash-order-qty")
	}

	header := fmt.Sprintf("%s=%s|%s=%s|", FixTagBeginString, FixBeginValue, FixTagBodyLength, FixBodyLengthPlaceHolder)
	messageSeq := c.nextMessageSeq()
	sendingTime := makeSendingTime()
	senderCompID := c.senderCompID

	var timeInForce TimeInForce
	if order.TimeInForce == nil {
		timeInForce = TimeInForceGTC
	} else {
		timeInForce = *order.TimeInForce
	}

	body := fmt.Sprintf("%s=%s|%s=%s|%s=%s|%s=%s|%s=%s|%s=%s|%s=%s|%s=%s|%s=%s|%s=%s|",
		string(FixTagMsgType), string(FixMessageNewOrderSingle),
		string(FixTagMsgSeqNum), messageSeq,
		string(FixTagSenderCompID), senderCompID,
		string(FixTagSendingTime), sendingTime,
		string(FixTagTargetCompID), "SPOT",
		string(FixTagSymbol), order.Symbol,
		string(FixTagClOrdID), order.ClientOrderId,
		string(FixTagOrdType), string(order.OrderType),
		string(FixTagSide), string(order.Side),
		string(FixTagTimeInForce), string(timeInForce),
	)
	if order.Price != nil {
		body = body + string(FixTagPrice) + "=" + fmt.Sprintf("%f", *order.Price) + "|"
	}
	if order.OrderQty != nil {
		body = body + string(FixTagOrderQty) + "=" + fmt.Sprintf("%f", *order.OrderQty) + "|"
	} else {
		body = body + string(FixTagCashOrderQty) + "=" + fmt.Sprintf("%f", *order.CashOrderQty) + "|"
	}

	header = strings.ReplaceAll(header, FixBodyLengthPlaceHolder, fmt.Sprintf("%d", len(body)))
	encode := header + body
	tail := fmt.Sprintf("%s=%s|", string(FixTagCheckSum), calCheckSum(strings.ReplaceAll(encode, "|", FixMsgSeparator)))

	encode = encode + tail
	c.send(strings.ReplaceAll(encode, "|", FixMsgSeparator))

	return nil
}

func (c *BinanceFixClient) CancelOrder(order CancelOrder) error {
	if order.ClientOrderId == "" && order.OrderId == "" {
		return errors.New("require client-order-id or order-id")
	}
	header := fmt.Sprintf("%s=%s|%s=%s|", FixTagBeginString, FixBeginValue, FixTagBodyLength, FixBodyLengthPlaceHolder)
	messageSeq := c.nextMessageSeq()
	sendingTime := makeSendingTime()
	senderCompID := c.senderCompID

	var orderIdTag FixTagType
	var orderIdValue string
	if order.ClientOrderId != "" {
		orderIdTag = FixTagOrigClOrdID
		orderIdValue = order.ClientOrderId
	} else {
		orderIdTag = FixTagOrderID
		orderIdValue = order.OrderId
	}

	clientId := fmt.Sprintf("co%d", time.Now().UnixMilli())

	body := fmt.Sprintf("%s=%s|%s=%s|%s=%s|%s=%s|%s=%s|%s=%s|%s=%s|%s=%s|",
		string(FixTagMsgType), string(FixMessageOrderCancelRequest),
		string(FixTagMsgSeqNum), messageSeq,
		string(FixTagSenderCompID), senderCompID,
		string(FixTagSendingTime), sendingTime,
		string(FixTagTargetCompID), "SPOT",
		string(FixTagSymbol), order.Symbol,
		string(orderIdTag), orderIdValue,
		string(FixTagClOrdID), clientId,
	)
	header = strings.ReplaceAll(header, FixBodyLengthPlaceHolder, fmt.Sprintf("%d", len(body)))
	encode := header + body
	tail := fmt.Sprintf("%s=%s|", string(FixTagCheckSum), calCheckSum(strings.ReplaceAll(encode, "|", FixMsgSeparator)))

	encode = encode + tail
	c.send(strings.ReplaceAll(encode, "|", FixMsgSeparator))

	return nil
}

func (c *BinanceFixClient) dial() (error, bool) {

	if !c.closed || c.connecting {
		return nil, true
	}
	c.connecting = true

	config := tls.Config{
		InsecureSkipVerify: true,
		MinVersion:         tls.VersionTLS12,
	}

	ctx, _ := context.WithCancel(context.Background())
	dialer := &net.Dialer{}
	if c.localIP != "" {
		localAddr, _ := net.ResolveTCPAddr("tcp", c.localIP+":0")
		dialer.LocalAddr = localAddr
	}
	if c.NetResolver != nil {
		dialer.Resolver = c.NetResolver
	}

	var serverAddress string
	if c.UseTestServer {
		serverAddress = FixTestServerAddress
	} else {
		serverAddress = FixServerAddress
	}

	conn, err := dialer.DialContext(ctx, "tcp", serverAddress)
	if err != nil {
		c.connecting = false
		return err, false
	}

	tlsConn := tls.Client(conn, &config)
	err = tlsConn.Handshake()
	if err != nil {
		c.connecting = false
		return err, false
	}

	state := tlsConn.ConnectionState()
	if !state.HandshakeComplete {
		return nil, false
	}

	c.connecting = false
	c.tlsConn = tlsConn
	c.closed = false

	return nil, true
}

func (c *BinanceFixClient) logon() error {
	header := fmt.Sprintf("%s=%s|%s=%s|", FixTagBeginString, FixBeginValue, FixTagBodyLength, FixBodyLengthPlaceHolder)

	messageSeq := c.nextMessageSeq()
	sendingTime := makeSendingTime()
	senderCompID := c.senderCompID
	tobeSign := strings.Join([]string{string(FixMessageLogon), senderCompID, "SPOT", messageSeq, sendingTime}, "|")
	tobeSign = strings.ReplaceAll(tobeSign, "|", FixMsgSeparator)
	sign, err := signPayload(tobeSign, c.privateKey)
	if err != nil {
		return err
	}

	body := fmt.Sprintf("%s=%s|%s=%s|%s=%s|%s=%s|%s=%s|%s=%s|%s=%s|%s=%s|%s=%s|%s=%s|%s=%s|%s=%s|",
		string(FixTagMsgType), string(FixMessageLogon),
		string(FixTagMsgSeqNum), messageSeq,
		string(FixTagSenderCompID), senderCompID,
		string(FixTagSendingTime), sendingTime,
		string(FixTagTargetCompID), "SPOT",
		string(FixTagRawDataLength), fmt.Sprintf("%d", len(sign)),
		string(FixTagRawData), sign,
		string(FixTagUsername), c.apiKey,
		string(FixTagEncryptMethod), "0",
		string(FixTagHeartBtInt), "30",
		string(FixTagResetSeqNumFlag), "Y",
		string(FixTagMessageHandling), "2",
	)
	header = strings.ReplaceAll(header, FixBodyLengthPlaceHolder, fmt.Sprintf("%d", len(body)))
	encode := header + body
	//encode = "8=FIX.4.4|9=247|35=A|34=1|49=EXAMPLE|52=20250114-14:54:45.502|56=SPOT|95=88|96=sR+dUmvMK2P8l6c3d2at/PKGcHsPTp82bN/rrhsw8Fgn605Ve0Cnf3r55NkfKGqCT7nUSKPRw5JVxvs6GTmwBw==|553=NJhDFgid3ctQyNX3qCYo9MpG76WTKYIMZSux6DgIxtk8UpSNjaEy7mikgkqSGmzO|98=0|108=30|141=Y|25035=2|10=207|"
	tail := fmt.Sprintf("%s=%s|", string(FixTagCheckSum), calCheckSum(strings.ReplaceAll(encode, "|", FixMsgSeparator)))

	encode = encode + tail
	c.send(strings.ReplaceAll(encode, "|", FixMsgSeparator))
	return nil
}

func (c *BinanceFixClient) send(message string) {

	//fmt.Printf("Send: %s\n", message)
	allBytes := []byte(message)
	c.writeChan <- allBytes
}

func (c *BinanceFixClient) startReadLoop() {

	go func() {
		if c.closed {
			return
		}

		var msg []byte
		var readCount int
		reader := bufio.NewReader(c.tlsConn)
		for {
			buff, err := reader.ReadBytes(byte(1))
			if err != nil {
				if !c.closed {
					c.failConn <- err
				}
				break
			}
			msg = append(msg, buff...)
			buffLen := len(buff)
			readCount += buffLen
			if buffLen >= 3 && bytes.Equal(buff[0:3], []byte("10=")) {
				// 读取到一个完整的消息
				c.readChan <- string(msg)

				msg = make([]byte, 0)
				readCount = 0
			} else {
				if readCount >= 10240 {
					c.failConn <- errors.New("not found end symbol after read many bytes")
				}
			}
		}

		fmt.Printf("Start Read Loop\n")
	}()
}

func (c *BinanceFixClient) startWriteLoop() {

	go func() {
		if c.closed {
			return
		}
		for {
			msg := <-c.writeChan

			if _, err := c.tlsConn.Write(msg); err != nil {
				if !c.closed {
					c.failConn <- err
				}
				break
			}
		}
	}()

	fmt.Printf("Start Write Loop\n")
}

func (c *BinanceFixClient) startHeartbeat() {

	go func() {
		ticker := time.NewTicker(time.Second * 10)
		for {
			_ = <-ticker.C
			if c.closed {
				ticker.Stop()
				break
			}
			c.heartbeat("")
		}
	}()
}

func (c *BinanceFixClient) startResponseParsing() {

	go func() {
		for {
			content := <-c.readChan
			//fmt.Printf("binance fix response: %s\n", content)

			msgType, values := metaParse(content)

			//fmt.Printf("binance fix meta parse: %s, %+v\n", msgType, values)
			if values == nil || len(values) == 0 {
				continue
			}

			if msgType == FixMessageHeartbeat {
				go c.processHeartbeat(values)
			} else if msgType == FixMessageTestRequest {
				go c.processTestRequest(values)
			} else if msgType == FixMessageLogon {
				go c.processLogon(values)
			} else if msgType == FixMessageReject {
				go c.processReject(values)
			} else if msgType == FixMessageLogout {
				if c.logoutChan != nil {
					*c.logoutChan <- true
				} else {
					fmt.Printf("has logout")
				}
			} else if msgType == FixMessageExecutionReport {
				go c.processExecutionReport(values)
			} else {
				if !c.ignoreUnknownMessage && c.unknownChan != nil {
					// 透传未知的消息体
					*c.unknownChan <- content
				}
			}

		}
	}()

	fmt.Printf("Start Response Parsing\n")
}

func (c *BinanceFixClient) processExecutionReport(values map[string]string) {

	if c.orderChan != nil {
		message := parseExecutionReportMessage(values)
		*c.orderChan <- message
	}
}

func (c *BinanceFixClient) processLogon(values map[string]string) {

	message := parseLogonMessage(values)

	if message.UUID != "" {
		c.authenticated = true

		if c.logonChan != nil {
			*c.logonChan <- message.UUID
		}
	} else {
		c.failConn <- errors.New("logon fail")
	}
}

func (c *BinanceFixClient) processTestRequest(values map[string]string) {

	testReqID := parseTestRequestMessage(values)
	if testReqID == "" {
		return
	}

	// 需要回应一个Heartbeat
	c.heartbeat(testReqID)
}

func (c *BinanceFixClient) heartbeat(testReqID string) {
	if testReqID == "" {
		testReqID = makeSendingTime()
	}
	header := fmt.Sprintf("%s=%s|%s=%s|", FixTagBeginString, FixBeginValue, FixTagBodyLength, FixBodyLengthPlaceHolder)
	messageSeq := c.nextMessageSeq()
	sendingTime := makeSendingTime()
	senderCompID := c.senderCompID

	body := fmt.Sprintf("%s=%s|%s=%s|%s=%s|%s=%s|%s=%s|%s=%s|",
		string(FixTagMsgType), string(FixMessageHeartbeat),
		string(FixTagMsgSeqNum), messageSeq,
		string(FixTagSenderCompID), senderCompID,
		string(FixTagSendingTime), sendingTime,
		string(FixTagTargetCompID), "SPOT",
		string(FixTagTestReqID), testReqID,
	)
	header = strings.ReplaceAll(header, FixBodyLengthPlaceHolder, fmt.Sprintf("%d", len(body)))
	encode := header + body
	tail := fmt.Sprintf("%s=%s|", string(FixTagCheckSum), calCheckSum(strings.ReplaceAll(encode, "|", FixMsgSeparator)))

	encode = encode + tail
	c.send(strings.ReplaceAll(encode, "|", FixMsgSeparator))
}

func (c *BinanceFixClient) processHeartbeat(values map[string]string) {

	testReqID := parseHeartbeatMessage(values)
	if testReqID == "" {
		return
	}

	// 需要回应一个TestRequest
	header := fmt.Sprintf("%s=%s|%s=%s|", FixTagBeginString, FixBeginValue, FixTagBodyLength, FixBodyLengthPlaceHolder)
	messageSeq := c.nextMessageSeq()
	sendingTime := makeSendingTime()
	senderCompID := c.senderCompID

	body := fmt.Sprintf("%s=%s|%s=%s|%s=%s|%s=%s|%s=%s|%s=%s|",
		string(FixTagMsgType), string(FixMessageTestRequest),
		string(FixTagMsgSeqNum), messageSeq,
		string(FixTagSenderCompID), senderCompID,
		string(FixTagSendingTime), sendingTime,
		string(FixTagTargetCompID), "SPOT",
		string(FixTagTestReqID), testReqID,
	)
	header = strings.ReplaceAll(header, FixBodyLengthPlaceHolder, fmt.Sprintf("%d", len(body)))
	encode := header + body
	tail := fmt.Sprintf("%s=%s|", string(FixTagCheckSum), calCheckSum(strings.ReplaceAll(encode, "|", FixMsgSeparator)))

	encode = encode + tail
	c.send(strings.ReplaceAll(encode, "|", FixMsgSeparator))
}

func (c *BinanceFixClient) processReject(values map[string]string) {

	message := parseRejectMessage(values)
	if message.RefMsgType == FixMessageLogon {
		c.authenticated = false
		c.failConn <- errors.New("reject logon: " + message.Text)
	} else {
		if typeName, has := c.flipMessageTypeMap[string(message.RefMsgType)]; has {
			message.RefMsgTypeName = typeName
		}
		if tagName, has := c.flipTagMap[string(message.RefTagID)]; has {
			message.RefTagName = tagName
		}
		if c.rejectChan != nil {
			*c.rejectChan <- message
		}
	}
}

func (c *BinanceFixClient) nextMessageSeq() string {
	atomic.AddInt64(&c.messageSeqNumber, 1)
	return strconv.FormatInt(atomic.LoadInt64(&c.messageSeqNumber), 10)
}
