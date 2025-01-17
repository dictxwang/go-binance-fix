package spot

type NewOrder struct {
	Symbol        string
	ClientOrderId string
	OrderQty      *float64
	CashOrderQty  *float64 // 以报价资产单位指定的订单数量，用于反向市场订单。
	OrderType     OrderType
	Price         *float64
	Side          OrderSide
	TimeInForce   *TimeInForce
}

type CancelOrder struct {
	Symbol        string
	ClientOrderId string
	OrderId       string
}
