package spot

type RejectMessage struct {
	RefSeqNum           int64
	RefTagID            int64
	RefTagName          string
	RefMsgType          FixMessageType
	RefMsgTypeName      string
	SessionRejectReason int64
	ErrorCode           int64
	Text                string
}

type LogonMessage struct {
	EncryptMethod int64
	HeartBtInt    int64
	UUID          string
}

type OrderType string

const (
	OrderTypeMarket    OrderType = "1"
	OrderTypeLimit     OrderType = "2"
	OrderTypeStopLoss  OrderType = "3"
	OrderTypeStopLimit OrderType = "4"
)

type OrderSide string

const (
	OrderSideBuy  OrderSide = "1"
	OrderSideSell OrderSide = "2"
)

type TimeInForce string

const (
	TimeInForceGTC TimeInForce = "1" //GOOD_TILL_CANCEL
	TimeInForceIOC TimeInForce = "3" // IMMEDIATE_OR_CANCEL
	TimeInForceFOK TimeInForce = "4" // FILL_OR_KILL
)

type ExecuteType string

const (
	ExecuteTypeNew       ExecuteType = "0"
	ExecuteTypeCancelled ExecuteType = "4"
	ExecuteTypeReplaced  ExecuteType = "5"
	ExecuteTypeRejected  ExecuteType = "8"
	ExecuteTypeTrade     ExecuteType = "F"
	ExecuteTypeExpired   ExecuteType = "C"
)

type OrderStatus string

const (
	OrderStatusNew             OrderStatus = "0"
	OrderStatusPartiallyFilled OrderStatus = "1"
	OrderStatusFilled          OrderStatus = "2"
	OrderStatusCancelled       OrderStatus = "4"
	OrderStatusPendingCancel   OrderStatus = "6"
	OrderStatusRejected        OrderStatus = "8"
	OrderStatusPendingNew      OrderStatus = "A"
	OrderStatusExpired         OrderStatus = "C"
)

type ExecutionReportMessage struct {
	ClOrdID           string
	OrigClOrdID       string
	OrderID           string
	OrderQty          float64 // 订单数量
	OrderType         OrderType
	OrderSide         OrderSide
	Symbol            string
	Price             float64
	TimeInForce       TimeInForce
	TransactTime      int64
	OrderCreationTime int64
	ExecType          ExecuteType
	CumQty            float64 // 在此订单上交易的 base asset 总数。
	CumQuoteQty       float64 // 在此订单上交易的 quote asset 总数。
	LastPx            float64 // 最后一次执行的价格。
	LastQty           float64 // 最后一次执行的数量。
	OrderStatus       OrderStatus
}
