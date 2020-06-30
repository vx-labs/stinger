package audit

type event string

const (
	AccountCreated        event = "account_created"
	AccountDeleted        event = "account_deleted"
	DeviceCreated         event = "device_created"
	DeviceDeleted         event = "device_deleted"
	DeviceDisabled        event = "device_disabled"
	DeviceEnabled         event = "device_enabled"
	DevicePasswordChanged event = "device_password_changed"
)

type Recorder interface {
	RecordEvent(tenant string, eventKind event, payload map[string]string) error
}
