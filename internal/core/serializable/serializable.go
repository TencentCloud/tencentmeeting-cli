package serializable

// Serializable 序列化能力
type Serializable interface {
	Name() string
	ContentType() string
	Serialize(v interface{}) ([]byte, error)
	Deserialize(data []byte, v interface{}) error
}
