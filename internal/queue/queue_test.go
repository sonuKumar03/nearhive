package queue

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestCancelEvent_Serialization(t *testing.T) {
	id := uuid.New()
	ev := CancelEvent{
		JobID:  id,
		Action: "cancel",
	}

	bytes, err := json.Marshal(ev)
	assert.NoError(t, err)

	var decoded CancelEvent
	err = json.Unmarshal(bytes, &decoded)
	assert.NoError(t, err)
	assert.Equal(t, id, decoded.JobID)
	assert.Equal(t, "cancel", decoded.Action)
}
