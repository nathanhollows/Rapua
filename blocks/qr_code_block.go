package blocks

import (
	"encoding/json"
)

// QRCodeBlock is a placeholder block that instructs players to scan a QR code.
type QRCodeBlock struct {
	BaseBlock
	Instructions string `json:"instructions,omitempty"` // Optional custom instructions
}

func (q *QRCodeBlock) GetID() string {
	return q.ID
}

func (q *QRCodeBlock) GetType() string {
	return "qr_code"
}

func (q *QRCodeBlock) GetLocationID() string {
	return q.LocationID
}

func (q *QRCodeBlock) GetName() string {
	return "QR Code"
}

func (q *QRCodeBlock) GetDescription() string {
	return "Instructs players to scan a QR code"
}

func (q *QRCodeBlock) GetOrder() int {
	return q.Order
}

func (q *QRCodeBlock) GetIconSVG() string {
	return `<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" class="lucide lucide-qr-code-icon lucide-qr-code"><rect width="5" height="5" x="3" y="3" rx="1"/><rect width="5" height="5" x="16" y="3" rx="1"/><rect width="5" height="5" x="3" y="16" rx="1"/><path d="M21 16h-3a2 2 0 0 0-2 2v3"/><path d="M21 21v.01"/><path d="M12 7v3a2 2 0 0 1-2 2H7"/><path d="M3 12h.01"/><path d="M12 3h.01"/><path d="M12 16v.01"/><path d="M16 12h1"/><path d="M21 12v.01"/><path d="M12 21v-1"/></svg>`
}

func (q *QRCodeBlock) GetPoints() int {
	return q.Points
}

func (q *QRCodeBlock) GetData() json.RawMessage {
	data, _ := json.Marshal(q)
	return data
}

func (q *QRCodeBlock) ParseData() error {
	return json.Unmarshal(q.Data, q)
}

func (q *QRCodeBlock) UpdateBlockData(input map[string][]string) error {
	if instructions := input["instructions"]; len(instructions) > 0 {
		q.Instructions = instructions[0]
	}

	// Serialize updated data
	data, err := json.Marshal(q)
	if err != nil {
		return err
	}
	q.Data = data

	return nil
}

func (q *QRCodeBlock) RequiresValidation() bool {
	return false
}

func (q *QRCodeBlock) ValidatePlayerInput(state PlayerState, input map[string][]string) (PlayerState, error) {
	return state, nil
}
