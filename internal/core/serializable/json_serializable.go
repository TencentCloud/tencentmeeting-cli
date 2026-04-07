package serializable

import "encoding/json"

type JSONSerializer struct {
}

func (d *JSONSerializer) Name() string {
	return "JSONSerializer"
}

func (d *JSONSerializer) ContentType() string {
	return "application/json; charset=utf-8"
}

func (d *JSONSerializer) Serialize(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}
func (d *JSONSerializer) Deserialize(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

// JsonSerializer is the global JSON serializer.
var JsonSerializer = &JSONSerializer{}

var DefaultSerializer Serializable = JsonSerializer
