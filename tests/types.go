package tests

type Employee struct {
	ID       int64
	Name     *string
	Age      int
	Phone    string
	Position *PositionInfo
}

type PositionInfo struct {
	Province *string
	Country  *string
	City     *string
}
