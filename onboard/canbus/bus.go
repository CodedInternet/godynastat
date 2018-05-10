package canbus

type CANBusInterface interface {
	AddListener(nodeId uint32, rxchan chan CANMsg)
	SendMsg(msg CANMsg) error
}
