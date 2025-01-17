package spot

import (
	"strconv"
	"strings"
)

func metaParse(content string) (FixMessageType, map[string]string) {

	parts := strings.Split(content, FixMsgSeparator)
	var messageType FixMessageType
	values := map[string]string{}

	for _, part := range parts {
		pair := strings.Split(part, "=")
		if len(pair) != 2 {
			continue
		}
		if pair[0] == string(FixTagMsgType) {
			messageType = FixMessageType(pair[1])
		} else if pair[0] == string(FixTagBeginString) ||
			pair[0] == string(FixTagBodyLength) ||
			pair[0] == string(FixTagCheckSum) {
			continue
		} else {
			values[pair[0]] = pair[1]
		}
	}

	return messageType, values
}

func parseRejectMessage(values map[string]string) RejectMessage {

	message := RejectMessage{}
	for k, v := range values {
		if k == string(FixTagRefSeqNum) {
			message.RefSeqNum, _ = strconv.ParseInt(v, 10, 64)
		} else if k == string(FixTagRefTagID) {
			message.RefTagID, _ = strconv.ParseInt(v, 10, 64)
		} else if k == string(FixTagRefMsgType) {
			message.RefMsgType = FixMessageType(v)
		} else if k == string(FixTagSessionRejectReason) {
			message.SessionRejectReason, _ = strconv.ParseInt(v, 10, 64)
		} else if k == string(FixTagErrorCode) {
			message.ErrorCode, _ = strconv.ParseInt(v, 10, 64)
		} else if k == string(FixTagText) {
			message.Text = v
		}
	}
	return message
}

func parseHeartbeatMessage(values map[string]string) string {

	var testReqID string
	for k, v := range values {
		if k == string(FixTagTestReqID) {
			testReqID = v
			break
		}
	}
	return testReqID
}

func parseTestRequestMessage(values map[string]string) string {

	var testReqID string
	for k, v := range values {
		if k == string(FixTagTestReqID) {
			testReqID = v
			break
		}
	}
	return testReqID
}

func parseLogonMessage(values map[string]string) LogonMessage {

	message := LogonMessage{}
	for k, v := range values {
		if k == string(FixTagEncryptMethod) {
			message.EncryptMethod, _ = strconv.ParseInt(v, 10, 64)
		} else if k == string(FixTagHeartBtInt) {
			message.HeartBtInt, _ = strconv.ParseInt(v, 10, 64)
		} else if k == string(FixTagUUID) {
			message.UUID = v
		}
	}
	return message
}

func parseExecutionReportMessage(values map[string]string) ExecutionReportMessage {
	message := ExecutionReportMessage{}
	for k, v := range values {
		if k == string(FixTagClOrdID) {
			message.ClOrdID = v
		} else if k == string(FixTagOrigClOrdID) {
			message.OrigClOrdID = v
		} else if k == string(FixTagOrderID) {
			message.OrderID = v
		} else if k == string(FixTagOrderQty) {
			message.OrderQty, _ = strconv.ParseFloat(v, 64)
		} else if k == string(FixTagOrdType) {
			message.OrderType = OrderType(v)
		} else if k == string(FixTagSide) {
			message.OrderSide = OrderSide(v)
		} else if k == string(FixTagSymbol) {
			message.Symbol = v
		} else if k == string(FixTagPrice) {
			message.Price, _ = strconv.ParseFloat(v, 64)
		} else if k == string(FixTagTimeInForce) {
			message.TimeInForce = TimeInForce(v)
		} else if k == string(FixTagTransactTime) {
			message.TransactTime, _ = strconv.ParseInt(v, 10, 64)
		} else if k == string(FixTagOrderCreationTime) {
			message.OrderCreationTime, _ = strconv.ParseInt(v, 10, 64)
		} else if k == string(FixTagExecType) {
			message.ExecType = ExecuteType(v)
		} else if k == string(FixTagCumQty) {
			message.CumQty, _ = strconv.ParseFloat(v, 64)
		} else if k == string(FixTagCumQuoteQty) {
			message.CumQuoteQty, _ = strconv.ParseFloat(v, 64)
		} else if k == string(FixTagLastPx) {
			message.LastPx, _ = strconv.ParseFloat(v, 64)
		} else if k == string(FixTagLastQty) {
			message.LastQty, _ = strconv.ParseFloat(v, 64)
		} else if k == string(FixTagOrdStatus) {
			message.OrderStatus = OrderStatus(v)
		}
	}
	return message

}
