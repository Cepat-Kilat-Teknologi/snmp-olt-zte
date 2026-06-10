package trap

import (
	"encoding/json"

	"github.com/Cepat-Kilat-Teknologi/snmp-olt-zte/internal/model"
)

// GenericFormatter sends the raw TrapEvent JSON without transformation.
type GenericFormatter struct{}

func (f *GenericFormatter) Format(event model.TrapEvent) ([]byte, error) {
	return json.Marshal(event)
}

func (f *GenericFormatter) FormatBatch(_ Severity, events []model.TrapEvent) ([]byte, error) {
	return json.Marshal(events)
}

func (f *GenericFormatter) ContentType() string {
	return "application/json"
}
